package core

import (
	"encoding/json"
	"github.com/gage-technologies/gitea-go/gitea"
	"github.com/google/go-cmp/cmp"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestJsonifyGiteaResponse(t *testing.T) {
	testCases := []struct {
		name     string
		response *gitea.Response
		expected string
	}{
		// ... other test cases ...
		{
			name: "NonUtf8",
			response: &gitea.Response{
				Response: &http.Response{
					Status:        "200 OK",
					StatusCode:    200,
					Header:        http.Header{"Content-Type": []string{"application/octet-stream"}},
					ContentLength: 4,
					Body:          io.NopCloser(strings.NewReader("\x80\x80\x80\x80")),
				},
			},
			expected: `{"status":"200 OK","status_code":200,"headers":{"Content-Type":["application/octet-stream"]},"content_length":4,"body":"gICAgA=="}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := JsonifyGiteaResponse(tc.response)

			// Compare JSON strings by unmarshaling them into maps and comparing the maps
			var resultMap, expectedMap map[string]interface{}
			if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
				t.Errorf("Error unmarshaling result: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.expected), &expectedMap); err != nil {
				t.Errorf("Error unmarshaling expected: %v", err)
			}

			if !cmp.Equal(resultMap, expectedMap) {
				t.Errorf("JsonifyGiteaResponse() = %v, want %v", resultMap, expectedMap)
			}
		})
	}
}
