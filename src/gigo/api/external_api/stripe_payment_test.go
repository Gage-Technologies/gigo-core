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

func TestHTTPServer_CreateProduct(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestCreateProduct failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Assume we have a post with id 1
	postId := 69

	// Send a request to the CreateProduct endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"post_id": "%v", "cost": "%v"}`, postId, 1500)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/createProduct", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateProduct failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_CreateProduct failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreateProduct failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateProduct failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateProduct failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreateProduct failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreateProduct succeeded")
}

func TestHTTPServer_GetProjectPriceId(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetProjectPriceId failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Assume we have a post with id 1
	postId := 69

	// Send a request to the GetProjectPriceId endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"post_id": "%v"}`, postId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/getPriceId", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_GetProjectPriceId failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_GetProjectPriceId failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_GetProjectPriceId failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectPriceId failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_GetProjectPriceId failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_GetProjectPriceId failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_GetProjectPriceId succeeded")
}

func TestHTTPServer_CancelSubscription(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestCancelSubscription failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Send a request to the CancelSubscription endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{}`)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/cancelSubscription", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CancelSubscription failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_CancelSubscription failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CancelSubscription failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CancelSubscription failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CancelSubscription failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CancelSubscription failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CancelSubscription succeeded")
}

func TestHTTPServer_UpdateClientPayment(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestUpdateClientPayment failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Send a request to the UpdateClientPayment endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"source_token": "test"}`)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/updatePayment", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_UpdateClientPayment failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_UpdateClientPayment failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_UpdateClientPayment failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateClientPayment failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_UpdateClientPayment failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_UpdateClientPayment failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_UpdateClientPayment succeeded")
}

func TestHTTPServer_CreatePortalSession(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestCreatePortalSession failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Send a request to the CreatePortalSession endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{}`)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/portalSession", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreatePortalSession failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_CreatePortalSession failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreatePortalSession failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreatePortalSession failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreatePortalSession failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreatePortalSession failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreatePortalSession succeeded")
}

func TestHTTPServer_CreateConnectedAccount(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestCreateConnectedAccount failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Send a request to the CreateConnectedAccount endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{}`)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/portalSession", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_CreateConnectedAccount failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_CreateConnectedAccount succeeded")
}

func TestHTTPServer_StripeCheckoutSession(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestStripeCheckoutSession failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Assume we have a post with id 1
	postId := 69

	// Send a request to the StripeCheckoutSession endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{"postId": "%v", "priceId": "%v"}`, postId, postId)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/stripeCheckoutSession", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_StripeCheckoutSession failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_StripeCheckoutSession succeeded")
}

func TestHTTPServer_StripePremiumMembershipSession(t *testing.T) {
	// Create test user
	testUser, err := models.CreateUser(1, "testUser", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestStripePremiumMembershipSession failed\n    Error: %v\n", err)
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
		testHttpServer.tiDB.DB.Exec("delete from users where _id = 1 and username = 'testUser'")
	}()

	// Send a request to the StripePremiumMembershipSession endpoint
	body := bytes.NewReader([]byte(fmt.Sprintf(`{}`)))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/stripe/StripePremiumMembershipSession", body)
	if err != nil {
		t.Errorf("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: %v", err)
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
		t.Errorf("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: incorrect response code")
		return
	}

	// Parse the response
	resBody, err := ioutil.ReadAll(res.Body)
	if err != nil {
		t.Error("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: ", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(resBody, &resJson)
	if err != nil {
		t.Error("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: ", err)
		return
	}

	// Verify that the response is as expected.
	// The expected response should reflect the actual structure and data that your function is supposed to return.
	expectedJson := map[string]interface{}{} // adjust to match your expected response structure
	if !reflect.DeepEqual(resJson, expectedJson) {
		t.Error("\nTestHTTPServer_StripePremiumMembershipSession failed\n    Error: response JSON does not match expected JSON")
		return
	}

	t.Log("\nTestHTTPServer_StripePremiumMembershipSession succeeded")
}
