package core

import (
	"context"
	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	_ "github.com/gage-technologies/gigo-lib/search"
	"reflect"
	"testing"
)

//func TestSearchPosts(t *testing.T) {
//	cfg := config.MeiliConfig{
//		Host:  "http://gigo-dev-meili:7700",
//		Token: "gigo-dev",
//		Indices: map[string]config.MeiliIndexConfig{
//			"posts": {
//				Name:                 "posts",
//				PrimaryKey:           "_id",
//				SearchableAttributes: []string{"title", "description", "author"},
//				FilterableAttributes: []string{
//					"languages",
//					"attempts",
//					"completions",
//					"coffee",
//					"views",
//					"tags",
//					"post_type",
//					"visibility",
//					"created_at",
//					"updated_at",
//					"published",
//					"tier",
//					"author_id",
//				},
//				SortableAttributes: []string{
//					"attempts",
//					"completions",
//					"coffee",
//					"views",
//					"created_at",
//					"updated_at",
//					"tier",
//				},
//			},
//		},
//	}
//
//	meili, err := search.CreateMeiliSearchEngine(cfg)
//	if err != nil {
//		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
//	}
//
//	defer meili.DeleteDocuments("posts", 69420)
//
//	post := models.Post{
//		ID:          69420,
//		Title:       "test title 1",
//		Description: "test description 2",
//		Author:      "test author 1",
//		AuthorID:    69,
//		CreatedAt:   time.Now().Add(-time.Hour * 24 * 32),
//		UpdatedAt:   time.Now().Add(-time.Hour * 24 * 7),
//		RepoID:      42069,
//		Tier:        models.Tier6,
//		Awards:      []int64{1, 2, 3, 4, 5, 6},
//		Coffee:      17283,
//		Tags:        []int64{7, 8, 9},
//		PostType:    models.CompetitiveChallenge,
//		Views:       4200,
//		Languages:   []models.ProgrammingLanguage{models.Go, models.JavaScript},
//		Attempts:    420,
//		Completions: 69,
//		Published:   true,
//		Visibility:  models.PublicVisibility,
//	}
//
//	err = meili.AddDocuments("posts", post)
//	if err != nil {
//		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
//	}
//
//	res, err := SearchPosts(
//		meili,
//		nil,
//		"test",
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		nil,
//		0,
//		10,
//	)
//	if err != nil {
//		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
//	}
//
//	posts, ok := res["challenges"]
//	if !ok {
//		t.Fatalf("\n%s failed\n    Error: incorrect return %v", t.Name(), res)
//	}
//
//	if len(posts.([]*models.PostFrontend)) != 1 {
//		t.Fatalf("\n%s failed\n    Error: incorrect return %v", t.Name(), posts)
//	}
//
//	if posts.([]*models.PostFrontend)[0].ID != "69420" {
//		t.Fatalf("\n%s failed\n    Error: incorrect return %v", t.Name(), posts)
//	}
//
//	t.Logf("\n%s succeeded", t.Name())
//}

func TestConditionallyAddRangeFilter(t *testing.T) {
	tests := []struct {
		name       string
		conditions []search.FilterCondition
		attribute  string
		lower      interface{}
		upper      interface{}
		expected   []search.FilterCondition
	}{
		{
			name:      "No bounds",
			attribute: "test",
			lower:     nil,
			upper:     nil,
			expected:  []search.FilterCondition{},
		},
		{
			name:      "Lower bound only",
			attribute: "test",
			lower:     5,
			upper:     nil,
			expected: []search.FilterCondition{
				{
					And: true,
					Filters: []search.Filter{
						{
							Attribute: "test",
							Operator:  search.OperatorGreaterThanOrEquals,
							Value:     5,
						},
					},
				},
			},
		},
		{
			name:      "Upper bound only",
			attribute: "test",
			lower:     nil,
			upper:     10,
			expected: []search.FilterCondition{
				{
					And: true,
					Filters: []search.Filter{
						{
							Attribute: "test",
							Operator:  search.OperatorLessThanOrEquals,
							Value:     10,
						},
					},
				},
			},
		},
		{
			name:      "Both bounds",
			attribute: "test",
			lower:     5,
			upper:     10,
			expected: []search.FilterCondition{
				{
					And: true,
					Filters: []search.Filter{
						{
							Attribute: "test",
							Operator:  search.OperatorGreaterThanOrEquals,
							Value:     5,
						},
						{
							Attribute: "test",
							Operator:  search.OperatorLessThanOrEquals,
							Value:     10,
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := conditionallyAddRangeFilter(tt.conditions, tt.attribute, tt.lower, tt.upper)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected: %v, Got: %v", tt.expected, result)
			}
		})
	}
}

// You need to define MockDatabase and MockMeiliSearchEngine with appropriate interfaces and mocking methods.

func TestSearchUsers(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// todo: set correct meili indices for test
	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"discussion": {
				Name:                 "discussion",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	tests := []struct {
		name       string
		query      string
		skip       int
		limit      int
		db         *ti.Database
		meili      *search.MeiliSearchEngine
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name: "Test search users",
			// Fill in your test case details here
			query:   "Sample Query",
			skip:    0,
			limit:   10,
			db:      testTiDB,
			meili:   meili,
			wantErr: false,
			wantResult: map[string]interface{}{
				"users": []interface{}{}, // Expected user list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := SearchUsers(context.Background(), tt.db, tt.meili, tt.query, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchUsers() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("SearchUsers() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestSearchTags(t *testing.T) {
	// todo: set correct meili indices for test
	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"tags": {
				Name:                 "tags",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"name", "description"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	tests := []struct {
		name       string
		query      string
		skip       int
		limit      int
		mockMeili  *search.MeiliSearchEngine
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name: "Test search tags",
			// Fill in your test case details here
			query:     "Sample Query",
			skip:      0,
			limit:     10,
			mockMeili: meili,
			wantErr:   false,
			wantResult: map[string]interface{}{
				"tags": []*models.TagFrontend{}, // Expected tag list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := SearchTags(context.Background(), tt.mockMeili, tt.query, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchTags() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("SearchTags() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestSearchDiscussions(t *testing.T) {
	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"discussion": {
				Name:                 "discussion",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "body"},
				FilterableAttributes: []string{"post_id"},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	postId := int64(1)

	tests := []struct {
		name       string
		query      string
		skip       int
		limit      int
		postId     *int64
		mockMeili  *search.MeiliSearchEngine
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name: "Test search discussions",
			// Fill in your test case details here
			query:     "Sample Query",
			skip:      0,
			limit:     10,
			postId:    &postId,
			mockMeili: meili,
			wantErr:   false,
			wantResult: map[string]interface{}{
				"discussions": []*models.DiscussionFrontend{}, // Expected discussion list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := SearchDiscussions(context.Background(), tt.mockMeili, tt.query, tt.skip, tt.limit, tt.postId)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchDiscussions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("SearchDiscussions() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestSearchComments(t *testing.T) {
	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"comment": {
				Name:                 "comment",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"text"},
				FilterableAttributes: []string{"discussion_id"},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	discussionId := int64(1)

	tests := []struct {
		name         string
		query        string
		skip         int
		limit        int
		discussionId *int64
		mockMeili    *search.MeiliSearchEngine
		wantErr      bool
		wantResult   map[string]interface{}
	}{
		{
			name: "Test search comments",
			// Fill in your test case details here
			query:        "Sample Query",
			skip:         0,
			limit:        10,
			discussionId: &discussionId,
			mockMeili:    meili,
			wantErr:      false,
			wantResult: map[string]interface{}{
				"comment": []*models.CommentFrontend{}, // Expected comment list
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := SearchComments(context.Background(), tt.mockMeili, tt.query, tt.skip, tt.limit, tt.discussionId)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchComments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("SearchComments() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}

func TestSearchWorkspaceConfigs(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"workspace_configs": {
				Name:                 "workspace_configs",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"title", "description", "languages", "tags"},
				FilterableAttributes: []string{"languages", "tags"},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	tests := []struct {
		name       string
		query      string
		languages  []models.ProgrammingLanguage
		tags       []int64
		skip       int
		limit      int
		db         *ti.Database
		mockMeili  *search.MeiliSearchEngine
		wantErr    bool
		wantResult map[string]interface{}
	}{
		{
			name: "Test search workspace configs",
			// Fill in your test case details here
			query: "Sample Query",
			languages: []models.ProgrammingLanguage{
				models.Java,
				models.Python,
			},
			tags:      []int64{1, 2, 3},
			skip:      0,
			limit:     10,
			db:        testTiDB,
			mockMeili: meili,
			wantErr:   false,
			wantResult: map[string]interface{}{
				"workspace_configs": []*models.WorkspaceConfigFrontend{}, // Expected workspace config list
				"tags":              map[string]*models.TagFrontend{},    // Expected tag map
			},
		},
		// Add more test cases if needed
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, err := SearchWorkspaceConfigs(context.Background(), tt.db, tt.mockMeili, tt.query, tt.languages, tt.tags, tt.skip, tt.limit)
			if (err != nil) != tt.wantErr {
				t.Errorf("SearchWorkspaceConfigs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotResult, tt.wantResult) {
				t.Errorf("SearchWorkspaceConfigs() gotResult = %v, want %v", gotResult, tt.wantResult)
			}
		})
	}
}
