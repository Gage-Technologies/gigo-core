package external_api

import (
	"bytes"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHTTPServer_GetUserStreaks(t *testing.T) {
	// Send a request to the GetUserStreaks endpoint
	req, err := http.NewRequest("GET", "http://localhost:1818/api/user/streakPage", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserStreaks failed\n    Error: %v", err)
		return
	}
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserStreaks failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetUserStreaks failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_GetUserStreaks succeeded")
}

func TestHTTPServer_GetStreakFreezeCount(t *testing.T) {
	// Add streak freeze count data for test user
	testStreakFreeze := models.UserStats{
		ID:            69,
		UserID:        callingUser.ID,
		StreakFreezes: 2,
	}
	freezeStmt := testStreakFreeze.ToSQLNative()

	for _, stmt := range freezeStmt {
		_, err := testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test streak freeze count: %v", err)
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from user_stats")
	}()

	body := bytes.NewReader([]byte(`{"test":true}`))
	// Send a request to the GetStreakFreezeCount endpoint
	req, err := http.NewRequest("POST", "http://localhost:1818/api/streakFreeze/get", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetStreakFreezeCount failed\n    Error: %v", err)
		return
	}
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetStreakFreezeCount failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetStreakFreezeCount failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_GetStreakFreezeCount succeeded")
}
