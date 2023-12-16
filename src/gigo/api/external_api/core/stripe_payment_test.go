package core

import (
	"context"
	"testing"
	"time"

	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/go-redis/redis/v8"
)

func TestCreateProduct(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	id := int64(5)
	post, err := models.CreatePost(
		69420, "test", "content", "autor", 42069, time.Now(),
		time.Now(), 69, 1, []int64{}, &id, 6969, 20, 40, 24, 27,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
		64752, 3, &models.DefaultWorkspaceSettings, false, false, nil,
	)
	if err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}
	defer testTiDB.DB.Exec("delete from post where _id = ?;", post.ID)

	postStmt, err := post.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		if _, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test post: %v", err)
		}
	}
	user := models.User{ID: 69420, UserName: "test"}

	res, err := CreateProduct(context.Background(), 400, testTiDB, 36, &user)
	if err != nil {
		t.Fatalf("Failed to create product: %v", err)
	}

	if res["message"].(string) != "Product has been created" {
		t.Fatalf("Failed to create product: %v", err)
	}
	t.Logf("Created product function success")
}

func TestGetProjectPriceId(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	id := int64(5)
	stripeId := "420"
	post, err := models.CreatePost(
		69420, "test", "content", "autor", 42069, time.Now(),
		time.Now(), 69, 1, []int64{}, &id, 6969, 20, 40, 24, 27,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, &stripeId,
		64752, 3, &models.DefaultWorkspaceSettings, false, false, nil,
	)
	if err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}
	defer testTiDB.DB.Exec("delete from post where _id = ?;", post.ID)

	postStmt, err := post.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		if _, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...); err != nil {
			t.Fatalf("Failed to insert test post: %v", err)
		}
	}

	res, err := GetProjectPriceId(context.Background(), 69420, testTiDB)
	if err != nil {
		t.Fatalf("Failed to get price id: %v", err)
	}

	if res["priceId"].(string) != stripeId {
		t.Fatalf("Failed to get price id: %v", err)
	}
	t.Logf("get price id function success")
}

func TestCancelSubscription(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusPremium, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nCancel Subscription failed\n    Error: %v\n", err)
		return
	}

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\ncancel subsciption failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\ncancel subscription failed\n    Error: ", err)
			return
		}
	}

	res, err := CancelSubscription(context.Background(), testTiDB, user)
	if err != nil {
		t.Fatalf("Failed to cancel subscription: %v", err)
	}

	if res["subscription"].(string) != "cancelled" {
		t.Fatalf("Failed to cancel subscription: %v", err)
	}
	t.Logf("cancel subscription function success")
}

func TestCreateSubscription(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "testEmail", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
	if err != nil {
		t.Errorf("\nCreate Subscription failed\n    Error: %v\n", err)
		return
	}

	stmt, err := user.ToSQLNative()
	if err != nil {
		t.Error("\ncancel subsciption failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nCreate Subscription failed\n    Error: ", err)
			return
		}
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	res, err := CreateSubscription(context.Background(), "testStripeUser", "testStripeSubscription", false, testTiDB, "testEmail", testRdb, "America/Chicago")
	if err != nil {
		t.Fatalf("Failed to Create Subscription: %v", err)
	}

	if res["subscription"].(string) != "subscription paid" {
		t.Fatalf("Failed to Create Subscription: %v", err)
	}
	t.Logf("Create Subscription function success")
}
