package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"reflect"
	"strconv"
	"testing"
)

func TestStartByteAttempt(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	testByteAttempt := models.ByteAttempts{
		ID:            1,
		ByteID:        2,
		AuthorID:      3,
		ContentEasy:   "Test Byte Attempt Easy",
		ContentMedium: "Test Byte Attempt Medium",
		ContentHard:   "Test Byte Attempt Hard",
	}

	attemptStmt, err := testByteAttempt.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert byte attempt to SQL native: %v", err)
	}

	for _, stmt := range attemptStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test byte attempt: %v", err)
		}
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0, nil)
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

	defer func() {
		_, err = testTiDB.DB.Exec("delete from users")
		if err != nil {
			t.Fatalf("Failed to cleanup test user: %v", err)
		}
	}()

	tests := []struct {
		name        string
		ctx         context.Context
		tidb        *ti.Database
		sf          *snowflake.Node
		callingUser *models.User
		byteId      int64
		wantErr     bool
		wantResult  map[string]interface{}
	}{
		{
			name:        "Test StartByteAttempt with new attempt",
			ctx:         context.Background(),
			tidb:        testTiDB,
			sf:          &snowflake.Node{},
			callingUser: user,
			byteId:      1, // Example byteId
			wantErr:     false,
			wantResult:  map[string]interface{}{"message": "Attempt created successfully."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := StartByteAttempt(tt.ctx, tt.tidb, tt.sf, tt.callingUser, tt.byteId)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartByteAttempt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if gotMessage, ok := gotResult["message"]; ok {
				if gotMessage != "Attempt created successfully." {
					t.Errorf("StartByteAttempt() gotMessage = %v, want 'Attempt created successfully.'", gotMessage)
				}
			} else {
				t.Error("StartByteAttempt() did not return a message")
			}
		})
	}

	defer func() {
		_, err = testTiDB.DB.Exec("DELETE FROM byte_attempts")
		if err != nil {
			t.Fatalf("Failed to cleanup test byte attempts: %v", err)
		}
	}()

}

func TestGetRecommendedBytes(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	expectedBytesFrontend := make([]*models.BytesFrontend, 0)

	// Insert test byte data into the database
	for i := 0; i < 50; i++ {
		// Create a Byte instance for insertion
		testByte := &models.Bytes{
			ID:                int64(i + 1),
			Name:              fmt.Sprintf("Byte %d", i+1),
			DescriptionEasy:   fmt.Sprintf("Description for Byte %d Easy", i+1),
			DescriptionMedium: fmt.Sprintf("Description for Byte %d Medium", i+1),
			DescriptionHard:   fmt.Sprintf("Description for Byte %d Hard", i+1),
		}
		// Assuming ToSQLNative is the method to prepare SQL insertion
		byteStmt, err := testByte.ToSQLNative()
		if err != nil {
			t.Fatalf("Failed to convert byte to SQL: %v", err)
		}
		for _, stmt := range byteStmt {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Fatalf("Failed to insert test byte: %v", err)
			}
		}

		// Add corresponding BytesFrontend instance to expected result
		expectedByteFrontend := &models.BytesFrontend{
			ID:                strconv.FormatInt(testByte.ID, 10),
			Name:              testByte.Name,
			DescriptionEasy:   testByte.DescriptionEasy,
			DescriptionMedium: testByte.DescriptionMedium,
			DescriptionHard:   testByte.DescriptionHard,
		}
		expectedBytesFrontend = append(expectedBytesFrontend, expectedByteFrontend)
	}

	// Defer statement for cleanup
	defer func() {
		_, err := testTiDB.DB.Exec("DELETE FROM bytes") // Replace 'bytes' with your actual table name
		if err != nil {
			t.Fatalf("Failed to cleanup test bytes: %v", err)
		}
	}()

	tests := []struct {
		name       string
		tidb       *ti.Database
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name:    "Test GetRecommendedBytes",
			tidb:    testTiDB,
			wantErr: false,
			wantResult: map[string]interface{}{
				"rec_bytes": expectedBytesFrontend,
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := GetRecommendedBytes(context.Background(), tt.tidb)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRecommendedBytes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("GetRecommendedBytes() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestGetByte(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// Insert a test byte into the database
	testByte := &models.Bytes{
		ID:                   1,
		Name:                 "Test Byte",
		DescriptionEasy:      "Test Description Easy",
		DescriptionMedium:    "Test Description Medium",
		DescriptionHard:      "Test Description Hard",
		OutlineContentEasy:   "Test Outline Easy",
		OutlineContentMedium: "Test Outline Medium",
		OutlineContentHard:   "Test Outline Hard",
		DevStepsEasy:         "Test Steps Easy",
		DevStepsMedium:       "Test Steps Medium",
		DevStepsHard:         "Test Steps Hard",
	}
	byteStmt, err := testByte.ToSQLNative()
	if err != nil {
		t.Fatalf("Failed to convert byte to SQL: %v", err)
	}
	for _, stmt := range byteStmt {
		_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test byte: %v", err)
		}
	}

	// Defer statement for cleanup
	defer func() {
		_, err := testTiDB.DB.Exec("DELETE FROM bytes") // Replace 'bytes' with your actual table name
		if err != nil {
			t.Fatalf("Failed to cleanup test byte: %v", err)
		}
	}()

	tests := []struct {
		name       string
		tidb       *ti.Database
		byteId     int64
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name:    "Test GetByte",
			tidb:    testTiDB,
			byteId:  1,
			wantErr: false,
			wantResult: map[string]interface{}{
				"rec_bytes": &models.BytesFrontend{
					ID:                   strconv.FormatInt(testByte.ID, 10),
					Name:                 testByte.Name,
					DescriptionEasy:      testByte.DescriptionEasy,
					DescriptionMedium:    testByte.DescriptionMedium,
					DescriptionHard:      testByte.DescriptionHard,
					OutlineContentEasy:   testByte.OutlineContentEasy,
					OutlineContentMedium: testByte.OutlineContentMedium,
					OutlineContentHard:   testByte.OutlineContentHard,
					DevStepsEasy:         testByte.DevStepsEasy,
					DevStepsMedium:       testByte.DevStepsMedium,
					DevStepsHard:         testByte.DevStepsHard,
				},
			},
		},
		// Additional test cases can be added here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := GetByte(context.Background(), tt.tidb, tt.byteId)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetByte() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("GetByte() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
