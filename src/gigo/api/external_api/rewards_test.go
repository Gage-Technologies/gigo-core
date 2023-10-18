package external_api

import (
	"encoding/json"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

//func TestHTTPServer_AddXP(t *testing.T) {
//	// Create test user
//	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nTestAddXP failed\n    Error: %v\n", err)
//		return
//	}
//
//	userStmt, err := testUser.ToSQLNative()
//	if err != nil {
//		t.Fatalf("Failed to convert user to SQL: %v", err)
//	}
//
//	for _, stmt := range userStmt {
//		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert test user: %v", err)
//		}
//	}
//
//	defer func() {
//		testHttpServer.tiDB.DB.Exec("delete from users")
//	}()
//
//	// Define some request variables
//	source := "test_source"
//	renownOfChallenge := models.TierTypeGold
//	nemesisBasesCaptured := 1
//
//	// Send a request to the AddXP endpoint
//	body := bytes.NewReader([]byte(fmt.Sprintf(`{"source": "%v", "renown_of_challenge": "%v", "nemesis_bases_captured": "%v"}`, source, renownOfChallenge.String(), nemesisBasesCaptured)))
//	req, err := http.NewRequest("POST", "http://localhost:1818/api/add_xp", body)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_AddXP failed\n    Error: %v", err)
//		return
//	}
//
//	req.AddCookie(&http.Cookie{
//		Name:  "gigoAuthToken",
//		Value: testUserAuth,
//	})
//
//	// Perform the request
//	client := &http.Client{}
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_AddXP failed\n    Error: %v", err)
//		return
//	}
//
//	// Check the response status code
//	if res.StatusCode != http.StatusOK {
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		fmt.Println(res.StatusCode)
//		t.Error("\nTestHTTPServer_AddXP failed\n    Error: incorrect response code")
//		return
//	}
//
//	// Parse the response
//	resBody, err := ioutil.ReadAll(res.Body)
//	if err != nil {
//		t.Error("\nTestHTTPServer_AddXP failed\n    Error: ", err)
//		return
//	}
//
//	var resJson map[string]interface{}
//	err = json.Unmarshal(resBody, &resJson)
//	if err != nil {
//		t.Error("\nTestHTTPServer_AddXP failed\n    Error: ", err)
//		return
//	}
//
//	// Verify that the response is as expected.
//	// The expected response should reflect the actual structure and data that your function is supposed to return.
//	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
//	if !reflect.DeepEqual(resJson, expectedJson) {
//		t.Error("\nTestHTTPServer_AddXP failed\n    Error: response JSON does not match expected JSON")
//		return
//	}
//
//	t.Log("\nTestHTTPServer_AddXP succeeded")
//}

func TestHTTPServer_GetUserRewardsInventory(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetUserRewardsInventory failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
	}()

	// Send a request to the GetUserRewardsInventory endpoint
	req, err := http.NewRequest("GET", "http://localhost:1818/api/reward/getUserRewardInventory", nil)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetUserRewardsInventory failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetUserRewardsInventory succeeded")
}

func TestHTTPServer_SetUserReward(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestSetUserReward failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	rewardId := int64(1)

	// Add the reward to the user_rewards_inventory table
	_, err = testHttpServer.tiDB.DB.Exec("INSERT INTO user_rewards_inventory (reward_id, user_id) VALUES (?, ?)", rewardId, testUser.ID)
	if err != nil {
		t.Fatalf("Failed to add the reward to the user_rewards_inventory table: %v", err)
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from users")
		testHttpServer.tiDB.DB.Exec("delete from user_rewards_inventory")
	}()

	// Send a request to the SetUserReward endpoint
	payload := fmt.Sprintf(`{"reward_id": "%d"}`, rewardId)
	req, err := http.NewRequest("POST", "http://localhost:1818/api/reward/setUserReward", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_SetUserReward failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_SetUserReward failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_SetUserReward failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_SetUserReward succeeded")
}
