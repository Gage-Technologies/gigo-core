package utils

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"github.com/bits-and-blooms/bloom/v3"
	"github.com/gage-technologies/gigo-lib/storage"
	"strings"
)

type PasswordFilter struct {
	filter *bloom.BloomFilter
}

func NewPasswordFilter(storageEngine storage.Storage) (*PasswordFilter, error) {
	// get bloom filter from storage
	f, err := storageEngine.GetFile("pass/filter.db")
	if err != nil {
		return nil, fmt.Errorf("unable to get password filter file: %s", err)
	}

	defer f.Close()

	// read the bloom filter from storage
	filter := &bloom.BloomFilter{}
	_, err = filter.ReadFrom(f)
	if err != nil {
		return nil, fmt.Errorf("unable to read password filter: %s", err)
	}

	// return the filter
	return &PasswordFilter{
		filter: filter,
	}, nil

}

func (f *PasswordFilter) CheckPasswordFilter(password string) (bool, error) {
	// Generate sha1 sum of password
	h := sha1.New()
	h.Write([]byte(password))
	data := h.Sum(nil)

	// Convert to hex
	shaStr := hex.EncodeToString(data)

	// Make uppercase hex to filter in bitset
	up := strings.ToUpper(shaStr)

	// return password exist status in bitset
	return f.filter.TestString(up), nil
}
