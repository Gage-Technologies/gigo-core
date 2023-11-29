package core

import (
	"context"
	"fmt"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/utils"
	"github.com/mailgun/mailgun-go/v4"
	"testing"
	"time"
)

const (
	domain                 = ""
	apiKey                 = ""
	templateName           = "passwordresetworking"
	welcomeTemplate        = "welcomemessage"
	mailGunVerificationKey = ""
)

func TestSendPasswordVerificationEmail(t *testing.T) {
	// create new Mailgun client
	mg := mailgun.NewMailgun(domain, apiKey)

	// configure verification email content
	message := mg.NewMessage("", "", "", "Insert Email to test with")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	defer cancel()

	res, err := mg.GetTemplate(ctx, templateName)
	if err != nil {
		t.Fatalf("Failed to find template%v", err)
	}

	fmt.Println(res.Name)

	// set the preconfigured email template
	message.SetTemplate(templateName)

	err = message.AddTemplateVariable("username", "Preston")
	if err != nil {
		t.Fatalf("Failed to add variables%v", err)
	}

	err = message.AddTemplateVariable("reseturl", "www.google.com")
	if err != nil {
		t.Fatalf("Failed to add variables%v", err)
	}

	// send the message
	resp, id, err := mg.Send(ctx, message)
	if err != nil {
		if resp != "" {
			fmt.Println("Message is : " + resp)
		}
		if id != "" {
			fmt.Println("Id is : " + id)
		}
		t.Fatalf("Failed to send email%v", err)
	}

	fmt.Println("Message is : " + resp)
	fmt.Println("Id is : " + id + "\nSuccessfully sent email")
}

func TestSendSignUpMessage(t *testing.T) {
	// create new Mailgun client
	mg := mailgun.NewMailgun(domain, apiKey)

	// configure verification email content
	message := mg.NewMessage("GIGO", "", "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	defer cancel()

	// set the preconfigured email template
	message.SetTemplate(welcomeTemplate)

	err := message.AddTemplateVariable("username", "Preston")
	if err != nil {
		t.Fatalf("Failed to add variables%v", err)
	}

	// send the message
	resp, id, err := mg.Send(ctx, message)
	if err != nil {
		if resp != "" {
			fmt.Println("Message is : " + resp)
		}
		if id != "" {
			fmt.Println("Id is : " + id)
		}
		t.Fatalf("Failed to send email%v", err)
	}

	fmt.Println("Message is : " + resp)
	fmt.Println("Id is : " + id + "\nSuccessfully sent email")
}

func TestListActiveTemplates(t *testing.T) {
	// create new Mailgun client
	mg := mailgun.NewMailgun(domain, apiKey)

	res, err := ListActiveTemplates(mg)
	if err != nil {
		t.Fatalf("TestListActiveTemplates Failed : %v", err)
	}

	if res == nil {
		t.Fatalf("TestListActiveTemplates Failed : %v", err)
	}

	fmt.Println(fmt.Sprintf("%v", res))
}

func TestListTemplateVersions(t *testing.T) {
	// create new Mailgun client
	mg := mailgun.NewMailgun(domain, apiKey)

	it := mg.ListTemplateVersions(templateName, nil)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()

	var page, result []mailgun.TemplateVersion

	for it.Next(ctx, &page) {
		result = append(result, page...)

		fmt.Println(fmt.Sprintf("%v", page))
	}

	if result != nil {
		for res := range result {
			fmt.Println(fmt.Sprintf("%v", res))
		}
	}
}

func TestVerifyEmailToken(t *testing.T) {
	t.Helper()

	// Create a test TiDB database
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}
	defer testTiDB.DB.Close()

	testUser, err := models.CreateUser(
		4206969,
		"test",
		"test",
		"test",
		"1234567891",
		models.UserStatusBasic,
		"",
		nil,
		nil,
		"test",
		"test",
		0,
		"None",
		models.UserStart{},
		"America/Chicago",
		models.AvatarSettings{},
		0,
	)
	if err != nil {
		t.Fatal("TestVerifyEmailToken test failed:", err)
		return
	}
	statements, err := testUser.ToSQLNative()
	if err != nil {
		t.Fatal("TestVerifyEmailToken test failed:", err)
		return
	}

	for _, statement := range statements {
		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Fatal("TestVerifyEmailToken test failed:", err)
		}
	}

	token, err := utils.GenerateEmailToken()
	if err != nil {
		t.Fatal("TestVerifyEmailToken test failed:", err)
		return
	}

	_, err = testTiDB.DB.Exec("update users set reset_token = ? where _id = ?", token, testUser.ID)
	if err != nil {
		t.Fatal("TestVerifyEmailToken test failed:", err)
	}

	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM users where _id = ?`, testUser.ID)
		if err != nil {
			t.Logf("Failed to delete test user: %v", err)
		}
	}()

	// Call the function being tested
	response, err := VerifyEmailToken(context.Background(), testTiDB, fmt.Sprintf("%v", testUser.ID), token)
	if err != nil {
		t.Fatalf("Failed to verify email token: %v", err)
	}

	// Assert the expected response
	expectedKey := "message"
	val, ok := response[expectedKey]
	if !ok {
		t.Fatalf("Expected key '%s' in response, but not found", expectedKey)
	}

	// Assert the type and value of the response
	message, ok := val.(string)
	if !ok {
		t.Fatalf("Expected value of type string, but got %T", val)
	}

	expectedMessage := "Token Validated"
	if message != expectedMessage {
		t.Fatalf("Expected message '%s', but got '%s'", expectedMessage, message)
	}

	t.Log("TestVerifyEmailToken succeeded")
}

func TestEmailVerification(t *testing.T) {
	validator := mailgun.NewEmailValidator(mailGunVerificationKey)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	email, err := validator.ValidateEmail(ctx, "", true)
	if err != nil {
		t.Fatalf("valiudate email failed error: %v", err)
	}

	fmt.Println("IsValid : " + fmt.Sprintf("%v", email.IsValid))

	fmt.Println(email.Address)

	fmt.Println("result : " + email.Result)

	flag := false

	if email.MailboxVerification == "unknown" || email.MailboxVerification == "false" {
		fmt.Println("mailbox verification : " + email.MailboxVerification)
		flag = false
	} else if email.MailboxVerification == "true" {
		flag = true
		fmt.Println("flag is true : " + email.MailboxVerification)
	}

}
