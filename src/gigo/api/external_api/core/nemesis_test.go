package core

import (
	"context"
	"fmt"
	config2 "github.com/gage-technologies/gigo-lib/config"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/gage-technologies/gigo-lib/mq"
)

// func TestDeclareNemesis(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	sf, err := snowflake.NewNode(0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
//		Host:        "mq://gigo-dev-nats:4222",
//		Username:    "gigo-dev",
//		Password:    "gigo-dev",
//		MaxPubQueue: 256,
//	}, logger)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	endTime := time.Now().Add(48 * time.Hour)
//
// 	// Call the DeclareNemesis function
// 	result, err := DeclareNemesis(testTiDB, sf, js, testUser1.ID, testUser2.UserName, &endTime)
// 	if err != nil {
// 		t.Errorf("DeclareNemesis() error = %v", err)
// 		return
// 	}
//
// 	nemesisRequest, ok := result["nemesis_request"].(*models.Nemesis)
// 	if !ok {
// 		t.Fatalf("DeclareNemesis() result did not contain a nemesis_request")
// 	}
//
// 	if nemesisRequest.AntagonistID != testUser1.ID || nemesisRequest.ProtagonistID != testUser2.ID {
// 		t.Errorf("DeclareNemesis() returned incorrect nemesis request data")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = ?", nemesisRequest.ID)
// 	}()
// }
//
// func TestAcceptNemesis(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	sf, err := snowflake.NewNode(0)
// 	if err != nil {
// 		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
//		Host:        "mq://gigo-dev-nats:4222",
//		Username:    "gigo-dev",
//		Password:    "gigo-dev",
//		MaxPubQueue: 256,
//	}, logger)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	// Call the DeclareNemesis function to create a nemesis request
// 	endTime := time.Now().Add(48 * time.Hour)
// 	_, _ = DeclareNemesis(testTiDB, sf, js, testUser1.ID, testUser2.UserName, &endTime)
//
// 	// Call the AcceptNemesis function
// 	err = AcceptNemesis(testTiDB, sf, js, testUser2.ID, testUser1.ID)
// 	if err != nil {
// 		t.Errorf("AcceptNemesis() error = %v", err)
// 		return
// 	}
// 	// Check if the nemesis request has been accepted
// 	var isAccepted bool
// 	err = testTiDB.DB.QueryRow("SELECT is_accepted FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID).Scan(&isAccepted)
// 	if err != nil {
// 		t.Errorf("Failed to query for nemesis acceptance status: %v", err)
// 		return
// 	}
//
// 	if !isAccepted {
// 		t.Errorf("AcceptNemesis() did not set is_accepted to true")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
// 	}()
// }
//
// func TestDeclineNemesis(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	sf, err := snowflake.NewNode(0)
// 	if err != nil {
// 		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
//		Host:        "mq://gigo-dev-nats:4222",
//		Username:    "gigo-dev",
//		Password:    "gigo-dev",
//		MaxPubQueue: 256,
//	}, logger)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	endTime := time.Now().Add(48 * time.Hour)
//
// 	// Call the DeclareNemesis function to create a nemesis request
// 	_, err = DeclareNemesis(testTiDB, sf, js, testUser1.ID, testUser2.UserName, &endTime)
// 	if err != nil {
// 		t.Errorf("DeclareNemesis() error = %v", err)
// 		return
// 	}
//
// 	// Call the DeclineNemesis function
// 	err = DeclineNemesis(testTiDB, sf, js, testUser2.ID, testUser1.ID)
// 	if err != nil {
// 		t.Errorf("DeclineNemesis() error = %v", err)
// 		return
// 	}
// 	// Check if the nemesis request has been removed from the database
// 	var nemesisCount int
// 	err = testTiDB.DB.QueryRow("SELECT COUNT(*) FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID).Scan(&nemesisCount)
// 	if err != nil {
// 		t.Errorf("Failed to query nemesis count: %v", err)
// 		return
// 	}
//
// 	if nemesisCount != 0 {
// 		t.Errorf("DeclineNemesis() failed to remove nemesis request from the database")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
// 	}()
// }
//
// func TestGetActiveNemesis(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	acceptedNemesisReq := models.Nemesis{
// 		ID:             1,
// 		AntagonistID:   testUser1.ID,
// 		AntagonistName: testUser1.UserName, // Add this line
// 		ProtagonistID:  testUser2.ID,
// 		IsAccepted:     true,
// 		EndTime:        nil,
// 	}
//
// 	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()
//
// 	for _, stmt := range nemesisReqStmt {
// 		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 		if err != nil {
// 			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
// 		}
// 	}
//
// 	// Call the GetActiveNemesis function
// 	result, err := GetActiveNemesis(testTiDB, testUser2.ID)
// 	if err != nil {
// 		t.Errorf("GetActiveNemesis() error = %v", err)
// 		return
// 	}
//
// 	activeNemesis, ok := result["nemesis"].([]*models.NemesisFrontend)
// 	if !ok {
// 		t.Fatalf("GetActiveNemesis() result did not contain a nemesis slice")
// 	}
//
// 	if len(activeNemesis) != 1 || activeNemesis[0].AntagonistID != fmt.Sprintf("%v", testUser1.ID) || activeNemesis[0].ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
// 		t.Errorf("GetActiveNemesis() returned incorrect nemesis data")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
// 	}()
// }
//
// func TestGetPendingNemesisRequests(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetPendingNemesisRequests failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetPendingNemesisRequests failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	pendingNemesisReq := models.Nemesis{
// 		ID:             1,
// 		AntagonistID:   testUser1.ID,
// 		AntagonistName: testUser1.UserName, // Add this line
// 		ProtagonistID:  testUser2.ID,
// 		IsAccepted:     false,
// 		EndTime:        nil,
// 	}
//
// 	nemesisReqStmt := pendingNemesisReq.ToSQLNative()
//
// 	for _, stmt := range nemesisReqStmt {
// 		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 		if err != nil {
// 			t.Fatalf("Failed to insert pending nemesis request: %v", err)
// 		}
// 	}
//
// 	// Call the GetPendingNemesisRequests function
// 	result, err := GetPendingNemesisRequests(testTiDB, testUser2.ID)
// 	if err != nil {
// 		t.Errorf("GetPendingNemesisRequests() error = %v", err)
// 		return
// 	}
//
// 	pendingNemesisRequests, ok := result["nemesis"].([]*models.NemesisFrontend)
// 	if !ok {
// 		t.Fatalf("GetPendingNemesisRequests() result did not contain a nemesis slice")
// 	}
//
// 	if len(pendingNemesisRequests) != 1 || pendingNemesisRequests[0].AntagonistID != fmt.Sprintf("%v", testUser1.ID) || pendingNemesisRequests[0].ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
// 		t.Errorf("GetPendingNemesisRequests() returned incorrect nemesis data")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
// 	}()
// }
//
// func TestGetNemesisBattleground(t *testing.T) {
// testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	acceptedNemesisReq := models.Nemesis{
// 		ID:            1,
// 		AntagonistID:  testUser1.ID,
// 		ProtagonistID: testUser2.ID,
// 		IsAccepted:    true,
// 		EndTime:       nil,
// 	}
//
// 	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()
//
// 	for _, stmt := range nemesisReqStmt {
// 		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 		if err != nil {
// 			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
// 		}
// 	}
//
// 	// Call the GetNemesisBattleground function
// 	result, err := GetNemesisBattleground(testTiDB, acceptedNemesisReq.ID, testUser1.ID, testUser2.ID)
// 	if err != nil {
// 		t.Errorf("GetNemesisBattleground() error = %v", err)
// 		return
// 	}
//
// 	battleground, ok := result["battleground"].(Battleground)
// 	if !ok {
// 		t.Fatalf("GetNemesisBattleground() result did not contain a Battleground")
// 	}
//
// 	if battleground.AntagonistID != fmt.Sprintf("%v", testUser1.ID) || battleground.ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
// 		t.Errorf("GetNemesisBattleground() returned incorrect nemesis data")
// 	}
//
// 	// Deferred removal of inserted data
// 	defer func() {
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
// 		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
// 	}()
// }
//
// func TestRecentNemesisBattleground(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	acceptedNemesisReq := models.Nemesis{
// 		ID:                        1,
// 		AntagonistID:              testUser1.ID,
// 		ProtagonistID:             testUser2.ID,
// 		TimeOfVillainy:            time.Now(),
// 		AntagonistTowersCaptured:  5,
// 		ProtagonistTowersCaptured: 3,
// 		IsAccepted:                true,
// 	}
//
// 	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()
// 	if err != nil {
// 		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
// 	}
//
// 	for _, stmt := range nemesisReqStmt {
// 		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 		if err != nil {
// 			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
// 		}
// 	}
//
// 	acceptedNemesisHistory := models.NemesisHistory{
// 		ID:            1,
// 		AntagonistID:  testUser1.ID,
// 		ProtagonistID: testUser2.ID,
// 	}
//
// 	nemesisHistoryStmt := acceptedNemesisHistory.ToSQLNative()
// 	if err != nil {
// 		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
// 	}
//
// 	_, err = testTiDB.DB.Exec(nemesisHistoryStmt.Statement, nemesisHistoryStmt.Values...)
// 	if err != nil {
// 		t.Fatalf("Failed to insert accepted nemesis request: %v", err)
// 	}
//
// 	// Call the function
// 	result, err := RecentNemesisBattleground(testTiDB, 1)
// 	if err != nil {
// 		t.Fatalf("failed to call RecentNemesisBattleground: %v", err)
// 	}
//
// 	// Verify the result
// 	battle, ok := result["battleground"].(Battleground)
// 	if !ok {
// 		t.Fatal("failed to cast battleground")
// 	}
//
// 	if battle.AntagonistID != "1" {
// 		t.Errorf("expected antagonist ID to be 1, but got %s", battle.AntagonistID)
// 	}
//
// 	if battle.ProtagonistID != "2" {
// 		t.Errorf("expected protagonist ID to be 2, but got %s", battle.ProtagonistID)
// 	}
//
// 	if battle.AntagTowersTaken != 5 {
// 		t.Errorf("expected antagonist towers taken to be 5, but got %d", battle.AntagTowersTaken)
// 	}
//
// 	if battle.ProtagTowersTaken != 3 {
// 		t.Errorf("expected protagonist towers taken to be 3, but got %d", battle.ProtagTowersTaken)
// 	}
// }
//
// func TestWarHistory(t *testing.T) {
// 	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
// 	// Create test users
// 	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
// 	if err != nil {
// 		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
// 		return
// 	}
//
// 	for _, user := range []*models.User{testUser1, testUser2} {
// 		userStmt, err := user.ToSQLNative()
// 		if err != nil {
// 			t.Fatalf("Failed to convert user to SQL: %v", err)
// 		}
//
// 		for _, stmt := range userStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert test user: %v", err)
// 			}
// 		}
// 	}
//
// 	// Insert test data
// 	nemesisMatches := []models.Nemesis{
// 		{
// 			ID:                        1,
// 			AntagonistID:              testUser1.ID,
// 			ProtagonistID:             testUser2.ID,
// 			Victor:                    &testUser1.ID,
// 			AntagonistTowersCaptured:  5,
// 			ProtagonistTowersCaptured: 3,
// 		},
// 		{
// 			ID:                        2,
// 			AntagonistID:              testUser1.ID,
// 			ProtagonistID:             testUser2.ID,
// 			Victor:                    &testUser2.ID,
// 			AntagonistTowersCaptured:  2,
// 			ProtagonistTowersCaptured: 7,
// 		},
// 	}
//
// 	for _, match := range nemesisMatches {
// 		nemesisStmt := match.ToSQLNative()
//
// 		for _, stmt := range nemesisStmt {
// 			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
// 			if err != nil {
// 				t.Fatalf("Failed to insert nemesis match: %v", err)
// 			}
// 		}
// 	}
//
// 	nemesisHistoryData := []models.NemesisHistory{
// 		{
// 			MatchID:            1,
// 			AntagonistTotalXP:  120,
// 			ProtagonistTotalXP: 80,
// 		},
// 		{
// 			MatchID:            2,
// 			AntagonistTotalXP:  70,
// 			ProtagonistTotalXP: 130,
// 		},
// 	}
//
// 	for _, history := range nemesisHistoryData {
// 		historyStmt := history.ToSQLNative()
//
// 		_, err = testTiDB.DB.Exec(historyStmt.Statement, historyStmt.Values...)
// 		if err != nil {
// 			t.Fatalf("Failed to insert nemesis history: %v", err)
// 		}
// 	}
//
// 	// Call the function
// 	result, err := WarHistory(testTiDB, testUser1.ID)
// 	if err != nil {
// 		t.Fatalf("failed to call WarHistory: %v", err)
// 	}
//
// 	// Verify the result
// 	history, ok := result["history"].([]*History)
// 	if !ok {
// 		t.Fatal("failed to cast history")
// 	}
//
// 	if len(history) != 2 {
// 		t.Fatalf("expected 2 nemesis matches in history, but got %d", len(history))
// 	}
//
// 	for _, match := range history {
// 		if match.Victor != fmt.Sprintf("%v", testUser1.ID) && match.ProtagonistName != testUser1.UserName {
// 			t.Errorf("expected one of the match user IDs to be %d, but got antagonist ID %v and protagonist ID %v", testUser1.ID, match.AntagonistName, match.ProtagonistName)
// 		}
// 	}
// }

func TestDeclareNemesis(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestDeclareNemesis failed\n    Error: %v\n", err)
		return
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
		Host:        "mq://gigo-dev-nats:4222",
		Username:    "gigo-dev",
		Password:    "gigo-dev",
		MaxPubQueue: 256,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Call the DeclareNemesis function
	result, err := DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)
	if err != nil {
		t.Errorf("DeclareNemesis() error = %v", err)
		return
	}

	// nemesisRequest, ok := result["nemesis_request"].(*models.Nemesis)
	// if !ok {
	//	t.Fatalf("DeclareNemesis() result did not contain a nemesis_request")
	// }

	if result["declare_nemesis"] == nil {
		t.Errorf("DeclareNemesis() returned nil nemesis request data")
	}

	if !(result["declare_nemesis"].(string) == "request sent") || !(result["declare_nemesis"].(string) != "request already sent") {
		t.Errorf("DeclareNemesis() returned incorrect nemesis request data")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis where antagonist_id = ? and protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()
}

func TestAcceptNemesis(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestAcceptNemesis failed\n    Error: %v\n", err)
		return
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
		Host:        "mq://gigo-dev-nats:4222",
		Username:    "gigo-dev",
		Password:    "gigo-dev",
		MaxPubQueue: 256,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Call the DeclareNemesis function to create a nemesis request
	_, _ = DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)

	// Call the AcceptNemesis function
	err = AcceptNemesis(context.Background(), testTiDB, sf, js, testUser2.ID, testUser1.ID)
	if err != nil {
		t.Errorf("AcceptNemesis() error = %v", err)
		return
	}
	// Check if the nemesis request has been accepted
	var isAccepted bool
	err = testTiDB.DB.QueryRow("SELECT is_accepted FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID).Scan(&isAccepted)
	if err != nil {
		t.Errorf("Failed to query for nemesis acceptance status: %v", err)
		return
	}

	if !isAccepted {
		t.Errorf("AcceptNemesis() did not set is_accepted to true")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()
}

func TestDeclineNemesis(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	sf, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestDeclineNemesis failed\n    Error: %v\n", err)
		return
	}

	logger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-core-test.log"))
	if err != nil {
		t.Fatal(err)
	}

	js, err := mq.NewJetstreamClient(config2.JetstreamConfig{
		Host:        "mq://gigo-dev-nats:4222",
		Username:    "gigo-dev",
		Password:    "gigo-dev",
		MaxPubQueue: 256,
	}, logger)
	if err != nil {
		t.Fatal(err)
	}

	// Call the DeclareNemesis function to create a nemesis request
	_, err = DeclareNemesis(context.Background(), testTiDB, sf, js, testUser1.ID, testUser2.ID)
	if err != nil {
		t.Errorf("DeclareNemesis() error = %v", err)
		return
	}

	// Call the DeclineNemesis function
	err = DeclineNemesis(context.Background(), testTiDB, sf, js, testUser2.ID, testUser1.ID)
	if err != nil {
		t.Errorf("DeclineNemesis() error = %v", err)
		return
	}
	// Check if the nemesis request has been removed from the database
	var nemesisCount int
	err = testTiDB.DB.QueryRow("SELECT COUNT(*) FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID).Scan(&nemesisCount)
	if err != nil {
		t.Errorf("Failed to query nemesis count: %v", err)
		return
	}

	if nemesisCount != 0 {
		t.Errorf("DeclineNemesis() failed to remove nemesis request from the database")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE antagonist_id = ? AND protagonist_id = ?", testUser1.ID, testUser2.ID)
	}()
}

func TestGetActiveNemesis(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetActiveNemesis failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	acceptedNemesisReq := models.Nemesis{
		ID:             1,
		AntagonistID:   testUser1.ID,
		AntagonistName: testUser1.UserName, // Add this line
		ProtagonistID:  testUser2.ID,
		IsAccepted:     true,
		EndTime:        nil,
	}

	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()

	for _, stmt := range nemesisReqStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	// Call the GetActiveNemesis function
	result, err := GetActiveNemesis(context.Background(), testTiDB, testUser2.ID)
	if err != nil {
		t.Errorf("GetActiveNemesis() error = %v", err)
		return
	}

	activeNemesis, ok := result["nemesis"].([]*models.NemesisFrontend)
	if !ok {
		t.Fatalf("GetActiveNemesis() result did not contain a nemesis slice")
	}

	if len(activeNemesis) != 1 || activeNemesis[0].AntagonistID != fmt.Sprintf("%v", testUser1.ID) || activeNemesis[0].ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
		t.Errorf("GetActiveNemesis() returned incorrect nemesis data")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
	}()
}

// func TestGetPendingNemesisRequests(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	// Create test users
//	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nTestGetPendingNemesisRequests failed\n    Error: %v\n", err)
//		return
//	}
//
//	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Errorf("\nTestGetPendingNemesisRequests failed\n    Error: %v\n", err)
//		return
//	}
//
//	for _, user := range []*models.User{testUser1, testUser2} {
//		userStmt, err := user.ToSQLNative()
//		if err != nil {
//			t.Fatalf("Failed to convert user to SQL: %v", err)
//		}
//
//		for _, stmt := range userStmt {
//			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//			if err != nil {
//				t.Fatalf("Failed to insert test user: %v", err)
//			}
//		}
//	}
//
//	pendingNemesisReq := models.Nemesis{
//		ID:             1,
//		AntagonistID:   testUser1.ID,
//		AntagonistName: testUser1.UserName, // Add this line
//		ProtagonistID:  testUser2.ID,
//		IsAccepted:     false,
//		EndTime:        nil,
//	}
//
//	nemesisReqStmt := pendingNemesisReq.ToSQLNative()
//
//	for _, stmt := range nemesisReqStmt {
//		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
//		if err != nil {
//			t.Fatalf("Failed to insert pending nemesis request: %v", err)
//		}
//	}
//
//	// Call the GetPendingNemesisRequests function
//	result, err := GetPendingNemesisRequests(testTiDB, testUser2.ID)
//	if err != nil {
//		t.Errorf("GetPendingNemesisRequests() error = %v", err)
//		return
//	}
//
//	pendingNemesisRequests, ok := result["nemesis"].([]*models.NemesisFrontend)
//	if !ok {
//		t.Fatalf("GetPendingNemesisRequests() result did not contain a nemesis slice")
//	}
//
//	if len(pendingNemesisRequests) != 1 || pendingNemesisRequests[0].AntagonistID != fmt.Sprintf("%v", testUser1.ID) || pendingNemesisRequests[0].ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
//		t.Errorf("GetPendingNemesisRequests() returned incorrect nemesis data")
//	}
//
//	// Deferred removal of inserted data
//	defer func() {
//		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
//		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
//		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
//	}()
// }

func TestGetNemesisBattleground(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	acceptedNemesisReq := models.Nemesis{
		ID:            1,
		AntagonistID:  testUser1.ID,
		ProtagonistID: testUser2.ID,
		IsAccepted:    true,
		EndTime:       nil,
	}

	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()

	for _, stmt := range nemesisReqStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	// Call the GetNemesisBattleground function
	result, err := GetNemesisBattleground(context.Background(), testTiDB, acceptedNemesisReq.ID, testUser1.ID, testUser2.ID)
	if err != nil {
		t.Errorf("GetNemesisBattleground() error = %v", err)
		return
	}

	battleground, ok := result["battleground"].(Battleground)
	if !ok {
		t.Fatalf("GetNemesisBattleground() result did not contain a Battleground")
	}

	if battleground.AntagonistID != fmt.Sprintf("%v", testUser1.ID) || battleground.ProtagonistID != fmt.Sprintf("%v", testUser2.ID) {
		t.Errorf("GetNemesisBattleground() returned incorrect nemesis data")
	}

	// Deferred removal of inserted data
	defer func() {
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 1")
		_, _ = testTiDB.DB.Exec("DELETE FROM user WHERE _id = 2")
		_, _ = testTiDB.DB.Exec("DELETE FROM nemesis WHERE _id = 1")
	}()
}

func TestRecentNemesisBattleground(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	acceptedNemesisReq := models.Nemesis{
		ID:                        1,
		AntagonistID:              testUser1.ID,
		ProtagonistID:             testUser2.ID,
		TimeOfVillainy:            time.Now(),
		AntagonistTowersCaptured:  5,
		ProtagonistTowersCaptured: 3,
		IsAccepted:                true,
	}

	nemesisReqStmt := acceptedNemesisReq.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
	}

	for _, stmt := range nemesisReqStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	acceptedNemesisHistory := models.NemesisHistory{
		ID:            1,
		AntagonistID:  testUser1.ID,
		ProtagonistID: testUser2.ID,
	}

	nemesisHistoryStmt := acceptedNemesisHistory.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert accepted nemesis request to SQL: %v", err)
	}

	for _, stmt := range nemesisHistoryStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert accepted nemesis request: %v", err)
		}
	}

	// Call the function
	result, err := RecentNemesisBattleground(context.Background(), testTiDB, 1)
	if err != nil {
		t.Fatalf("failed to call RecentNemesisBattleground: %v", err)
	}

	// Verify the result
	battle, ok := result["battleground"].(Battleground)
	if !ok {
		t.Fatal("failed to cast battleground")
	}

	if battle.AntagonistID != "1" {
		t.Errorf("expected antagonist ID to be 1, but got %s", battle.AntagonistID)
	}

	if battle.ProtagonistID != "2" {
		t.Errorf("expected protagonist ID to be 2, but got %s", battle.ProtagonistID)
	}

	if battle.AntagTowersTaken != 5 {
		t.Errorf("expected antagonist towers taken to be 5, but got %d", battle.AntagTowersTaken)
	}

	if battle.ProtagTowersTaken != 3 {
		t.Errorf("expected protagonist towers taken to be 3, but got %d", battle.ProtagTowersTaken)
	}
}

func TestWarHistory(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Create test users
	testUser1, err := models.CreateUser(1, "antagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	testUser2, err := models.CreateUser(2, "protagonist", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestGetNemesisBattleground failed\n    Error: %v\n", err)
		return
	}

	for _, user := range []*models.User{testUser1, testUser2} {
		userStmt, err := user.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert user to SQL: %v", err)
		}

		for _, stmt := range userStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test user: %v", err)
			}
		}
	}

	// Insert test data
	nemesisMatches := []models.Nemesis{
		{
			ID:                        1,
			AntagonistID:              testUser1.ID,
			ProtagonistID:             testUser2.ID,
			Victor:                    &testUser1.ID,
			AntagonistTowersCaptured:  5,
			ProtagonistTowersCaptured: 3,
		},
		{
			ID:                        2,
			AntagonistID:              testUser1.ID,
			ProtagonistID:             testUser2.ID,
			Victor:                    &testUser2.ID,
			AntagonistTowersCaptured:  2,
			ProtagonistTowersCaptured: 7,
		},
	}

	for _, match := range nemesisMatches {
		nemesisStmt := match.ToSQLNative()

		for _, stmt := range nemesisStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert nemesis match: %v", err)
			}
		}
	}

	nemesisHistoryData := []models.NemesisHistory{
		{
			MatchID:            1,
			AntagonistTotalXP:  120,
			ProtagonistTotalXP: 80,
		},
		{
			MatchID:            2,
			AntagonistTotalXP:  70,
			ProtagonistTotalXP: 130,
		},
	}

	for _, history := range nemesisHistoryData {
		historyStmt := history.ToSQLNative()

		for _, stmt := range historyStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert nemesis history: %v", err)
			}
		}
	}

	// Call the function
	result, err := WarHistory(context.Background(), testTiDB, testUser1.ID)
	if err != nil {
		t.Fatalf("failed to call WarHistory: %v", err)
	}

	// Verify the result
	history, ok := result["history"].([]*History)
	if !ok {
		t.Fatal("failed to cast history")
	}

	if len(history) != 1 {
		t.Fatalf("expected 2 nemesis matches in history, but got %d", len(history))
	}

}
