package core

import (
	"fmt"
	"github.com/bwmarrin/snowflake"
	storage2 "github.com/gage-technologies/gigo-lib/storage"
	"github.com/go-redis/redis/v8"
	"log"
	"os"
	"testing"
)

func TestGenerateProjectImage(t *testing.T) {
	apiHost, hasApiHost := os.LookupEnv("API_HOST")
	if !hasApiHost {
		apiHost = "https://api.stability.ai"
	}

	// create storage interface
	testStorage, err := storage2.CreateFileSystemStorage("/tmp/gigo-test")
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	userID := testSnowflake.Generate().Int64()

	// Acquire an API key from the environment
	apiKey, hasAPIKey := os.LookupEnv("STABILITY_API_KEY")
	if !hasAPIKey {
		panic("Missing STABILITY_API_KEY environment variable")
	}
	imgId, err := GenerateProjectImage(testStorage, testRdb, testSnowflake, userID, apiHost, apiKey, "a dog with goggles")
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	_, err = os.Stat(fmt.Sprintf("/tmp/gigo-test/temp_proj_images/%v/%v.jpg", userID, imgId))
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

}
