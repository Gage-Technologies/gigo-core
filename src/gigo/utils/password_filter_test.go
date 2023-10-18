package utils

import (
	"fmt"
	"github.com/gage-technologies/gigo-lib/storage"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckPasswordFilter(t *testing.T) {
	_, b, _, _ := runtime.Caller(0)
	basepath := strings.Replace(filepath.Dir(b), "/src/gigo/utils", "", -1)

	// create storage interface
	testStorage, err := storage.CreateFileSystemStorage(basepath + "/test_data")
	if err != nil {
		log.Panicf("Error: %v", err)
	}

	filter, err := NewPasswordFilter(testStorage)

	// if this works, it returns true for "password"
	check, err := filter.CheckPasswordFilter("password")

	fmt.Println(check)
}
