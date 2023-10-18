package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHTTPServer_CreateReportIssue(t *testing.T) {
	var ava *models.AvatarSettings

	newUser := models.User{
		UserName:       "testUserName",
		Email:          "testEmail@test.com",
		Phone:          "1234567890",
		Bio:            "This is a test bio",
		FirstName:      "Test",
		LastName:       "User",
		Timezone:       "America/Los_Angeles",
		Password:       "testPassword",
		AvatarSettings: ava,
	}

	body, err := json.Marshal(newUser)
	if err != nil {
		t.Fatalf("Failed to marshal new user: %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/reportIssue", bytes.NewBuffer(body))
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateReportIssue failed\n    Error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateReportIssue failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreateReportIssue failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_CreateReportIssue succeeded")
}
