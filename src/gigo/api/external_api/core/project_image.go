package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/gigo-lib/storage"
	"github.com/go-redis/redis/v8"
	"io"
	"net/http"
	"time"
)

type TextToImageImage struct {
	Base64       string `json:"base64"`
	Seed         uint32 `json:"seed"`
	FinishReason string `json:"finishReason"`
}

type TextToImageResponse struct {
	Images []TextToImageImage `json:"artifacts"`
}

func GenerateProjectImage(storageEngine storage.Storage, rdb redis.UniversalClient, sf *snowflake.Node, userID int64, apiHost string, apiKey string, prompt string) (map[string]interface{}, error) {
	// define a Redis key to track user generation count
	redisKey := fmt.Sprintf("user:%v:image:gen:count", userID)

	// increment the count in Redis
	count, err := rdb.Incr(context.Background(), redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to increment count in Redis: %v", err)
	}

	// set the timeout for the key to 5 minutes
	_ = rdb.Expire(context.Background(), redisKey, time.Minute*5).Err()

	// check if the user has already generated 10 images
	if count >= 10 {
		return map[string]interface{}{"message": "User has already reached the generation limit"}, nil
	}

	// create failure flag
	failed := true

	// deferred decrement of the count in case of generation failure
	defer func() {
		if !failed {
			return
		}

		// decrement the count
		_, _ = rdb.Decr(context.Background(), redisKey).Result()

	}()

	// if it's the first image the user is generating, set a one day expiration
	if count == 1 {
		err = rdb.Expire(context.Background(), redisKey, time.Hour*24).Err()
		if err != nil {
			return nil, fmt.Errorf("failed to set expiry for Redis key: %v", err)
		}
	}

	// Build REST endpoint URL w/ specified engine
	engineId := "stable-diffusion-xl-1024-v1-0"

	reqUrl := apiHost + "/v1/generation/" + engineId + "/text-to-image"

	var data = []byte(fmt.Sprintf(`{
		"text_prompts": [
		  {
			"text": "%v"
		  }
		],
		"cfg_scale": 7,
		"clip_guidance_preset": "FAST_BLUE",
		"height": 768,
		"width": 1344,
		"samples": 1,
		"steps": 50
  	}`, prompt))

	req, err := http.NewRequest("POST", reqUrl, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", "Bearer "+apiKey)

	// Execute the request & read all the bytes of the body
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %v", err)
	}

	// ensure res.Body is not nil
	if res.Body == nil {
		return nil, fmt.Errorf("failed to execute request, res.Body was nil: %v", err)
	}

	defer res.Body.Close()

	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("stability api failed with non-200 status code: %s", string(b))
	}

	// Decode the JSON body
	var body TextToImageResponse
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("Failed to decode response to model: %v", err)
	}

	imgId := sf.Generate().Int64()

	// ensure storage engine is not nil
	if storageEngine == nil {
		return nil, fmt.Errorf("storageEngine was nil: %v", err)
	}

	// Write the images to disk
	for _, image := range body.Images {
		imageBytes, err := base64.StdEncoding.DecodeString(image.Base64)
		if err != nil {
			return nil, fmt.Errorf("Failed to decode image to bytes: %v", err)
		}

		err = storageEngine.CreateFile(
			fmt.Sprintf("temp_proj_images/%v/%v.jpg", userID, imgId),
			imageBytes,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create file in storage engine for image: %v, err: %v", imgId, err)
		}

		err = storageEngine.CreateFile(
			fmt.Sprintf("temp_proj_images/%v/%v.exp", userID, imgId),
			[]byte(fmt.Sprintf("%v", time.Now().UTC().Add(15*time.Minute))),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create file in storage engine for image: %v, err: %v", imgId, err)
		}
	}

	// convert image id to string for frontend
	imgIdString := fmt.Sprintf("%v", imgId)

	// set failed flag to false
	failed = false

	return map[string]interface{}{"image": &imgIdString}, nil
}
