package external_api

//func TestHTTPServer_UiFiles(t *testing.T) {
//	// Prepare the request
//	req, err := http.NewRequest("GET", "http://localhost:1818/path-to-existing-file", nil)
//	if err != nil {
//		t.Fatalf("Failed to create request: %v", err)
//	}
//
//	req.AddCookie(&http.Cookie{
//		Name:  "gigoAuthToken",
//		Value: testUserAuth,
//	})
//
//	// Perform the request
//	res, err := client.Do(req)
//	if err != nil {
//		t.Errorf("\nTestHTTPServer_UiFiles failed\n    Error: %v", err)
//		return
//	}
//
//	// Check the response status code
//	if res.StatusCode != http.StatusOK {
//		body, _ := ioutil.ReadAll(res.Body)
//		fmt.Println(string(body))
//		fmt.Println(res.StatusCode)
//		t.Error("\nTestHTTPServer_UiFiles failed\n    Error: incorrect response code")
//		return
//	}
//
//	// Check the Content-Type header
//	contentType := res.Header.Get("Content-Type")
//	expectedContentType := "expected/mime-type" // replace with the expected MIME type
//	if contentType != expectedContentType {
//		t.Errorf("\nTestHTTPServer_UiFiles failed\n    Error: incorrect Content-Type, expected %v, got %v", expectedContentType, contentType)
//		return
//	}
//
//	t.Log("\nTestHTTPServer_UiFiles succeeded")
//}
