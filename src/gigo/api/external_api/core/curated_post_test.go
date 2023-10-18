package core

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/kisielk/sqlstruct"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"testing"
)

func TestAddPostToCurated(t *testing.T) {
	t.Helper()

	// Initialize test database and other dependencies
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}
	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Fatalf("Failed to create Snowflake node: %v", err)
	}

	adminUser := &models.User{
		UserName: "gigo",
	}

	postId := int64(1)
	proficiencyTypes := []models.ProficiencyType{models.ProficiencyType(1)}
	postLanguage := models.ProgrammingLanguage(1)

	// Call the function
	_, err = AddPostToCurated(context.Background(), testTiDB, testSnowflake, adminUser, postId, proficiencyTypes, postLanguage)
	if err != nil {
		t.Fatalf("Failed to add post to curated: %v", err)
	}

	// Query the created curated post from the database
	res, err := testTiDB.DB.Query("SELECT * FROM curated_post WHERE post_id = ?", postId)
	if err != nil {
		t.Fatalf("Failed to query created curated post: %v", err)
	}
	defer res.Close()

	// Check if the created curated post exists in the database
	ok := res.Next()
	if !ok {
		t.Fatal("Expected to find the created curated post in the database, but it was not found")
	}

	// Decode the row results into a CuratedPost model
	dbCuratedPost := models.CuratedPost{}
	err = sqlstruct.Scan(&dbCuratedPost, res)
	if err != nil {
		t.Fatalf("Failed to scan curated post row: %v", err)
	}

	// Assertions (you might need to add more based on your specific fields)
	assert.Equal(t, postId, dbCuratedPost.PostID, "Expected database PostID to match")
	// Continue with more assertions based on your CuratedPost model

	t.Log("TestAddPostToCurated succeeded")
}

func TestRemoveCuratedPost(t *testing.T) {
	t.Helper()

	// Initialize test database and other dependencies
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	adminUser := &models.User{
		UserName: "gigo",
	}

	curatedPostID := int64(1)

	// Call the function
	_, err = RemoveCuratedPost(context.Background(), testTiDB, adminUser, curatedPostID)
	if err != nil {
		t.Fatalf("Failed to remove curated post: %v", err)
	}

	// Query the database to make sure the curated post has been removed
	res, err := testTiDB.DB.Query("SELECT * FROM curated_post WHERE _id = ?", curatedPostID)
	if err != nil {
		t.Fatalf("Failed to query for removed curated post: %v", err)
	}
	defer res.Close()

	// Check if the curated post has been removed
	ok := res.Next()
	if ok {
		t.Fatal("Expected the curated post to be removed, but it still exists in the database")
	}

	t.Log("TestRemoveCuratedPost succeeded")
}

func TestGetCuratedPostsAdmin(t *testing.T) {
	// Initialize test database and other dependencies
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_test_db")
	if err != nil {
		t.Fatalf("Initialize test database failed: %v", err)
	}

	// Initialize Tracer for testing
	// Assuming you have an import: otel "go.opentelemetry.io/otel"
	ctx, _ := otel.Tracer("gigo-test").Start(context.Background(), "test")

	adminUser := &models.User{
		UserName: "gigo",
	}
	proficiencyFilter := models.ProficiencyType(1)
	languageFilter := models.ProgrammingLanguage(1)

	// Call the function
	result, err := GetCuratedPostsAdmin(ctx, testTiDB, adminUser, proficiencyFilter, languageFilter)
	if err != nil {
		t.Fatalf("Failed to get curated posts: %v", err)
	}

	// Assertions
	curatedPosts, ok := result["curated_posts"].([]*models.PostFrontend)
	if !ok {
		t.Fatalf("Result does not contain curated posts")
	}

	if len(curatedPosts) == 0 {
		t.Fatalf("Expected curated posts but found none")
	}

	t.Log("TestGetCuratedPostsAdmin succeeded")
}

func TestCurationAuth2(t *testing.T) {
	ctx := context.Background()

	adminUser := &models.User{UserName: "gigo"}
	nonAdminUser := &models.User{UserName: "not-gigo"}

	curateSecret := "correctSecret"
	wrongSecret := "wrongSecret"

	tests := []struct {
		name        string
		user        *models.User
		secret      string
		password    string
		wantAuth    bool
		wantMessage string
	}{
		{"Nil User", nil, curateSecret, "anyPassword", false, "Incorrect calling user"},
		{"Non-Admin User", nonAdminUser, curateSecret, "anyPassword", false, "Incorrect calling user"},
		{"Admin User, Incorrect Password", adminUser, curateSecret, wrongSecret, false, "Incorrect password"},
		{"Admin User, Correct Password", adminUser, curateSecret, curateSecret, true, "Access Granted"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CurationAuth(ctx, tt.user, tt.secret, tt.password)
			if (err != nil) != tt.wantAuth {
				t.Errorf("CurationAuth() error = %v, wantAuth %v", err, tt.wantAuth)
				return
			}

			// Using testify for better assertions. You can use standard assertions as well.
			assert.Equal(t, tt.wantAuth, result["auth"])
			assert.Equal(t, tt.wantMessage, result["message"])
		})
	}
}

func TestCurationAuth(t *testing.T) {
	ctx := context.Background()

	adminUser := &models.User{UserName: "gigo"}
	//nonAdminUser := &models.User{UserName: "not-gigo"}

	curateSecret := ""
	//wrongSecret := "wrongSecret"

	res, err := CurationAuth(ctx, adminUser, curateSecret, "")
	if err != nil {
		fmt.Println("message is :" + res["message"].(string) + fmt.Sprintf("\nerr is %v", err))
		t.Fatalf("CurationAuth() error = %v", err)
	}

	fmt.Println("success")
}
