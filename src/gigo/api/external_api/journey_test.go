package external_api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"
)

func TestHTTPServer_SaveJourneyInfo(t *testing.T) {
	// Create a user
	user, err := models.CreateUser(420, "test", "", "", "", models.UserStatusPremium, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestSaveJourneyInfo failed\n    Error: %v\n", err)
		return
	}

	// Add user to DB
	userStmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: ", err)
		return
	}

	for _, s := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: ", err)
			return
		}
	}

	// Insert sample attempt data
	sampleJourneyInfo := models.JourneyInfo{
		ID:               1,
		UserID:           69420,
		LearningGoal:     "Hobby",
		SelectedLanguage: 5,
		EndGoal:          "Fullstack",
		ExperienceLevel:  "Intermediate",
		FamiliarityIDE:   "Some Experience",
		FamiliarityLinux: "Some Experience",
		Tried:            "Tried",
		TriedOnline:      "Tried",
		AptitudeLevel:    "Intermediate",
	}

	journeyStmt := sampleJourneyInfo.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert journey info to SQL: %v", err)
	}

	for _, stmt := range journeyStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample journey info: %v", err)
		}
	}

	body := bytes.NewReader([]byte(`{"attempt_id":"1", "test": true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/getProject", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: incorrect response code")
		return
	}

	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: ", err)
		return
	}

	expectedJson := map[string]interface{}{} // Update the expectedJson to reflect the actual expected data.
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_SaveJourneyInfo failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_SaveJourneyInfo succeeded")
}
