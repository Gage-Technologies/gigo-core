package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gage-technologies/GIGO/src/gigo/api/external_api/core/query_models"
	"github.com/gage-technologies/gigo-lib/config"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/search"
	"github.com/gage-technologies/gigo-lib/utils"
)

func TestGetDiscussions(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	awards := make([]int64, 0)

	tags := []int64{1, 2}

	discussions := []models.Discussion{
		{
			ID:         69,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   0,
			Leads:      true,
		},
		{
			ID:         69,
			Body:       "Test 2",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   1,
			Leads:      true,
		},
		{
			ID:         6969,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			UpdatedAt:  time.Date(420, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Awards:     awards,
			Coffee:     5,
			PostId:     420,
			Title:      "title",
			Tags:       tags,
			Revision:   0,
			Leads:      true,
		},
	}

	for i := 0; i < len(discussions); i++ {
		discussion, err := models.CreateDiscussion(discussions[i].ID, discussions[i].Body, discussions[i].Author, discussions[i].AuthorID,
			discussions[i].CreatedAt, discussions[i].UpdatedAt, discussions[i].AuthorTier, discussions[i].Awards, discussions[i].Coffee,
			discussions[i].PostId, discussions[i].Title, discussions[i].Tags, discussions[i].Leads, discussions[i].Revision, 0)
		if err != nil {
			t.Errorf("\nTestGetDiscussions failed\n    Error: %v\n", err)
			return
		}

		statement := discussion.ToSQLNative()

		for _, stmt := range statement {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetDiscussions failed\n    Error: %v", err)
				return
			}
		}
	}

	upVotes := []models.UpVote{
		{
			ID:             420,
			DiscussionType: 0,
			DiscussionId:   69,
			UserId:         69,
		},
		{
			ID:             500,
			DiscussionType: 0,
			DiscussionId:   6969,
			UserId:         69,
		},
		{
			ID:             1000,
			DiscussionType: 0,
			DiscussionId:   420,
			UserId:         69,
		},
	}

	for i := 0; i < len(upVotes); i++ {
		vote := models.CreateUpVote(upVotes[i].ID, upVotes[i].DiscussionType, upVotes[i].DiscussionId, upVotes[i].UserId)
		if err != nil {
			t.Errorf("\nTestGetDiscussions failed\n    Error: %v\n", err)
			return
		}

		statement := vote.ToSQLNative()

		for _, stmt := range statement {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetDiscussions failed\n    Error: %v", err)
				return
			}
		}
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM discussion`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM users where _id =?`, user.ID)
		if err != nil {
			t.Logf("Failed to delete sample user: %v", err)
		}
	}()

	res, err := GetDiscussions(context.Background(), testTiDB, user, 420, 0, 10)
	if err != nil {
		t.Errorf("\nTestGetDiscussions failed\n    Error: %v", err)
		return
	}

	b1, _ := json.Marshal(res["discussions"])

	outputHash, err := utils.HashData(b1)
	if err != nil {
		t.Error("TestGetDiscussions home failed")
		return
	}

	if res["discussions"] == nil {
		t.Errorf("\nTestGetDiscussions failed\n")
		return
	}

	if outputHash != "7267eee0ad46cd528e411131643958de26e7b623c616a3cd47d9c61134c3b3dd" {
		t.Errorf("\nTestGetDiscussions failed\n")
		return
	}

	t.Log("\nTestGetDiscussions succeeded")
}

func TestGetDiscussionComments(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	awards := make([]int64, 0)

	comments := []models.Comment{
		{
			ID:           69,
			Body:         "Test 1",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 420,
			Leads:        true,
			Revision:     0,
		},
		{
			ID:           69,
			Body:         "Test 2",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 420,
			Leads:        true,
			Revision:     1,
		},
		{
			ID:           420,
			Body:         "Test 1",
			Author:       "Test Author 1",
			AuthorID:     69,
			CreatedAt:    time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:   1,
			Awards:       awards,
			Coffee:       5,
			DiscussionId: 420,
			Leads:        true,
			Revision:     0,
		},
	}

	for i := 0; i < len(comments); i++ {
		comment, err := models.CreateComment(comments[i].ID, comments[i].Body, comments[i].Author, comments[i].AuthorID, comments[i].CreatedAt, comments[i].AuthorTier,
			comments[i].Awards, comments[i].Coffee, comments[i].DiscussionId, comments[i].Leads, comments[i].Revision, 1)
		if err != nil {
			t.Errorf("\nTestGetDiscussionComments failed\n    Error: %v\n", err)
			return
		}

		statement := comment.ToSQLNative()

		for _, stmt := range statement {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetDiscussionComments failed\n    Error: %v", err)
				return
			}
		}
	}

	idArray := []int64{420, 69}

	res, err := GetDiscussionComments(context.Background(), testTiDB, user, idArray, 0, 10)
	if err != nil {
		t.Errorf("\nTestGetDiscussionComments failed\n    Error: %v", err)
		return
	}

	// Add cleanup for the attempt
	defer func() {
		_, err = testTiDB.DB.Exec(`DELETE FROM comment`)
		if err != nil {
			t.Logf("Failed to delete sample attempt: %v", err)
		}
		_, err = testTiDB.DB.Exec(`DELETE FROM users where _id =?`, user.ID)
		if err != nil {
			t.Logf("Failed to delete sample user: %v", err)
		}
	}()

	b1, _ := json.Marshal(res["comments"])

	b2, _ := json.Marshal(res["lead_ids"])

	commentHash, err := utils.HashData(b1)
	if err != nil {
		t.Error("TestGetDiscussionComments home failed")
		return
	}

	leadHash, err := utils.HashData(b2)
	if err != nil {
		t.Error("TestGetDiscussionComments home failed")
		return
	}

	if res["comments"] == nil || res["lead_ids"] == nil {
		t.Errorf("\nTestGetDiscussionComments failed\n")
		return
	}

	if commentHash != "43bc198c71ae70e6d8e835bb9df1f9b2dc919932b3b67b8c86d925327d536cf0" {
		t.Errorf("\nTestGetDiscussionComments failed\n")
		return
	}

	if leadHash != "d5aa9968c79442c0c4df9fb34d90fc0a6f06172ea49984b819e14d0a1dfefed3" {
		t.Errorf("\nTestGetDiscussionComments failed\n")
		return
	}

	t.Log("\nTestGetDiscussionComments succeeded")
}

func TestGetCommentThreads(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	threads := []models.ThreadComment{
		{
			ID:         69,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Coffee:     5,
			CommentId:  420,
			Leads:      true,
			Revision:   0,
		},
		{
			ID:         69,
			Body:       "Test 2",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Coffee:     5,
			CommentId:  420,
			Leads:      true,
			Revision:   1,
		},
		{
			ID:         420,
			Body:       "Test 1",
			Author:     "Test Author 1",
			AuthorID:   69,
			CreatedAt:  time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier: 1,
			Coffee:     5,
			CommentId:  420,
			Leads:      false,
			Revision:   0,
		},
	}

	for i := 0; i < len(threads); i++ {
		commentThreads, err := models.CreateThreadComment(threads[i].ID, threads[i].Body, threads[i].Author, threads[i].AuthorID, threads[i].CreatedAt, threads[i].AuthorTier,
			threads[i].Coffee, threads[i].CommentId, threads[i].Leads, threads[i].Revision, 2)
		if err != nil {
			t.Errorf("\nTestGetCommentThreads failed\n    Error: %v\n", err)
			return
		}

		statement := commentThreads.ToSQLNative()

		for _, stmt := range statement {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetCommentThreads failed\n    Error: %v", err)
				return
			}
		}
	}

	idArray := []int64{420, 69}

	res, err := GetCommentThreads(context.Background(), testTiDB, user, idArray, 0, 10)
	if err != nil {
		t.Errorf("\nTestGetCommentThreads failed\n    Error: %v", err)
		return
	}

	b1, _ := json.Marshal(res["threads"])

	b2, _ := json.Marshal(res["lead_ids"])

	threadHash, err := utils.HashData(b1)
	if err != nil {
		t.Error("TestGetCommentThreads home failed")
		return
	}

	leadHash, err := utils.HashData(b2)
	if err != nil {
		t.Error("TestGetCommentThreads home failed")
		return
	}

	if res["threads"] == nil {
		t.Errorf("\nTestGetCommentThreads failed\n")
		return
	}

	if threadHash != "af6938b570df004b61ad971f4587f1cccbb45e7ad1848bf529209b2d6fe8cec0" {
		t.Errorf("\nTestGetCommentThreads failed\n")
		return
	}

	if leadHash != "0a1085bc35d7bd276dd038238484a02fbc877977a375fccbf44ee282b125713c" {
		t.Errorf("\nTestGetCommentThreads failed\n")
		return
	}

	t.Log("\nTestGetCommentThreads succeeded")
}

func TestGetGetThreadReply(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	replys := []models.ThreadReply{
		{
			ID:              69,
			Body:            "Test 1",
			Author:          "Test Author 1",
			AuthorID:        69,
			CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
			AuthorTier:      1,
			Coffee:          5,
			ThreadCommentId: 420,
		},
		// {
		//	ID:              69,
		//	Body:            "Test 1",
		//	Author:          "Test Author 1",
		//	AuthorID:        69,
		//	CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
		//	AuthorTier:      1,
		//	Coffee:          5,
		//	ThreadCommentId: 420,
		// },
		// {
		//	ID:              69,
		//	Body:            "Test 1",
		//	Author:          "Test Author 1",
		//	AuthorID:        69,
		//	CreatedAt:       time.Date(69, 6, 9, 4, 2, 0, 6, time.UTC),
		//	AuthorTier:      1,
		//	Coffee:          5,
		//	ThreadCommentId: 420,
		// },
	}

	for i := 0; i < len(replys); i++ {
		threadReply, err := models.CreateThreadReply(replys[i].ID, replys[i].Body, replys[i].Author, replys[i].AuthorID, replys[i].CreatedAt, replys[i].AuthorTier,
			replys[i].Coffee, replys[i].ThreadCommentId, 0, 3)
		if err != nil {
			t.Errorf("\nTestGetGetThreadReply failed\n    Error: %v\n", err)
			return
		}

		statement := threadReply.ToSQLNative()

		for _, stmt := range statement {
			_, err = testTiDB.DB.Exec(stmt.Statement, stmt.Values...)
			if err != nil {
				t.Errorf("\nTestGetGetThreadReply failed\n    Error: %v", err)
				return
			}
		}
	}

	idArray := []int64{420, 69}

	res, err := GetThreadReply(context.Background(), testTiDB, user, idArray, 0, 10)
	if err != nil {
		t.Errorf("\nTestGetGetThreadReply failed\n    Error: %v", err)
		return
	}

	// b1, _ := json.Marshal(res["thread_reply"])
	//
	// replyHash, err := utils.HashData(b1)
	// if err != nil {
	//	t.Error("TestGetGetThreadReply home failed")
	//	return
	// }

	if res["thread_reply"] == nil {
		t.Errorf("\nTestGetGetThreadReply failed\n")
		return
	}

	fmt.Println(res["thread_reply"].([]*query_models.ThreadReplyBackgroundFrontend)[0])

	// if replyHash != "e75d0115a9ab1e16d55ea70d3a27a98099f608a24c369bbaaaeee948354b9970" {
	//	t.Errorf("\nTestGetGetThreadReply failed\n")
	//	return
	// }

	t.Log("\nTestGetGetThreadReply succeeded")
}

func TestCreateDiscussion(t *testing.T) {
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

	defer func() {
		testTiDB.DB.Exec("drop table discussion")
		testTiDB.DB.Exec("drop table users")
		testTiDB.DB.Exec("drop table discussion_awards")
		testTiDB.DB.Exec("drop table discussion_tags")
		meili.DeleteDocuments("discussion", 69)
	}()

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	discussion, err := CreateDiscussion(context.Background(), testTiDB, meili, user, testSnowflake, 69, "test-title", "test123", nil)
	if err != nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	if discussion["discussion"].(*models.DiscussionFrontend) == nil {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	if discussion["message"].(string) != "Discussion has been posted" {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	if discussion["discussion"].(*models.DiscussionFrontend).Author != "test" {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	if discussion["discussion"].(*models.DiscussionFrontend).PostId != "69" {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	if discussion["discussion"].(*models.DiscussionFrontend).Body != "test123" {
		t.Errorf("\nTestCreateDiscussion failed\n    Error: %v\n", err)
		return
	}

	t.Log("\nTestCreateDiscussion succeeded")
}

func TestCreateComment(t *testing.T) {
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
			"comment": {
				Name:                 "comment",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	defer func() {
		meili.DeleteDocuments("comment", 69)
	}()

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	comment, err := CreateComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "test123")
	if err != nil {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	if comment["comment"].(*models.CommentFrontend) == nil {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	if comment["message"].(string) != "Comment has been posted" {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	fmt.Println(fmt.Sprintf("%v", comment["comment"].(*models.CommentFrontend)))

	if comment["comment"].(*models.CommentFrontend).Author != "test" {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	if comment["comment"].(*models.CommentFrontend).DiscussionId != "69" {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	if comment["comment"].(*models.CommentFrontend).Body != "test123" {
		t.Errorf("\nTestCreateComment failed\n    Error: %v\n", err)
		return
	}

	t.Log("\nTestCreateComment succeeded")
}

func TestCreateThreadComment(t *testing.T) {
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
			"thread_comment": {
				Name:                 "thread_comment",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	defer func() {
		meili.DeleteDocuments("thread_comment", 69)
	}()

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	threadComment, err := CreateThreadComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "test123")
	if err != nil {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	if threadComment["thread_comment"].(*models.ThreadCommentFrontend) == nil {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	if threadComment["message"].(string) != "Comment has been posted" {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	fmt.Println(fmt.Sprintf("%v", threadComment["thread_comment"].(*models.ThreadCommentFrontend)))

	if threadComment["thread_comment"].(*models.ThreadCommentFrontend).Author != "test" {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	if threadComment["thread_comment"].(*models.ThreadCommentFrontend).CommentId != "69" {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	if threadComment["thread_comment"].(*models.ThreadCommentFrontend).Body != "test123" {
		t.Errorf("\nTestCreateThreadComment failed\n    Error: %v\n", err)
		return
	}

	t.Log("\nTestCreateThreadComment succeeded")
}

func TestCreateThreadReply(t *testing.T) {
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
			"thread_comment": {
				Name:                 "thread_comment",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	defer func() {
		meili.DeleteDocuments("thread_comment", 69)
	}()

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	thread, err := CreateThreadComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "test123")
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	id, err := strconv.ParseInt(thread["thread_comment"].(*models.ThreadCommentFrontend).ID, 10, 64)
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	threadReply, err := CreateThreadReply(context.Background(), testTiDB, user, testSnowflake, id, "test123")
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	if threadReply["thread_reply"].(*models.ThreadReplyFrontend) == nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	if threadReply["message"].(string) != "Reply has been posted" {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	fmt.Println(fmt.Sprintf("%v", threadReply["thread_reply"].(*models.ThreadReplyFrontend)))

	if threadReply["thread_reply"].(*models.ThreadReplyFrontend).Author != "test" {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	if threadReply["thread_reply"].(*models.ThreadReplyFrontend).ThreadCommentId != fmt.Sprintf("%v", id) {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	if threadReply["thread_reply"].(*models.ThreadReplyFrontend).Body != "test123" {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	res, err := testTiDB.DB.Query("select * from thread_comment where _id = ? and leads = true", id)
	if err != nil {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	if !res.Next() {
		t.Errorf("\nTestCreateThreadReply failed\n    Error: %v\n", err)
		return
	}

	t.Log("\nTestCreateThreadReply succeeded")
}

func TestEditDiscussions(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"discussion": {
				Name:                 "discussion",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	defer func() {
		meili.DeleteDocuments("discussion", 69)
	}()

	discussion, err := CreateDiscussion(context.Background(), testTiDB, meili, user, testSnowflake, 69, "title", "body", nil)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	comment, err := CreateComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "body")
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	thread, err := CreateThreadComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "body")
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	// reply, err := CreateThreadReply(testTiDB, user, testSnowflake, 69, "body")
	// if err!= nil {
	//	t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
	//	return
	// }

	discId, err := strconv.ParseInt(discussion["discussion"].(*models.DiscussionFrontend).ID, 10, 64)
	// newTitle := "edited title"

	editDiscussion, err := EditDiscussions(context.Background(), testTiDB, user, meili, testSnowflake, "discussion", discId, nil, "edited body", nil)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	// if editDiscussion["new_discussion"].(*models.DiscussionFrontend).Title != newTitle {
	//	t.Errorf("\nTestEditDiscussions failed\n    Error: title not updated")
	// }

	if editDiscussion["new_discussion"].(*models.DiscussionFrontend).Body != "edited body" {
		t.Errorf("\nTestEditDiscussions failed\n    Error: body not updated")
	}

	fmt.Println(editDiscussion["new_discussion"].(*models.DiscussionFrontend))

	commId, err := strconv.ParseInt(comment["comment"].(*models.CommentFrontend).ID, 10, 64)

	editComment, err := EditDiscussions(context.Background(), testTiDB, user, meili, testSnowflake, "comment", commId, nil, "edited body", nil)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	if editComment["new_comment"].(*models.CommentFrontend).Body != "edited body" {
		t.Errorf("\nTestEditDiscussions failed\n    Error: body not updated")
	}

	thrId, err := strconv.ParseInt(thread["thread_comment"].(*models.ThreadCommentFrontend).ID, 10, 64)

	editThread, err := EditDiscussions(context.Background(), testTiDB, user, meili, testSnowflake, "thread_comment", thrId, nil, "edited body", nil)
	if err != nil {
		t.Errorf("\nTestEditDiscussions failed\n    Error: %v\n", err)
		return
	}

	if editThread["new_thread_comment"].(*models.ThreadCommentFrontend).Body != "edited body" {
		t.Errorf("\nTestEditDiscussions failed\n    Error: body not updated")
	}
}

func TestAddDiscussionCoffee(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"discussion": {
				Name:                 "discussion",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	defer func() {
		meili.DeleteDocuments("discussion", 69)
	}()

	discussion, err := CreateDiscussion(context.Background(), testTiDB, meili, user, testSnowflake, 69, "title", "body", nil)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	comment, err := CreateComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "body")
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	discId, err := strconv.ParseInt(discussion["discussion"].(*models.DiscussionFrontend).ID, 10, 64)

	resOne, err := AddDiscussionCoffee(context.Background(), testTiDB, user, testSnowflake, discId, "discussion")
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if resOne["message"].(string) != "Coffee added to discussion" {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect message returned")
	}

	checkOne, err := testTiDB.DB.Query("SELECT * FROM discussion WHERE _id = ?", discId)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if !checkOne.Next() {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect result set")
		return
	} else {
		resOneCheck, err := models.DiscussionFromSQLNative(testTiDB, checkOne)
		if err != nil {
			t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
			return
		}
		if strconv.FormatUint(resOneCheck.Coffee-1, 10) != discussion["discussion"].(*models.DiscussionFrontend).Coffee {
			t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect coffee returned")
		}
	}

	uvOne, err := testTiDB.DB.Query("SELECT * FROM up_vote WHERE discussion_id = ? and discussion_type = 0", discId)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	var upVote *models.UpVote

	for uvOne.Next() {
		upVote, err = models.UpVoteFromSQLNative(uvOne)
		if err != nil {
			t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		}
	}

	if upVote == nil || upVote.UserId != 69 || upVote.DiscussionId != discId {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect result set")
		return
	}

	commId, err := strconv.ParseInt(comment["comment"].(*models.CommentFrontend).ID, 10, 64)

	resTwo, err := AddDiscussionCoffee(context.Background(), testTiDB, user, testSnowflake, commId, "comment")
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if resTwo["message"].(string) != "Coffee added to discussion" {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect message returned")
	}

	checkTwo, err := testTiDB.DB.Query("SELECT * FROM comment WHERE _id = ?", commId)
	if err != nil {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if !checkTwo.Next() {
		t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect result set")
		return
	} else {
		resTwoCheck, err := models.CommentFromSQLNative(testTiDB, checkTwo)
		if err != nil {
			t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: %v\n", err)
			return
		}
		if strconv.FormatUint(resTwoCheck.Coffee-1, 10) != comment["comment"].(*models.CommentFrontend).Coffee {
			t.Errorf("\nTestAddDiscussionCoffee failed\n    Error: incorrect coffee returned")
		}
	}
}

func TestRemoveDiscussionCoffee(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	var ava models.AvatarSettings

	user, err := models.CreateUser(69, "test", "", "", "", models.UserStatusBasic, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", ava, 0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	cfg := config.MeiliConfig{
		Host:  "http://gigo-dev-meili:7700",
		Token: "gigo-dev",
		Indices: map[string]config.MeiliIndexConfig{
			"discussion": {
				Name:                 "discussion",
				PrimaryKey:           "_id",
				SearchableAttributes: []string{"body", "author"},
				FilterableAttributes: []string{},
				SortableAttributes:   []string{},
			},
		},
	}

	meili, err := search.CreateMeiliSearchEngine(cfg)
	if err != nil {
		t.Fatalf("\n%s failed\n    Error: %v", t.Name(), err)
	}

	testSnowflake, err := snowflake.NewNode(0)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	defer func() {
		meili.DeleteDocuments("discussion", 69)
	}()

	discussion, err := CreateDiscussion(context.Background(), testTiDB, meili, user, testSnowflake, 69, "title", "body", nil)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	comment, err := CreateComment(context.Background(), testTiDB, meili, user, testSnowflake, 69, "body")
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	discId, err := strconv.ParseInt(discussion["discussion"].(*models.DiscussionFrontend).ID, 10, 64)

	_, err = testTiDB.DB.Exec("update discussion set coffee = 2 where _id = ?", discId)

	resOne, err := RemoveDiscussionCoffee(context.Background(), testTiDB, user, discId, "discussion")
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if resOne["message"].(string) != "Coffee removed from discussion" {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect message returned")
	}

	checkOne, err := testTiDB.DB.Query("SELECT * FROM discussion WHERE _id = ?", discId)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if !checkOne.Next() {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect result set")
		return
	} else {
		resOneCheck, err := models.DiscussionFromSQLNative(testTiDB, checkOne)
		if err != nil {
			t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
			return
		}
		if resOneCheck.Coffee != 1 {
			t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect coffee returned")
		}
	}

	uvOne, err := testTiDB.DB.Query("SELECT * FROM up_vote WHERE discussion_id = ? and discussion_type = 0", discId)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if uvOne.Next() {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: upvote not deleted")
		return
	}

	commId, err := strconv.ParseInt(comment["comment"].(*models.CommentFrontend).ID, 10, 64)

	_, err = testTiDB.DB.Exec("update comment set coffee = 2 where _id = ?", commId)

	resTwo, err := RemoveDiscussionCoffee(context.Background(), testTiDB, user, commId, "comment")
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if resTwo["message"].(string) != "Coffee removed from discussion" {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect message returned")
	}

	checkTwo, err := testTiDB.DB.Query("SELECT * FROM comment WHERE _id = ?", commId)
	if err != nil {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
		return
	}

	if !checkTwo.Next() {
		t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect result set")
		return
	} else {
		resTwoCheck, err := models.CommentFromSQLNative(testTiDB, checkTwo)
		if err != nil {
			t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: %v\n", err)
			return
		}
		if resTwoCheck.Coffee != 1 {
			t.Errorf("\nTestRemoveDiscussionCoffee failed\n    Error: incorrect coffee returned")
		}
	}
}
