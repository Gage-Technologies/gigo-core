package core

import (
	"context"
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core/query_models"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"strconv"
	"testing"
	"time"
)

func TestPastWeekActive(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestPastWeekActive failed\n    Error: %v\n", err)
		return
	}

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

	// Insert sample post data
	location, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatalf("Failed to load location: %v", err)
	}

	// Insert sample post data
	samplePost := &models.Post{
		ID:          1,
		Title:       "Test Post",
		Description: "Test Description",
		Tier:        1,
		Coffee:      69,
		Author:      "test",
		AuthorID:    user.ID,
		CreatedAt:   time.Now().Add(-6 * 24 * time.Hour), // 6 days ago
		UpdatedAt:   time.Date(1, 1, 1, 1, 1, 1, 1, location),
	}
	postStmt, err := samplePost.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert post to SQL: %v", err)
	}

	for _, stmt := range postStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample post: %v", err)
		}
	}

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		CreatedAt:   time.Now().Add(-5 * 24 * time.Hour), // 5 days ago
		UpdatedAt:   time.Date(1, 1, 1, 1, 1, 1, 1, location),
		PostID:      1,
	}
	attemptStmt, err := sampleAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample attempt: %v", err)
		}
	}

	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM users`)
		if err != nil {
			t.Logf("Failed to delete test user: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM post`)
		if err != nil {
			t.Logf("Failed to delete sample post: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM attempt`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	tests := []struct {
		name        string
		callingUser *models.User
		tidb        *ti.Database
		skip        int
		limit       int
		wantErr     bool
		wantResult  map[string]interface{}
	}{
		{
			name:        "Test past week active projects",
			callingUser: user,
			tidb:        testTiDB,
			skip:        0,
			limit:       10,
			wantErr:     false,
			wantResult: map[string]interface{}{
				"projects": []query_models.AttemptPostMergeFrontend{
					{
						PostId:      fmt.Sprintf("%v", samplePost.ID),
						PostTitle:   samplePost.Title,
						Description: samplePost.Description,
						Tier:        samplePost.Tier,
						Coffee:      fmt.Sprintf("%v", samplePost.Coffee),
						UpdatedAt:   fmt.Sprintf("%v", samplePost.UpdatedAt),
						ID:          fmt.Sprintf("%v", sampleAttempt.ID),
						Thumbnail:   fmt.Sprintf("/static/posts/t/%v", samplePost.ID),
					},
				},
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := PastWeekActive(context.Background(), tt.callingUser, tt.tidb, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("PastWeekActive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare only the most important fields of the returned result
			gotProjects, ok := gotResult["projects"].([]query_models.AttemptPostMergeFrontend)
			if !ok {
				t.Errorf("PastWeekActive() gotResult type assertion failed")
				return
			}

			wantProjects, ok := tt.wantResult["projects"].([]query_models.AttemptPostMergeFrontend)
			if !ok {
				t.Errorf("PastWeekActive() wantResult type assertion failed")
				return
			}

			for i := range gotProjects {
				if gotProjects[i].PostId != wantProjects[i].PostId ||
					gotProjects[i].PostTitle != wantProjects[i].PostTitle ||
					gotProjects[i].Description != wantProjects[i].Description ||
					gotProjects[i].Tier != wantProjects[i].Tier ||
					gotProjects[i].Coffee != wantProjects[i].Coffee {
					t.Errorf("PastWeekActive() gotProject = %v, want %v", gotProjects[i], wantProjects[i])
				}
			}
		})
	}
}

func TestMostChallengingActive(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestMostChallengingActive failed\n    Error: %v\n", err)
		return
	}

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

	// Insert sample attempt data
	sampleAttempt := &models.Attempt{
		ID:          1,
		PostTitle:   "Test Attempt",
		Description: "Test Description",
		Author:      "test",
		AuthorID:    user.ID,
		PostID:      1,
	}
	attemptStmt, err := sampleAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert attempt to SQL: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert sample attempt: %v", err)
		}
	}

	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM users`)
		if err != nil {
			t.Logf("Failed to delete test user: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM attempt`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	tests := []struct {
		name        string
		callingUser *models.User
		tidb        *ti.Database
		skip        int
		limit       int
		wantErr     bool
		wantResult  map[string]interface{}
	}{
		{
			name:        "Test most challenging active projects",
			callingUser: user,
			tidb:        testTiDB,
			skip:        0,
			limit:       10,
			wantErr:     false,
			wantResult: map[string]interface{}{
				"projects": []*models.AttemptFrontend{
					sampleAttempt.ToFrontend(),
				}, // Expected projects list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := MostChallengingActive(context.Background(), tt.callingUser, tt.tidb, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("MostChallengingActive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare only the most important fields of the returned result
			gotProjects, ok := gotResult["projects"].([]*models.AttemptFrontend)
			if !ok {
				t.Errorf("MostChallengingActive() gotResult type assertion failed")
				return
			}

			wantProjects, ok := tt.wantResult["projects"].([]*models.AttemptFrontend)
			if !ok {
				t.Errorf("MostChallengingActive() wantResult type assertion failed")
				return
			}

			for i := range gotProjects {
				if gotProjects[i].ID != wantProjects[i].ID ||
					gotProjects[i].PostTitle != wantProjects[i].PostTitle ||
					gotProjects[i].Description != wantProjects[i].Description ||
					gotProjects[i].Author != wantProjects[i].Author ||
					gotProjects[i].AuthorID != wantProjects[i].AuthorID ||
					gotProjects[i].PostID != wantProjects[i].PostID {
					t.Errorf("MostChallengingActive() gotProject = %v, want %v", gotProjects[i], wantProjects[i])
				}
			}
		})
	}
}

func TestDontGiveUpActive(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestDontGiveUpActive failed\n    Error: %v\n", err)
		return
	}

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

	// Insert sample data here

	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM users WHERE _id = ?`, user.ID)
		if err != nil {
			t.Logf("Failed to delete test user: %v", err)
		}
		// Add more cleanup if needed
	}()

	tests := []struct {
		name        string
		callingUser *models.User
		tidb        *ti.Database
		skip        int
		limit       int
		wantErr     bool
		wantResult  map[string]interface{}
	}{
		{
			name:        "Test DontGiveUpActive projects",
			callingUser: user,
			tidb:        testTiDB,
			skip:        0,
			limit:       10,
			wantErr:     false,
			wantResult: map[string]interface{}{
				"projects": []query_models.AttemptPostMergeFrontend{}, // Expected projects list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := DontGiveUpActive(context.Background(), tt.callingUser, tt.tidb, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("DontGiveUpActive() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Compare only the most important fields of the returned result
			gotProjects, ok := gotResult["projects"].([]query_models.AttemptPostMergeFrontend)
			if !ok {
				t.Errorf("DontGiveUpActive() gotResult type assertion failed")
				return
			}

			wantProjects, ok := tt.wantResult["projects"].([]query_models.AttemptPostMergeFrontend)
			if !ok {
				t.Errorf("DontGiveUpActive() wantResult type assertion failed")
				return
			}

			for i := range gotProjects {
				// Change this comparison to match the fields of your AttemptPostMergeFrontend struct
				if gotProjects[i].ID != wantProjects[i].ID {
					t.Errorf("DontGiveUpActive() gotProject = %v, want %v", gotProjects[i], wantProjects[i])
				}
			}
		}) // Insert sample attempt data
		sampleAttempt := &models.Attempt{
			ID:          1,
			PostTitle:   "Test Attempt",
			Description: "Test Description",
			Author:      "test",
			AuthorID:    user.ID,
			PostID:      1,
		}
		attemptStmt, err := sampleAttempt.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert attempt to SQL: %v", err)
		}

		for _, stmt := range attemptStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert sample attempt: %v", err)
			}
		}

		// Convert the sample attempt to AttemptPostMergeFrontend
		frontendAttempt := query_models.AttemptPostMergeFrontend{
			PostId:      strconv.FormatInt(sampleAttempt.PostID, 10),
			PostTitle:   sampleAttempt.PostTitle,
			Description: sampleAttempt.Description,
			Tier:        sampleAttempt.Tier,
			Coffee:      strconv.FormatInt(int64(sampleAttempt.Coffee), 10),
			UpdatedAt:   fmt.Sprintf("%v", sampleAttempt.UpdatedAt),
			ID:          strconv.FormatInt(sampleAttempt.ID, 10),
		}

		// Modify the test case to expect this attempt
		tests[0].wantResult = map[string]interface{}{
			"projects": []query_models.AttemptPostMergeFrontend{frontendAttempt},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				gotResult, err := DontGiveUpActive(context.Background(), tt.callingUser, tt.tidb, tt.skip, tt.limit)
				if (err != nil) != tt.wantErr {
					t.Errorf("DontGiveUpActive() error = %v, wantErr %v", err, tt.wantErr)
					return
				}

				// Compare only the most important fields of the returned result
				gotProjects, ok := gotResult["projects"].([]query_models.AttemptPostMergeFrontend)
				if !ok {
					t.Errorf("DontGiveUpActive() gotResult type assertion failed")
					return
				}

				wantProjects, ok := tt.wantResult["projects"].([]query_models.AttemptPostMergeFrontend)
				if !ok {
					t.Errorf("DontGiveUpActive() wantResult type assertion failed")
					return
				}

				for i := range gotProjects {
					// Change this comparison to match the fields of your AttemptPostMergeFrontend struct
					if gotProjects[i].ID != wantProjects[i].ID ||
						gotProjects[i].PostTitle != wantProjects[i].PostTitle ||
						gotProjects[i].Description != wantProjects[i].Description ||
						gotProjects[i].PostId != wantProjects[i].PostId {
						t.Errorf("DontGiveUpActive() gotProject = %v, want %v", gotProjects[i], wantProjects[i])
					}
				}
			})
		}
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM attempt`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()
}
