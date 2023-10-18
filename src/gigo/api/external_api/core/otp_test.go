package core

import (
	"context"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gotp"
	"testing"
	"time"
)

func TestGenerateUserOtpUri(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGenerateUserOtpUri failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Call the GenerateUserOtpUri function
	result, err := GenerateUserOtpUri(context.Background(), testUser, testTiDB)
	if err != nil {
		t.Errorf("GenerateUserOtpUri() error = %v", err)
		return
	}

	otpURI, ok := result["otp_uri"].(string)
	if !ok {
		t.Fatalf("GenerateUserOtpUri() result did not contain an otp_uri")
	}

	if otpURI == "" {
		t.Errorf("GenerateUserOtpUri() returned an empty otp_uri")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

func TestVerifyUserOtp(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create a test user
	testUser, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestVerifyUserOtp failed\n    Error: %v\n", err)
		return
	}

	userStmt, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert user to SQL: %v", err)
	}

	for _, stmt := range userStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test user: %v", err)
		}
	}

	// Generate an OTP secret and URI for the test user
	otpResult, err := GenerateUserOtpUri(context.Background(), testUser, testTiDB)
	if err != nil {
		t.Errorf("GenerateUserOtpUri() error = %v", err)
		return
	}

	otpURI, ok := otpResult["otp_uri"].(string)
	if !ok {
		t.Fatalf("GenerateUserOtpUri() result did not contain an otp_uri")
	}

	// Generate a valid OTP code
	otpKey := gotp.NewDefaultTOTP(otpURI)
	if err != nil {
		t.Fatalf("Failed to generate OTP key: %v", err)
	}

	otpCode := otpKey.At(time.Now().Unix())

	//// Test the VerifyUserOtp function
	//storageEngine := storage.NewMemoryStorage()
	//ip := "127.0.0.1"

	result, token, err := VerifyUserOtp(context.Background(), testUser, testTiDB, nil, otpCode, "localhost")
	if err != nil {
		t.Errorf("VerifyUserOtp() error = %v", err)
		return
	}

	auth, ok := result["auth"].(bool)
	if !ok {
		t.Fatalf("VerifyUserOtp() result did not contain 'auth'")
	}

	if !auth {
		t.Errorf("VerifyUserOtp() failed to authenticate user")
	}

	if token == "" {
		t.Errorf("VerifyUserOtp() returned an empty token")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
	}()
}

//func TestOtpData_VerifyUserOtp(t *testing.T) {
//
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	secret := gotp.RandomSecret(64)
//
//	badges := []int64{1, 2}
//
//	// Create a test user
//	user, err := models.CreateUser(1, "test_user", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nTestGenerateUserOtpUri failed\n    Error: %v\n", err)
//		return
//	}
//
//	defer testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
//
//	stmt := user.ToSQLNative()
//	if err != nil {
//		t.Error("\nrecc projects home failed\n    Error: ", err)
//		return
//	}
//
//	for _, s := range stmt {
//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
//		if err != nil {
//			t.Error("\nrecc projects home failed\n    Error: ", err)
//			return
//		}
//	}
//
//	otp := gotp.NewDefaultTOTP(secret)
//
//	code := otp.At(time.Now().Unix())
//
//	// test a valid code with a validated otp user
//	validated := true
//	res, token, err := VerifyUserOtp(models.User{Otp: &secret, OtpValidated: &validated, UserName: "test", ID: 2}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || !valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response did not contain a token %+v", res)
//		return
//	}
//
//	if token == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//
//	code = otp.At(time.Now().Add(time.Minute * -2).Unix())
//
//	// test valid user passes an expired token
//	res, token, err = VerifyUserOtp(models.User{Otp: &secret, OtpValidated: &validated}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) != "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response contained a token %+v", res)
//		return
//	}
//
//	if token != "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned with value")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//
//	code = otp.At(time.Now().Unix())
//
//	// test a valid code with an unvalidated otp user
//	validated = false
//	res, token, err = VerifyUserOtp(models.User{ID: 2, Otp: &secret, OtpValidated: &validated, UserName: "testUser"}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || !valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response did not contain a token %+v", res)
//		return
//	}
//
//	if token == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	//query images
//	response, err := testTiDB.DB.Query("select * from users where _id = ? and user_name = ? limit 1", 2, "testUser")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	// defer closure of rows
//	defer response.Close()
//
//	//create variable to decode res into
//	var userz models.User
//
//	//// create variable to decode res into for text to image
//	//txtImg := models.SourceText{}
//
//	// load row into the first position for decoding
//	ok := response.Next()
//	if !ok {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	//decode row results
//	err = response.Scan(&userz)
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	if validated := *userz.OtpValidated; !ok || !validated {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: failed to update user with opt_validated flag")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//}

//
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	badges := []int64{1, 2}
//
//	user, err := models.CreateUser(69, "test", "testpass", "testemail",
//		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
//		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("failed to create user, err: %v", err)
//		return
//	}
//
//	defer testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
//
//	stmt := user.ToSQLNative()
//	if err != nil {
//		t.Error("\nrecc projects home failed\n    Error: ", err)
//		return
//	}
//
//	for _, s := range stmt {
//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
//		if err != nil {
//			t.Error("\nrecc projects home failed\n    Error: ", err)
//			return
//		}
//	}
//
//	uri, err := GenerateUserOtpUri(&models.User{ID: 2, UserName: "testUser"}, testTiDB)
//	if err != nil {
//		t.Errorf("failed to generate uri: %v", err)
//		return
//	}
//	if uri["otp_uri"] == "" {
//		t.Errorf("uri is empty")
//		return
//	}
//	fmt.Println(uri)
//	t.Logf("TestGenerateUserOtpUri succeeded")
//}
//
//func TestOtpData_VerifyUserOtp(t *testing.T) {
//
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}

//	secret := gotp.RandomSecret(64)
//
//	badges := []int64{1, 2}
//
//	user, err := models.CreateUser(69, "test", "testpass", "testemail",
//		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
//		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago")
//	if err != nil {
//		t.Errorf("failed to create user, err: %v", err)
//		return
//	}
//
//	defer testTiDB.DB.Exec("delete from users where user_name = ?;", "test")
//
//	stmt, err := user.ToSQLNative()
//	if err != nil {
//		t.Error("\nrecc projects home failed\n    Error: ", err)
//		return
//	}
//
//	for _, s := range stmt {
//		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
//		if err != nil {
//			t.Error("\nrecc projects home failed\n    Error: ", err)
//			return
//		}
//	}
//
//	otp := gotp.NewDefaultTOTP(secret)
//
//	code := otp.At(time.Now().Unix())
//
//	// test a valid code with a validated otp user
//	validated := true
//	res, token, err := VerifyUserOtp(&models.User{Otp: &secret, OtpValidated: &validated, UserName: "test", ID: 2}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || !valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response did not contain a token %+v", res)
//		return
//	}
//
//	if token == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//
//	code = otp.At(time.Now().Add(time.Minute * -2).Unix())
//
//	// test valid user passes an expired token
//	res, token, err = VerifyUserOtp(&models.User{Otp: &secret, OtpValidated: &validated}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) != "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response contained a token %+v", res)
//		return
//	}
//
//	if token != "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned with value")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//
//	code = otp.At(time.Now().Unix())
//
//	// test a valid code with an unvalidated otp user
//	validated = false
//	res, token, err = VerifyUserOtp(&models.User{ID: 2, Otp: &secret, OtpValidated: &validated, UserName: "testUser"}, testTiDB, code, "localhost")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: res returned as nil")
//		return
//	}
//
//	if valid, ok := res["auth"]; !ok || !valid.(bool) {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response was not authenticated %+v", res)
//		return
//	}
//
//	if token, ok := res["token"]; !ok || token.(string) == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: response did not contain a token %+v", res)
//		return
//	}
//
//	if token == "" {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	// query images
//	response, err := testTiDB.DB.Query("select * from users where _id = ? and user_name = ? limit 1", 2, "testUser")
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	// defer closure of rows
//	defer response.Close()
//
//	// create variable to decode res into
//	var userz models.User
//
//	// // create variable to decode res into for text to image
//	// txtImg := models.SourceText{}
//
//	// load row into the first position for decoding
//	ok := response.Next()
//	if !ok {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	// decode row results
//	err = response.Scan(&userz)
//	if err != nil {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: cookie token returned empty")
//		return
//	}
//
//	if validated := *userz.OtpValidated; !ok || !validated {
//		t.Errorf("\nVerifyUserOtp failed\n    Error: failed to update user with opt_validated flag")
//		return
//	}
//
//	t.Log("\nVerifyUserOtp succeeded")
//}
