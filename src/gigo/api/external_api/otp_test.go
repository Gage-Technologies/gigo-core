package external_api

import (
	"bytes"
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHTTPServer_GenerateUserOtpUri(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")

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

	body := bytes.NewReader([]byte(`{"test":true}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/otp/generateUserOtpUri", body)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nadd feedback HTTP failed\n    Error: incorrect response code")
		return
	}

	t.Log("\ngenerate user otp uri HTTP succeeded")
}

func TestHTTPServer_VerifyUserOtp(t *testing.T) {
	badges := []int64{1, 2}

	user, err := models.CreateUser(69, "test", "testpass", "testemail",
		"phone", models.UserStatusBasic, "fryin with jigsaw", badges,
		[]int64{1, 2, 3}, "test", "test", 69420, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("failed to create user, err: %v", err)
		return
	}

	defer testHttpServer.tiDB.DB.Exec("delete from users where user_name = ?;", "test")

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

	body := bytes.NewReader([]byte(`{"test":true, "otp_code": "123456"}`))
	req, err := http.NewRequest("POST", "http://localhost:1818/api/otp/validate", body)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nadd feedback HTTP failed\n    Error: %v", err)
		return
	}

	if res.StatusCode != http.StatusOK {
		fmt.Println(res.StatusCode)
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		t.Error("\nadd feedback HTTP failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nverify user otp uri HTTP succeeded")
}
