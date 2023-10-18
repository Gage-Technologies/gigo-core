package core

import (
	"encoding/base64"
	"encoding/json"
	"github.com/gage-technologies/gitea-go/gitea"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// TeeReadCloser
//
//	Implements the io.ReadCloser interface for the io.TeeReader
type TeeReadCloser struct {
	teeReader  io.Reader
	teeCloser  io.Closer
	mainCloser io.Closer
}

func NewTeeReadCloser(main io.ReadCloser, tee io.WriteCloser) *TeeReadCloser {
	return &TeeReadCloser{
		teeReader:  io.TeeReader(main, tee),
		teeCloser:  tee,
		mainCloser: main,
	}
}

func (t *TeeReadCloser) Read(p []byte) (int, error) {
	return t.teeReader.Read(p)
}

func (t *TeeReadCloser) Close() error {
	err1 := t.mainCloser.Close()
	err2 := t.teeCloser.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

// ChannelBuffer
//
//	Implements the io.ReadWriteCloser interface for a channel so that
//	it can be used as a buffer for a tee reader.
type ChannelBuffer struct {
	ch     chan byte
	closed chan struct{}
}

func NewChannelBuffer(size int) *ChannelBuffer {
	return &ChannelBuffer{
		ch:     make(chan byte, size),
		closed: make(chan struct{}),
	}
}

func (cb *ChannelBuffer) Write(p []byte) (n int, err error) {
	for i, b := range p {
		select {
		case <-cb.closed:
			return i, io.ErrClosedPipe
		case cb.ch <- b:
		}
	}
	return len(p), nil
}

func (cb *ChannelBuffer) Read(p []byte) (n int, err error) {
	for i := range p {
		select {
		case <-cb.closed:
			return i, nil
		case b, ok := <-cb.ch:
			if !ok {
				return i, io.EOF
			}
			p[i] = b
		}
	}
	return len(p), nil
}

func (cb *ChannelBuffer) Close() error {
	close(cb.closed)
	close(cb.ch)
	return nil
}

// JsonifyGiteaResponse
//
//	Formats a gitea response as a JSON string with best effort try
//	ignoring any errors that occur in the formatting.
func JsonifyGiteaResponse(response *gitea.Response) string {
	// return an empty json for a nil body
	if response == nil {
		return "{}"
	}

	// attempt to read the body of the response
	body, _ := io.ReadAll(response.Body)

	// cap the body at 4Kib
	if len(body) > 4096 {
		body = body[:4096]
	}

	// base64 encode binary data
	if !utf8.Valid(body) {
		base64.StdEncoding.Encode(body, body)
	}

	// marshall the body into a json string
	buf, _ := json.Marshal(map[string]interface{}{
		"status":         response.Status,
		"status_code":    response.StatusCode,
		"headers":        response.Header,
		"content_length": response.ContentLength,
		"body":           string(body),
	})

	return string(buf)
}

// IsCompatible checks if the given version is compatible with the constraint.
// Returns true if compatible, otherwise returns false.
func IsCompatible(version string, constraint string) bool {
	// Remove caret symbol if present
	if strings.HasPrefix(constraint, "^") {
		constraint = strings.TrimPrefix(constraint, "^")
	}

	// Remove build metadata or pre-release identifier from versions
	version = strings.Split(version, "-")[0]
	constraint = strings.Split(constraint, "-")[0]

	// Convert to slices of integers for easy comparison
	versionNumbers := convertVersionToIntSlice(version)
	constraintNumbers := convertVersionToIntSlice(constraint)

	return checkCompatibility(versionNumbers, constraintNumbers)
}

// Converts a version string into a slice of integers
func convertVersionToIntSlice(version string) []int {
	parts := strings.Split(version, ".")
	nums := make([]int, len(parts))

	for i, part := range parts {
		num, _ := strconv.Atoi(part)
		nums[i] = num
	}

	return nums
}

// Checks if a version is compatible with a constraint
func checkCompatibility(version, constraint []int) bool {
	for i := 0; i < len(constraint); i++ {
		// If major version is different, return false
		if i == 0 && version[i] != constraint[i] {
			return false
		}

		// If major version is same but minor or patch version is less in version, return false
		if i != 0 && version[i] < constraint[i] {
			return false
		}

		// If major version is same but minor or patch version is greater in version, return true
		if i != 0 && version[i] > constraint[i] {
			return true
		}
	}

	// If major, minor and patch are the same or greater, return true
	return true
}
