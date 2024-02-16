package external_api

import (
	"io"
	"net/http"

	"github.com/gage-technologies/gigo-lib/network"
)

func (s *HTTPServer) GetSitemap(w http.ResponseWriter, r *http.Request) {
	// retrieve the sitemap from the storage engine
	file, _, err := s.storageEngine.GetFile("sitemap/sitemap.xml")
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to copy image file", r.URL.Path, "GetSitemap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "failed to retrieve sitemap", err)
		return
	}

	// return 404 if the sitemap isn't found
	if file == nil {
		// handle error internally
		s.handleError(w, "sitemap not found", r.URL.Path, "GetSitemap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusNotFound, "sitemap not found", err)
		return
	}

	// defer closure of image file
	defer file.Close()

	// add headers
	w.Header().Set("Content-Type", "text/xml")

	// Cache the image for up to 1 day
	w.Header().Set("Cache-Control", "public, max-age=86400")

	// set status code
	w.WriteHeader(200)

	// copy image to response
	_, err = io.Copy(w, file)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to copy sitemap file", r.URL.Path, "GetSitemap", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "anon", "-1", http.StatusInternalServerError, "failed to copy sitemap file", err)
		return
	}

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "GetSitemap", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "anon", "-1", http.StatusOK, nil)
}
