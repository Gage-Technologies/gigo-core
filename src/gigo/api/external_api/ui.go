package external_api

import (
	"fmt"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/h2non/filetype"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"
)

func (s *HTTPServer) UiFiles(w http.ResponseWriter, r *http.Request) {
	_, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "ui-files-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
	}

	// get path of image in repo
	path := strings.TrimPrefix(r.URL.Path, "/")

	// get mime type of file for header
	fileMime := mime.TypeByExtension(filepath.Ext(path))

	// conditionally infer the mime type
	if fileMime == "" {
		// get file to determine the mime type
		mimeReader, _, err := s.storageEngine.GetFileByteRange(path, 0, 8196)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to retrieve file", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
			// exit
			return
		}

		// handle file not found
		if mimeReader == nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("file not found: %s", path), r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusNotFound, "not found", err)
			// exit
			return
		}

		defer mimeReader.Close()

		// read the first 8196 bytes or less of the file
		mimeBuf := make([]byte, 8196)
		n, err := mimeReader.Read(mimeBuf)
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to read mime buffer", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
			// exit
			return
		}

		// determine the mime type
		mimeType, err := filetype.Match(mimeBuf[:n])
		if err != nil {
			// handle error internally
			s.handleError(w, "failed to determine mime type", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
			// exit
			return
		}

		// handle unknown by setting to octet-stream
		if mimeType == filetype.Unknown {
			fileMime = "application/octet-stream"
		} else {
			fileMime = mimeType.MIME.Value
		}
	}

	// add headers
	w.Header().Set("Content-Type", fileMime)

	// Cache the image for up to 10 minutes
	w.Header().Set("Cache-Control", "public, max-age=600")

	var start, end int64 = 0, -1

	rangeHeader := r.Header.Get("Range")
	if rangeHeader != "" {
		// Parse the Range header
		_, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
		if err != nil {
			_, err = fmt.Sscanf(rangeHeader, "bytes=%d-", &start)
			if err != nil {
				s.handleError(w, "invalid range header", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), userName, userId, http.StatusBadRequest, "invalid range header", fmt.Errorf("invalid range header: "+rangeHeader))
				return
			}
		}
	}

	// Adjust the call to GetFile to include the range parameters
	var fileBuf io.ReadCloser
	var fileSize int64
	var err error
	if start != 0 && end != -1 {
		fileBuf, fileSize, err = s.storageEngine.GetFileByteRange(path, start, end-start+1)
		if err != nil {
			s.handleError(w, "failed to retrieve file content", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
			return
		}
	} else {
		fileBuf, fileSize, err = s.storageEngine.GetFile(path)
		if err != nil {
			s.handleError(w, "failed to retrieve file content", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
			return
		}
	}

	// handle file not found
	if fileBuf == nil {
		// handle error internally
		s.handleError(w, fmt.Sprintf("file not found: %s", path), r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusNotFound, "not found", err)
		// exit
		return
	}

	status := http.StatusOK

	// Before writing the file to the response, check if a range was requested and handle accordingly
	if start != 0 && end != -1 {
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
		status = http.StatusPartialContent
	}

	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileSize))
	w.WriteHeader(status)

	defer fileBuf.Close()

	// set status code
	w.WriteHeader(200)

	// copy image to response
	_, err = io.Copy(w, fileBuf)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to copy file", r.URL.Path, "UiFiles", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error", err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"ui-files",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "UiFiles", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, status, nil)
}
