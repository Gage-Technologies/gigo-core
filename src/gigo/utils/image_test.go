package utils

import (
	"github.com/gage-technologies/gigo-lib/utils"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPrepImageFile(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	basepath := strings.Replace(filepath.Dir(b), "/src/gigo/utils", "", -1)

	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "test1",
			path: filepath.Join(basepath, "test_data", "images", "jpeg", "0a5db4b2b984afdc.jpg"),
			want: "b8b2cf4184b7139439b5f1b0de36819652a13ecf75888d150ac82f5823f7bc39",
		},
		{
			name: "test2",
			path: filepath.Join(basepath, "test_data", "images", "png", "gage.png"),
			want: "9b6bbe02f8227060b81987c40b013edc79ce5fa2ad7f08f54a054272f6c0fcc1",
		},
		{
			name: "test3",
			path: filepath.Join(basepath, "test_data", "api-key"),
			want: "fail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dst, err := os.CreateTemp("/tmp", "prep-image-file")
			if err != nil {
				t.Fatalf("\nPrepImageFile failed\n    Error: %v", err)
			}

			defer os.Remove(dst.Name())

			srcFile, err := os.Open(tt.path)
			if err != nil {
				t.Fatalf("\nPrepImageFile failed\n    Error: %v", err)
			}

			err = PrepImageFile(srcFile, dst)
			if tt.want != "fail" && err != nil {
				t.Fatalf("\nPrepImageFile failed\n    Error: %v", err)
			}

			if tt.want == "fail" {
				if err != nil {
					t.Logf("\nPrepImageFile succeeded")
					return
				}
				t.Fatalf("\nPrepImageFile failed\n    Error: succeeded on an invalid image file")
			}

			h, err := utils.HashFile(dst.Name())
			if err != nil {
				t.Fatalf("\nPrepImageFile failed\n    Error: %v", err)
			}

			if h != tt.want {
				t.Fatalf("\nPrepImageFile failed\n    want: %v\n    got: %v", tt.want, h)
			}

			t.Logf("\nPrepImageFile succeeded")
		})
	}
}
