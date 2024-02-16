package external_api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gigo-core/gigo/config"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/coder/retry"
	config2 "github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/git"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/gage-technologies/gigo-lib/search"
	storage2 "github.com/gage-technologies/gigo-lib/storage"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/go-redis/redis/v8"
)

var testHttpServer *HTTPServer
var client *http.Client
var testUserAuth = ""
var testStorage storage2.Storage
var testTiDB *ti.Database
var testRdb redis.UniversalClient
var testLogger logging.Logger
var testSnowflakeNode *snowflake.Node
var testVcsClient *git.VCSClient
var testMeili *search.MeiliSearchEngine
var callingUser *models.User

func init() {
	var err error

	// // generate path to base repo directory
	// _, b, _, _ := runtime.Caller(0)
	// basepath := strings.Replace(filepath.Dir(b), "/src/gigo/api/external_api", "", -1)
	//
	// // open api key file in test data directory
	// f, err := os.Open(basepath + "/test_data/api-key")
	// if err != nil {
	//	//time.Sleep(time.Hour)
	//	log.Panicf("Error: Init() : %v", err)
	// }
	//
	// // read all data from key file
	// apiKeyRaw, err := io.ReadAll(f)
	// if err != nil {
	//	log.Panicf("Error: Init() : %v", err)
	// }
	//
	// // assign api key in string for to auth
	// testUserAuth = string(apiKeyRaw)

	// create storage interface
	testStorage, err = storage2.CreateMinioObjectStorage(config2.StorageS3Config{
		Bucket:    "gigo-dev",
		AccessKey: "gigo-dev",
		SecretKey: "gigo-dev",
		Endpoint:  "gigo-dev-minio:9000",
		UseSSL:    false,
	})
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}
	testStorage.DeleteDir("temp/starter-upload", true)

	// valid, _, _, _ := utils.ValidateExternalJWT(testStorage, testUserAuth, getPublicIP(), map[string]interface{}{})
	// if !valid {
	//	token, err := utils.CreateExternalJWT(testStorage, "69", getPublicIP(), 12, 0, map[string]interface{}{})
	//	if err != nil {
	//		log.Panicf("Error: Init() : %v", err)
	//	}
	//
	//	err = os.WriteFile(basepath+"/test_data/api-key", []byte(token), 0644)
	//	if err != nil {
	//		log.Panicf("Error: Init() : %v", err)
	//	}
	//
	//	testUserAuth = token
	// }

	testTiDB, err = ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		log.Panicf("Error: Creating test tidb : %v", err)
	}

	_, err = testTiDB.DB.Exec("delete from users")
	if err != nil {
		log.Panicf("Error: Deleting test users : %v", err)
	}
	_, err = testTiDB.DB.Exec("delete from user_rewards_inventory")
	if err != nil {
		log.Panicf("Error: Deleting test user_rewards_inventory : %v", err)
	}

	bootstrapUser, err := models.CreateUser(
		69,
		"bootstrapper",
		"gigo-dev",
		"dev@gigo.dev",
		"1234567891",
		models.UserStatusBasic,
		"",
		nil,
		nil,
		"boot",
		"strapper",
		0,
		"None",
		models.UserStart{},
		"America/Chicago",
		models.AvatarSettings{},
		0,
		nil,
	)
	if err != nil {
		log.Panicf("failed to create bootstrapUser, err: %v", err)
		return
	}
	statements, err := bootstrapUser.ToSQLNative()
	if err != nil {
		log.Panicf("failed to create bootstrapUser, err: %v", err)
		return
	}

	for _, statement := range statements {
		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			log.Panicf("failed to create bootstrapUser, err: %v", err)
		}
	}

	// create redis connection
	testRdb = redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	// // create email client
	// emailClient, err := email.CreateClient(emailServer, senderEmail, password)
	// if err != nil { panic(err) }

	defer os.Remove("test.log")
	testLogger, err = logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Creating Basic Logger : %v", err)
	}

	// pm := network.CreatePrivateProxyManager("http://san8:22354/proxies", logger)

	testSnowflakeNode, err = snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Creating snowflake test node : %v", err)
	}

	// type HttpServerConfig struct {
	//    Hostname                     string              `yaml:"hostname"`
	//    Domain                       string              `yaml:"domain"`
	//    Address                      string              `yaml:"address"`
	//    Port                         string              `yaml:"port"`
	//    DevelopmentMode              bool                `yaml:"development_mode"`
	//    LoggerConfig                 config.LoggerConfig `yaml:"logger_config"`
	//    HostSite                     string              `yaml:"host_site"`
	//    UseTLS                       bool                `yaml:"use_tls"`
	//    BasePathHTTP                 string              `yaml:"base_path_http"`
	//    GitWebhookSecret             string              `yaml:"git_webhook_secret"`
	//    StripeWebhookSecret          string              `yaml:"stripe_webhook_secret"`
	//    StripeConnectedWebhookSecret string              `yaml:"stripe_connected_webhook_secret"`
	// }
	httpConfig := config.HttpServerConfig{
		Hostname:        "gigo.dev",
		Domain:          "gigo.dev",
		Address:         "127.0.0.1",
		Port:            "1818",
		DevelopmentMode: true,
		LoggerConfig: config2.LoggerConfig{
			Name: "gigo-http-api-test",
		},
		HostSite:                     "gigo.dev",
		UseTLS:                       false,
		BasePathHTTP:                 "http://gigo.dev",
		GitWebhookSecret:             "",
		StripeWebhookSecret:          "",
		StripeConnectedWebhookSecret: "",
		StableDiffusionHost:          "",
		StableDiffusionKey:           "",
		MailGunApiKey:                "",
		MailGunDomain:                "",
		GigoEmail:                    "",
	}

	// insert sensitive info when testing
	testVcsClient, err = git.CreateVCSClient("http://gigo-dev-git:3000", "gigo-dev", "gigo-dev", true)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create vsc client, %v", err))
	}

	cfg := config2.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config2.MeiliIndexConfig{
			"test": {
				Name:                 "test",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "description", "author"},
				FilterableAttributes: []string{
					"languages",
					"attempts",
					"completions",
				},
				SortableAttributes: []string{
					"attempts",
					"completions",
				},
			},
		},
	}

	testMeili, err = search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create meili client, %v", err))
	}

	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
		Host:        "mq://gigo-dev-nats:4222",
		Username:    "gigo-dev",
		Password:    "gigo-dev",
		MaxPubQueue: 256,
	}, testLogger)
	if err != nil {
		log.Fatal(fmt.Sprintf("failed to create jetstream client, %v", err))
	}

	testHttpServer, err = CreateHTTPServer(httpConfig, "420", testTiDB, testMeili, testRdb, testSnowflakeNode, testVcsClient, testStorage, nil, js, nil, nil, nil, "69", testLogger)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	// create transport credentials
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName:         "gigo-dev",
			InsecureSkipVerify: true,
		},
	}

	// assign new client
	client = &http.Client{
		Transport: tr,
	}

	go func() {
		if err := testHttpServer.Serve(); err != nil {
			log.Panicf("Error: Init() : %v", err)
		}
	}()

	waitUntilReady()

	// login to the session as the bootstrapper
	bootstrapToken := loginTestUser("bootstrapper", "gigo-dev")

	// create the calling user
	createCallingUser(bootstrapToken)

	// login to the calling user
	testUserAuth = loginTestUser("gigo-dev", "gigo-dev")

	// load calling user from the database
	res, err := testTiDB.DB.Query("select * from users where user_name = ?", "gigo-dev")
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}
	defer res.Close()
	if !res.Next() {
		log.Panicf("Error: Init() : user not found")
	}
	callingUser, err = models.UserFromSQLNative(testTiDB, res)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}
}

func getPublicIP() string {
	resp, err := http.Get("http://api.ipify.org")
	if err != nil {
		log.Panicf("Error: getPublicIP() : %v", err)
	}
	defer resp.Body.Close()

	ip, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Panicf("Error: getPublicIP() : %v", err)
	}

	return string(ip)
}

func waitUntilReady() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for retrier := retry.New(time.Millisecond*10, time.Second); retrier.Wait(ctx); {
		resp, err := http.Get("http://localhost:1818/ping")
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 200 {
			return
		}
	}

	log.Panicf("Error: waitUntilReady() : timeout")
}

func loginTestUser(username, password string) string {
	req, err := http.NewRequest("POST", "http://localhost:1818/api/auth/login", nil)
	if err != nil {
		log.Panicf("Error: loginCallingUser() : %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(username, password)

	resp, err := client.Do(req)
	if err != nil {
		log.Panicf("Error: loginCallingUser() : %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		buf, _ := io.ReadAll(resp.Body)
		log.Panicf("Error: loginCallingUser() : failed to login %d: %s", resp.StatusCode, string(buf))
	}

	buf, _ := io.ReadAll(resp.Body)
	log.Println(string(buf))

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "gigoAuthToken" {
			return cookie.Value
		}
	}

	log.Panicf("Error: loginCallingUser() : failed to get gigoAuthToken")
	return ""
}

func createCallingUser(token string) {
	body := map[string]interface{}{
		"user_name":       "gigo-dev",
		"password":        "gigo-dev",
		"email":           "gigo-dev@example.com",
		"phone":           "1234567892",
		"pfp_path":        "",
		"first_name":      "gigo",
		"last_name":       "dev",
		"bio":             "",
		"timezone":        "America/Chicago",
		"start_user_info": map[string]interface{}{},
		"upload_id":       "starter-upload",
		"chunk":           base64.StdEncoding.EncodeToString([]byte("123456789")),
		"part":            1,
		"total_parts":     1,
		"avatar_settings": map[string]interface{}{},
		"force_pass":      true,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		log.Panicf("Error: createCallingUser() : %v", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:1818/api/user/createNewUser", bytes.NewBuffer(buf))
	if err != nil {
		log.Panicf("Error: createCallingUser() : %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: token,
	})

	resp, err := client.Do(req)
	if err != nil {
		log.Panicf("Error: createCallingUser() : %v", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Panicf("Error: createCallingUser() : failed to create user, status code: %v", resp.StatusCode)
	}
}

func TestCreateHTTPServer(t *testing.T) {
	if testHttpServer == nil {
		t.Error("\nCreate HTTP server failed\n    Error: failed to create server")
		return
	}

	t.Log("\nCreate HTTP server succeeded")
}

func TestHandleError(t *testing.T) {
	testRecorder := httptest.NewRecorder()

	testHttpServer.handleError(testRecorder, "test error", "/api/test", "testFunc",
		"GET", int64(8673822), "localhost", "test", "test", http.StatusBadGateway, "test error", nil)

	res := testRecorder.Result()

	if res.StatusCode != http.StatusBadGateway {
		t.Error("\nHandle error failed\n    Error: incorrect status code returned")
		return
	}

	buf := make([]byte, 1024)
	n, _ := res.Body.Read(buf)

	if string(buf[:n]) != `{"message":"test error"}` {
		t.Error("\nHandle error failed\n    Error: incorrect body returned")
		return
	}

	contentType := res.Header.Get("Content-Type")

	if contentType == "" {
		t.Error("\nHandle error failed\n    Error: content type missing from header")
		return
	}

	if contentType != "application/json" {
		t.Error("\nHandle error failed\n    Error: incorrect content type returned")
		return
	}

	t.Log("\nHandle error succeeded")
}

func TestJsonResponse(t *testing.T) {
	testRecorder := httptest.NewRecorder()

	testHttpServer.jsonResponse(testRecorder, map[string]interface{}{"test": true}, "/api/test", "testFunc",
		"GET", int64(74747484), "localhost", "test", "test", http.StatusAccepted)

	res := testRecorder.Result()

	if res.StatusCode != http.StatusAccepted {
		t.Error("\nJSON response failed\n    Error: incorrect status code returned")
		return
	}

	buf := make([]byte, 1024)
	n, _ := res.Body.Read(buf)

	if string(buf[:n]) != `{"test":true}` {
		t.Error("\nJSON response failed\n    Error: incorrect body returned")
		return
	}

	contentType := res.Header.Get("Content-Type")

	if contentType == "" {
		t.Error("\nJSON response failed\n    Error: content type missing from header")
		return
	}

	if contentType != "application/json" {
		t.Error("\nJSON response failed\n    Error: incorrect content type returned")
		return
	}

	t.Log("\nJSON response succeeded")
}

func TestJsonRequest(t *testing.T) {
	body := `{"test":true,"testString":"test","testInt":26483}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/test", bytes.NewBuffer([]byte(body)))
	if err != nil {
		t.Errorf("\nJSON request failed\n    Error: %v", err)
	}

	testRecorder := httptest.NewRecorder()

	out := testHttpServer.jsonRequest(testRecorder, req, "Test", false, "test", 69)

	if out == nil {
		t.Error("\nJSON request failed\n    Error: failed to load JSON body")
		return
	}

	if val, ok := out["test"]; !ok || val != true {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	if val, ok := out["testString"]; !ok || val != "test" {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	if val, ok := out["testInt"]; !ok || val != float64(26483) {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	res := testRecorder.Result()

	if res.StatusCode != http.StatusOK {
		t.Error("\nJSON request failed\n    Error: incorrect status code returned")
		return
	}

	t.Log("\nJSON request succeeded")

	// ///////////////////////////////////////////////////////////////////////////////////////////

	req, err = http.NewRequest("GET", "http://localhost:1818/api/test?test=true&testString=test&testInt=26483", nil)
	if err != nil {
		t.Errorf("\nJSON request failed\n    Error: %v", err)
	}

	testRecorder = httptest.NewRecorder()

	out = testHttpServer.jsonRequest(testRecorder, req, "Test", false, "test", 69)

	if out == nil {
		t.Error("\nJSON request failed\n    Error: failed to load JSON body")
		return
	}

	if val, ok := out["test"]; !ok || val != "true" {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	if val, ok := out["testString"]; !ok || val != "test" {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	if val, ok := out["testInt"]; !ok || val != "26483" {
		t.Error("\nJSON request failed\n    Error: incorrect value returned for json")
		return
	}

	res = testRecorder.Result()

	if res.StatusCode != http.StatusOK {
		t.Error("\nJSON request failed\n    Error: incorrect status code returned")
		return
	}

	t.Log("\nJSON request succeeded")
}

func TestLoadValue(t *testing.T) {
	body := `{"test":true,"testString":"test","testInt":26483,"testSlice":[6573, 3759, 162, 586]}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/test", bytes.NewBuffer([]byte(body)))
	if err != nil {
		t.Errorf("\nLoad value failed\n    Error: %v", err)
	}

	testRecorder := httptest.NewRecorder()

	out := testHttpServer.jsonRequest(testRecorder, req, "Test", false, "test", 69)
	if out == nil {
		t.Error("\nLoad value failed\n    Error: failed to load JSON body")
		return
	}

	test, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "test", reflect.Bool, nil, false, "test", "test")
	if test == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load test key")
		return
	}

	if val, ok := test.(bool); !ok || !val {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for test key")
		return
	}

	res := testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testString, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testString", reflect.String, nil, false, "test", "test")
	if testString == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load testString key")
		return
	}

	if val, ok := testString.(string); !ok || val != "test" {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for testString key")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testInt, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testInt", reflect.Float64, nil, false, "test", "test")
	if testInt == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load testInt key")
		return
	}

	if val, ok := testInt.(float64); !ok || val != float64(26483) {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for testInt key")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	tType := reflect.Float64
	testSlice, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testSlice", reflect.Slice, &tType, false, "test", "test")
	if testSlice == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load testSlice key")
		return
	}

	if val, ok := testSlice.([]interface{}); !ok || !reflect.DeepEqual(val, []interface{}{float64(6573), float64(3759), float64(162), float64(586)}) {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for testSlice key")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testTypeCheck, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "test", reflect.String, nil, false, "test", "test")
	if testTypeCheck != nil || ok {
		t.Error("\nLoad value failed\n    Error: failed to validate type")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testOptional, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testOptional", reflect.String, nil, true, "test", "test")
	if testOptional != nil {
		t.Error("\nLoad value failed\n    Error: loaded key that was not present")
		return
	}

	if !ok {
		t.Error("\nLoad value failed\n    Error: unexpected failure occurred")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testMissing, ok := testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testMissing", reflect.String, nil, false, "test", "test")
	if testMissing != nil || ok {
		t.Error("\nLoad value failed\n    Error: loaded key that was not present")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusUnprocessableEntity {
		t.Error("\nLoad value failed\n    Error: incorrect status code after error should have been written to response")
		return
	}

	t.Log("\nLoad value succeeded")

	req, err = http.NewRequest("GET", "http://localhost:1818/api/test?test=true&testString=test", nil)
	if err != nil {
		t.Errorf("\nLoad value failed\n    Error: %v", err)
	}

	testRecorder = httptest.NewRecorder()

	out = testHttpServer.jsonRequest(testRecorder, req, "Test", false, "test", 69)
	if out == nil {
		t.Error("\nLoad value failed\n    Error: failed to load JSON body")
		return
	}

	test, ok = testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "test", reflect.String, nil, false, "test", "test")
	if test == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load test key")
		return
	}

	if val, ok := test.(string); !ok || val != "true" {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for test key")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")

	testRecorder = httptest.NewRecorder()

	testString, ok = testHttpServer.loadValue(testRecorder, req, out, "TestLoadValue", "testString", reflect.String, nil, false, "test", "test")
	if testString == nil || !ok {
		t.Error("\nLoad value failed\n    Error: failed to load testString key")
		return
	}

	if val, ok := testString.(string); !ok || val != "test" {
		t.Error("\nLoad value failed\n    Error: incorrect value returned for testString key")
		return
	}

	res = testRecorder.Result()
	if res.StatusCode != http.StatusOK {
		t.Error("\nLoad value failed\n    Error: error written to response when return was valid")
		return
	}

	t.Log("\nLoad value succeeded")
}

func TestAuthenticate(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "testUser")

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\nrecc projects home failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testHttpServer.tiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nrecc projects home failed\n    Error: ", err)
			return
		}
	}

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "api") || strings.Contains(r.URL.Path, "login") {
			return
		}

		val := r.Context().Value(CtxKeyUser)
		if val == nil {
			t.Error("\nAuthenticate failed\n    Error: calling user was not returned")
			return
		}

		callingUser, ok := val.(*models.User)
		if !ok {
			t.Error("\nAuthenticate failed\n    Error: calling user is invalid return type")
			return
		}

		if callingUser.UserName != "test" {
			t.Error("\nAuthenticate failed\n    Error: incorrect user returned")
			return
		}
	})

	req := httptest.NewRequest("GET", "http://localhost:1818/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})
	req.RemoteAddr = "127.0.0.1"

	testRecorder := httptest.NewRecorder()

	test := testHttpServer.authenticate(nextHandler)
	test.ServeHTTP(testRecorder, req)

	res := testRecorder.Result()

	if res.StatusCode != http.StatusOK {
		buf := make([]byte, 1024)
		n, _ := res.Body.Read(buf)
		t.Errorf("\nAuthenticate failed\n    Body: %s\n    Error: incorrect status returned", string(buf[:n]))
		return
	}

	t.Log("\nAuthenticate succeeded")

	req = httptest.NewRequest("GET", "http://localhost:1818/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "auth",
		Value: testUserAuth,
	})
	req.RemoteAddr = "127.0.0.1"

	testRecorder = httptest.NewRecorder()

	test = testHttpServer.authenticate(nextHandler)
	test.ServeHTTP(testRecorder, req)

	res = testRecorder.Result()

	buf := make([]byte, 1024)
	n, _ := res.Body.Read(buf)

	if res.StatusCode != http.StatusForbidden {
		t.Errorf("\nAuthenticate failed\n    StatusCode: %d\n    Body: %s\n    Error: incorrect status returned",
			res.StatusCode, string(buf[:n]))
		return
	}

	if string(buf[:n]) != `{"message":"logout"}` {
		t.Errorf("\nAuthenticate failed\n    Body: %s\n    Error: incorrect message returned", string(buf[:n]))
		return
	}

	t.Log("\nAuthenticate succeeded")

	req = httptest.NewRequest("GET", "http://localhost:1818/api/login", nil)
	req.RemoteAddr = "127.0.0.1"

	testRecorder = httptest.NewRecorder()

	test = testHttpServer.authenticate(nextHandler)
	test.ServeHTTP(testRecorder, req)

	res = testRecorder.Result()

	if res.StatusCode != http.StatusForbidden {
		buf := make([]byte, 1024)
		n, _ := res.Body.Read(buf)
		t.Errorf("\nAuthenticate failed\n    Body: %s\n    Error: incorrect status returned", string(buf[:n]))
		return
	}

	t.Log("\nAuthenticate succeeded")

	testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")

	req = httptest.NewRequest("GET", "http://localhost:1818/api/test", nil)
	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})
	req.RemoteAddr = "127.0.0.1"

	testRecorder = httptest.NewRecorder()

	test = testHttpServer.authenticate(nextHandler)
	test.ServeHTTP(testRecorder, req)

	res = testRecorder.Result()

	buf = make([]byte, 1024)
	n, _ = res.Body.Read(buf)

	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("\nAuthenticate failed\n    StatusCode: %d\n    Body: %s\n    Error: incorrect status returned",
			res.StatusCode, string(buf[:n]))
		return
	}

	if string(buf[:n]) != `{"message":"logout"}` {
		t.Errorf("\nAuthenticate failed\n    Body: %s\n    Error: incorrect message returned", string(buf[:n]))
		return
	}

	t.Log("\nAuthenticate succeeded")
}

func TestReceiveUpload(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	basepath := strings.Replace(filepath.Dir(b), "/src/gigo/api/external_api", "", -1)

	fp := basepath + "/test_data/images/jpeg/0a5db4b2b984afdc.jpg"

	fileStat, err := os.Stat(fp)
	if err != nil {
		t.Errorf("\nReceive upload failed\n    Error: %v", err)
		return
	}

	file, err := os.Open(fp)
	if err != nil {
		t.Errorf("\nReceive upload failed\n    Error: %v", err)
		return
	}

	part := int64(1)
	totalParts := int64(math.Ceil(float64(fileStat.Size()) / float64(1024)))
	uploadId := "test-upload"

	defer testHttpServer.storageEngine.DeleteDir("chunks/test-upload", true)
	defer testHttpServer.storageEngine.DeleteFile("temp/test-upload")

	var chunk []byte
	buf := make([]byte, 1024)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				t.Error("\nReceive upload failed\n    Error: incorrect total size")
				return
			}

			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		chunk = buf[:n]
		encodedChunk := base64.StdEncoding.EncodeToString(chunk)

		bodyMap := map[string]interface{}{
			"part":        part,
			"total_parts": totalParts,
			"upload_id":   uploadId,
			"test":        true,
			"chunk":       encodedChunk,
		}

		body, err := json.MarshalIndent(bodyMap, "", "  ")
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		req := httptest.NewRequest("POST", "http://localhost:1818/api/test", bytes.NewBuffer(body))

		testRecorder := httptest.NewRecorder()

		out := testHttpServer.receiveUpload(testRecorder, req, "test", "success", "test", 69)

		if part < totalParts {
			if out != nil {
				t.Error("\nReceive upload failed\n    Error: incorrect function output")
				return
			}

			res := testRecorder.Result()

			if res.StatusCode != http.StatusOK {
				fmt.Println(res.StatusCode)
				b, _ := ioutil.ReadAll(res.Body)
				fmt.Println(string(b))
				t.Error("\nReceive upload failed\n    Error: incorrect status code")
				return
			}

			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("\nReceive upload failed\n    Error: %v", err)
				return
			}

			if string(body) != `{"message":"success"}` {
				t.Error("\nReceive upload failed\n    Error: incorrect response message")
				return
			}

			if part == 1 {
				expFile, _, err := testHttpServer.storageEngine.GetFile("chunks/test-upload/exp")
				if err != nil {
					t.Errorf("\nReceive upload failed\n    Error: %v", err)
					return
				}

				expData, err := ioutil.ReadAll(expFile)
				if err != nil {
					t.Errorf("\nReceive upload failed\n    Error: %v", err)
					return
				}

				exp, err := strconv.ParseInt(string(expData), 10, 64)
				if err != nil {
					t.Errorf("\nReceive upload failed\n    Error: %v", err)
					return
				}

				if exp < time.Now().Add((time.Hour*11)+(time.Minute*59)).Unix() {
					t.Errorf("\nReceive upload failed\n    Error: incorrect expiration time")
					return
				}

				if exp > time.Now().Add((time.Hour*12)+time.Minute).Unix() {
					t.Errorf("\nReceive upload failed\n    Error: incorrect expiration time")
					return
				}
			}

			part++

			continue
		}

		if out == nil {
			t.Error("\nReceive upload failed\n    Error: incorrect function output")
			return
		}

		res := testRecorder.Result()

		if res.StatusCode != http.StatusOK {
			buf, _ := io.ReadAll(res.Body)
			t.Errorf("\nReceive upload failed\n    Error: incorrect status code %v\n    Body: %s", res.StatusCode, string(buf))
			return
		}

		body, err = io.ReadAll(res.Body)
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		if len(body) > 0 {
			t.Error("\nReceive upload failed\n    Error: incorrect body returned")
			return
		}

		if val, ok := out["test"]; !ok || val != true {
			t.Error("\nReceive upload failed\n    Error: incorrect function output")
			return
		}

		hash, err := utils.HashFile(fp)
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		outFile, _, err := testHttpServer.storageEngine.GetFile("temp/test-upload")
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		outFileBytes, err := io.ReadAll(outFile)
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		outHash, err := utils.HashData(outFileBytes)
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		if hash != outHash {
			t.Errorf("\nReceive upload failed\n    Error: file corrupted in upload %s != %s", hash, outHash)
			return
		}

		exists, _, err := testHttpServer.storageEngine.Exists("chunks/test-upload")
		if err != nil {
			t.Errorf("\nReceive upload failed\n    Error: %v", err)
			return
		}

		if exists {
			t.Errorf("\nReceive upload failed\n    Error: failed to remove chunks")
			return
		}

		break
	}

	t.Log("\nReceive upload succeeded")
}

func TestPing(t *testing.T) {
	res, err := client.Get("http://localhost:1818/ping")
	if err != nil {
		t.Errorf("\nPing HTTP failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		buf := make([]byte, 1024)
		n, _ := res.Body.Read(buf)

		t.Errorf("\nPing HTTP failed\n    Status Code: %d\n    Error: %v", res.StatusCode, string(buf[:n]))
		return
	}

	buf := make([]byte, 1024)
	n, err := res.Body.Read(buf)
	if err != nil && err != io.EOF {
		t.Errorf("\nPing HTTP failed\n    Error: %v", err)
		return
	}

	var resJson map[string]interface{}
	err = json.Unmarshal(buf[:n], &resJson)
	if err != nil {
		t.Errorf("\nPing HTTP failed\n    Error: %v", err)
		return
	}

	if val, ok := resJson["status"]; !ok || val != "running" {
		t.Error("\nPing HTTP failed\n    Error: incorrect value returned")
		return
	}

	t.Log("\nPing HTTP succeeded")
}
