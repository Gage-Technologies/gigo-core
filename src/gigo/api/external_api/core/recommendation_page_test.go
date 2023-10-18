package core

import (
	"context"
	"fmt"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core/query_models"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"testing"
	"time"
)

func TestTopRecommendation(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	id := int64(5)
	post, err := models.CreatePost(69420, "test", "content", "author", 42069, time.Now(),
		time.Now(), 69, 3, []int64{}, &id, 6969, 20, 40, 24, 27,
		[]models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil, nil,
		75827, 8, nil, false, false, nil)
	if err != nil {
		t.Error("\nTopRecommendation Failed")
		return
	}

	postTwo, err := models.CreatePost(42069, "test", "content", "author", 42069,
		time.Now(), time.Now(), 69, 3, []int64{}, &id, 6969, 20, 40, 24,
		27, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil,
		nil, 4698295, 43, nil, false, false, nil)
	if err != nil {
		t.Error("\nTopRecommendation Failed")
		return
	}

	postThree, err := models.CreatePost(6942069, "test", "content", "author", 42069,
		time.Now(), time.Now(), 69, 3, []int64{}, &id, 6969, 20, 40, 24,
		27, []models.ProgrammingLanguage{models.Go}, models.PublicVisibility, []int64{}, nil,
		nil, 27958, 12, nil, false, false, nil)
	if err != nil {
		t.Error("\nTopRecommendation Failed")
		return
	}

	stmt, err := post.ToSQLNative()
	if err != nil {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	for _, s := range stmt {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTopRecommendation failed\n    Error: ", err)
			return
		}
	}

	stmtTwo, err := postTwo.ToSQLNative()
	if err != nil {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	for _, s := range stmtTwo {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTopRecommendation failed\n    Error: ", err)
			return
		}
	}

	stmtThree, err := postThree.ToSQLNative()
	if err != nil {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	for _, s := range stmtThree {
		_, err = testTiDB.DB.Exec(s.Statement, s.Values...)
		if err != nil {
			t.Error("\nTopRecommendation failed\n    Error: ", err)
			return
		}
	}

	ids := []int64{42069, 69420, 6942069}

	authorIds := []int64{42069, 42069, 42069}

	postIds := []int64{69420, 42069, 6942069}

	tx, err := testTiDB.DB.BeginTx(context.TODO(), nil)
	if err != nil {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	for i := 0; i < len(ids); i++ {
		rp, err := models.CreateRecommendedPost(ids[i], authorIds[i], postIds[i], models.RecommendationType(i), ids[2-i], 69420, time.Now(), time.Now(), 0)
		if err != nil {
			t.Error("\nCreate Attempt Post Failed")
			return
		}

		statement := rp.ToSQLNative()

		stmts, err := tx.Prepare(statement.Statement)
		if err != nil {
			t.Error("\nCreate Attempt Post Failed\n    Error: ", err)
			return
		}

		_, err = stmts.Exec(statement.Values...)
		if err != nil {
			t.Error("\nCreate Attempt Post Failed\n    Error: ", err)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		t.Error("\nCreate Attempt Post Failed\n    Error: ", err)
		return
	}

	top, err := TopRecommendation(context.Background(), testTiDB, &models.User{ID: 42069})
	if err != nil {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	fmt.Println(top["top_project"].(*query_models.RecommendedPostMergeFrontend).ID)

	if top["top_project"].(*query_models.RecommendedPostMergeFrontend).ID != "69420" {
		t.Error("\nTopRecommendation failed\n    Error: ", err)
		return
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM recommended_post`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
	}()

	t.Log("\nTopRecommendation succeeded")
}

func TestRecommendByAttempt(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}
	awards := make([]int64, 0)

	attempts := []models.Attempt{
		{
			ID:          69,
			Description: "Test 1",
			Author:      "Test Author 1",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  4,
			Coffee:      5,
			PostID:      69,
			Closed:      false,
			Tier:        1,
		},
		{
			ID:          420,
			Description: "Test 1",
			Author:      "Test Author 2",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(520, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  3,
			Coffee:      12,
			PostID:      420,
			Closed:      true,
			Tier:        2,
		},
		{
			ID:          42069,
			Description: "Test 1",
			Author:      "Test Author 3",
			AuthorID:    42069,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(620, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			AuthorTier:  1,
			Coffee:      7,
			PostID:      6942069,
			Closed:      true,
			Tier:        3,
		},
	}

	tx, err := testTiDB.DB.Begin()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	for _, attempt := range attempts {
		stmt, err := attempt.ToSQLNative()
		// will break here if you add awards because stmt index hard coded to 0 because im being lazy
		for _, statement := range stmt {
			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
				return
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
		return
	}

	var topReply *int64

	posts := []models.Post{
		{
			ID:          69,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          420,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
		{
			ID:          6942069,
			Title:       "Test 1",
			Description: "Test 1",
			Author:      "giga chad",
			AuthorID:    420,
			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			RepoID:      6969,
			Tier:        4,
			Awards:      awards,
			TopReply:    topReply,
			Coffee:      12,
			Tags:        []int64{},
			PostType:    1,
			Views:       69420,
			Completions: 69,
			Attempts:    42069,
		},
	}

	for i := 0; i < len(posts); i++ {
		p, err := models.CreatePost(posts[i].ID, posts[i].Title, posts[i].Description, posts[i].Author,
			posts[i].AuthorID, posts[i].CreatedAt, posts[i].UpdatedAt, posts[i].RepoID,
			posts[i].Tier, posts[i].Awards, posts[i].TopReply, posts[i].Coffee, posts[i].PostType, 69,
			posts[i].Completions, posts[i].Attempts, posts[i].Languages, posts[i].Visibility, posts[i].Tags,
			nil, nil, 35930, 12, nil, false, false, nil)
		if err != nil {
			t.Error("\nRecommendByAttempt failed")
			return
		}

		statements, err := p.ToSQLNative()

		for _, statement := range statements {
			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
			if err != nil {
				t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
				return
			}
		}
	}

	recommendedPosts := []models.RecommendedPost{
		{
			ID:            69,
			UserID:        420,
			PostID:        posts[0].ID,
			Type:          0,
			ReferenceID:   69420,
			Score:         12,
			CreatedAt:     time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			ExpiresAt:     time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			ReferenceTier: 0,
		},
		{
			ID:            69,
			UserID:        420,
			PostID:        posts[1].ID,
			Type:          0,
			ReferenceID:   69420,
			Score:         12,
			CreatedAt:     time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			ExpiresAt:     time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			ReferenceTier: 0,
		},
		{
			ID:            69,
			UserID:        420,
			PostID:        posts[2].ID,
			Type:          0,
			ReferenceID:   69420,
			Score:         12,
			CreatedAt:     time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			ExpiresAt:     time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			ReferenceTier: 0,
		},
	}

	for i := 0; i < len(recommendedPosts); i++ {
		rp, err := models.CreateRecommendedPost(recommendedPosts[i].ID, recommendedPosts[i].UserID, recommendedPosts[i].PostID, recommendedPosts[i].Type, recommendedPosts[i].ReferenceID,
			recommendedPosts[i].Score, recommendedPosts[i].CreatedAt, recommendedPosts[i].ExpiresAt, recommendedPosts[i].ReferenceTier)
		if err != nil {
			t.Error("\nRecommendByAttempt failed")
			return
		}

		statement := rp.ToSQLNative()

		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
			return
		}

	}

	res, err := RecommendByAttempt(context.Background(), testTiDB, &models.User{ID: 42069})
	if err != nil {
		t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
		return
	}

	if res == nil {
		t.Errorf("\nRecommendByAttempt failed\n    Error: %v", err)
		return
	}

	t.Log("\nnRecommendByAttempt succeeded")
}

//func TestHarderRecommendation(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}

//	awards := make([]int64, 0)
//
//	attempts := []models.Attempt{
//		{
//			ID:          69,
//			Description: "Test 1",
//			Author:      "Test Author 1",
//			AuthorID:    69,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			AuthorTier:  4,
//			Awards:      awards,
//			Coffee:      5,
//			PostID:      420,
//			Completed:   true,
//			Tier:        3,
//		},
//		{
//			ID:          420,
//			Description: "Test 2",
//			Author:      "Test Author 2",
//			AuthorID:    69,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(520, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			AuthorTier:  3,
//			Awards:      awards,
//			Coffee:      12,
//			PostID:      420,
//			Completed:   true,
//			Tier:        2,
//		},
//		{
//			ID:          42069,
//			Description: "Test 3",
//			Author:      "Test Author 3",
//			AuthorID:    69,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(620, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			AuthorTier:  1,
//			Awards:      awards,
//			Coffee:      7,
//			PostID:      420,
//			Completed:   true,
//			Tier:        1,
//		},
//	}
//
//	tx, err := testTiDB.DB.Begin()
//	if err != nil {
//		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
//		return
//	}
//
//	for _, attempt := range attempts {
//		stmt, err := attempt.ToSQLNative()
//		// will break here if you add awards because stmt index hard coded to 0 because im being lazy
//		for _, statement := range stmt {
//			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//			if err != nil {
//				t.Errorf("\nHarderRecommendation failed\n    Error: %v", err)
//				return
//			}
//		}
//	}
//
//	err = tx.Commit()
//	if err != nil {
//		t.Errorf("\nGetActiveProjects failed\n    Error: %v\n", err)
//		return
//	}
//
//	var topReply *int64
//
//	posts := []models.Post{
//		{
//			ID:          69,
//			Title:       "Test 1",
//			Description: "Test 1",
//			Author:      "giga chad",
//			AuthorID:    420,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			Tier:        2,
//			Awards:      awards,
//			TopReply:    topReply,
//			Coffee:      12,
//			Tags:        []int64{},
//			PostType:    1,
//			Views:       69420,
//			Completions: 69,
//			Attempts:    42069,
//		},
//		{
//			ID:          420,
//			Title:       "Test 1",
//			Description: "Test 1",
//			Author:      "giga chad",
//			AuthorID:    420,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			Tier:        3,
//			Awards:      awards,
//			TopReply:    topReply,
//			Coffee:      12,
//			Tags:        []int64{},
//			PostType:    1,
//			Views:       69420,
//			Completions: 69,
//			Attempts:    42069,
//		},
//		{
//			ID:          6942069,
//			Title:       "Test 1",
//			Description: "Test 1",
//			Author:      "giga chad",
//			AuthorID:    420,
//			CreatedAt:   time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
//			UpdatedAt:   time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
//			RepoID:      6969,
//			Tier:        4,
//			Awards:      awards,
//			TopReply:    topReply,
//			Coffee:      12,
//			Tags:        []int64{},
//			PostType:    1,
//			Views:       69420,
//			Completions: 69,
//			Attempts:    42069,
//		},
//	}
//
//	for i := 0; i < len(posts); i++ {
//		p, err := models.CreatePost(posts[i].ID, posts[i].Title, posts[i].Description, posts[i].Author,
//			posts[i].AuthorID, posts[i].CreatedAt, posts[i].UpdatedAt, posts[i].RepoID,
//			posts[i].Tier, posts[i].Awards, posts[i].TopReply, posts[i].Coffee, posts[i].PostType, 69,
//			posts[i].Completions, posts[i].Attempts, posts[i].Languages, posts[i].Visibility, posts[i].Tags,
//			nil, nil, 89574, 12, nil, false, false)
//		if err != nil {
//			t.Error("\nHarderRecommendation failed")
//			return
//		}
//
//		statements, err := p.ToSQLNative()
//
//		for _, statement := range statements {
//			_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//			if err != nil {
//				t.Errorf("\nHarderRecommendation failed\n    Error: %v", err)
//				return
//			}
//		}
//	}
//
//	recommendedPosts := []models.RecommendedPost{
//		{
//			ID:         69,
//			AuthorID:   420,
//			PostID:     posts[0].ID,
//			Similarity: 35,
//		},
//		{
//			ID:         69,
//			AuthorID:   420,
//			PostID:     posts[1].ID,
//			Similarity: 55,
//		},
//		{
//			ID:         69,
//			AuthorID:   420,
//			PostID:     posts[2].ID,
//			Similarity: 95,
//		},
//	}
//
//	for i := 0; i < len(recommendedPosts); i++ {
//		rp, err := models.CreateRecommendedPost(recommendedPosts[i].ID, recommendedPosts[i].AuthorID, recommendedPosts[i].PostID, recommendedPosts[i].Similarity)
//		if err != nil {
//			t.Error("\nHarderRecommendation failed")
//			return
//		}
//
//		statement := rp.ToSQLNative()
//
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("\nHarderRecommendation failed\n    Error: %v", err)
//			return
//		}
//
//	}
//
//	res, err := HarderRecommendation(testTiDB, &models.User{ID: 42069})
//	if err != nil {
//		t.Errorf("\nHarderRecommendation failed\n    Error: %v", err)
//		return
//	}
//
//	if res == nil {
//		t.Errorf("\nHarderRecommendation failed\n    Error: %v", err)
//		return
//	}
//
//	t.Log("\nnHarderRecommendation succeeded")
//}
