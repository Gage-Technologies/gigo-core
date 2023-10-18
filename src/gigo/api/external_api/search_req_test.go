package external_api

import (
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPServer_CompleteSearch(t *testing.T) {
	// Create a test search record
	testSearchRecord := models.SearchRec{
		ID:    1,
		Query: "config",
	}

	searchRecStmt := testSearchRecord.ToSQLNative()

	for _, stmt := range searchRecStmt {
		_, err := testHttpServer.tiDB.DB.Exec(stmt.Statement, stmt.Values...)
		if err != nil {
			t.Fatalf("Failed to insert test search record: %v", err)
		}
	}

	defer func() {
		testHttpServer.tiDB.DB.Exec("delete from search_records")
	}()

	// Send a request to the CompleteSearch endpoint
	payload := `{
		"search_rec_id": "1",
		"query": "config",
		"post_id": "1"
	}`
	req, err := http.NewRequest("POST", "http://localhost:1818/api/search/complete", strings.NewReader(payload))
	if err != nil {
		t.Errorf("\nTestHTTPServer_CompleteSearch failed\n    Error: %v", err)
		return
	}

	req.AddCookie(&http.Cookie{
		Name:  "gigoAuthToken",
		Value: testUserAuth,
	})

	// Perform the request
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		t.Errorf("\nTestHTTPServer_CompleteSearch failed\n    Error: %v", err)
		return
	}

	// Check the response status code
	if res.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(body))
		fmt.Println(res.StatusCode)
		t.Error("\nTestHTTPServer_CompleteSearch failed\n    Error: incorrect response code")
		return
	}

	t.Log("\nTestHTTPServer_CompleteSearch succeeded")
}
