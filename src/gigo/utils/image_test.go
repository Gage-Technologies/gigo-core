package utils

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gage-technologies/gigo-lib/utils"
)

func TestPrepImageFile(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	basepath := strings.Replace(filepath.Dir(b), "/src/gigo/utils", "", -1)

	tests := []struct {
		name     string
		path     string
		vertical bool
		want     string
	}{
		{
			name:     "test1",
			path:     filepath.Join(basepath, "test_data", "images", "jpeg", "0a5db4b2b984afdc.jpg"),
			vertical: false,
			want:     "3b5052005a18d7ed3f216b05c695dd293ee963364650a2dd0bd777fce4c0e44a",
		},
		{
			name:     "test2",
			path:     filepath.Join(basepath, "test_data", "images", "png", "gage.png"),
			vertical: false,
			want:     "e381059f8889f1e97a49ecad9bccf468fb849b29897dad82257ac1a03906d2b3",
		},
		{
			name:     "test3",
			path:     filepath.Join(basepath, "test_data", "api-key"),
			vertical: false,
			want:     "fail",
		},
		{
			name:     "test4",
			path:     filepath.Join(basepath, "test_data", "images", "jpeg", "0a5db4b2b984afdc.jpg"),
			vertical: true,
			want:     "8fb918f5bbfecbadab8c0d69d3dd3895e6754eeb60f77a9a48e75edc0b212166",
		},
		{
			name:     "test5",
			path:     filepath.Join(basepath, "test_data", "images", "png", "gage.png"),
			vertical: true,
			want:     "c902745b5bbea6be727ab1bb596d06f79cf27248c359be5898fd00786545e28a",
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

			err = PrepImageFile(srcFile, dst, tt.vertical)
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
