package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var gitImagePathCleaner = regexp.MustCompile("^/static/git/a|p/[0-9]+/")

func (s *HTTPServer) SiteImages(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "site-images-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	var finalCallingUser *models.User

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	if callingUser != nil {
		finalCallingUser = callingUser.(*models.User)
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
	} else {
		finalCallingUser = nil
	}

	// attempt to retrieve target id from url
	vars := mux.Vars(r)
	idString, ok := vars["id"]
	if !ok {
		// handle error internally
		s.handleError(w, "no id found in path", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusMethodNotAllowed, "invalid path", nil)
		return
	}

	source := models.CodeSource(-1)
	// check if the called path is for a post
	if strings.HasPrefix(r.URL.Path, "/static/posts") {
		source = models.CodeSourcePost
	} else if strings.HasPrefix(r.URL.Path, "/static/attempts") {
		source = models.CodeSourceAttempt
	} else if strings.HasPrefix(r.URL.Path, "/static/bytes") {
		source = models.CodeSourceByte
	}

	// parse id to int if all the characters are numerical
	var username string
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		username = idString
		if source != -1 {
			// handle error internally
			s.handleError(w, "failed to parse id to int", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusUnprocessableEntity, "invalid id", err)
			return
		}
	}

	// execute core function logic
	img, err := core.SiteImages(ctx, finalCallingUser, s.tiDB, id, username, source, s.storageEngine)
	if err != nil {
		if err.Error() == "not found" {
			s.handleError(w, "SiteImages not found", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusNotFound, "not found", err)
			return
		}
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"errorMessage": err})
		// handle error internally
		s.handleError(w, "SiteImages core failed", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}
	if img == nil {
		s.handleError(w, "SiteImages not found", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusNotFound, "not found", err)
		return
	}

	// defer closure of image file
	defer img.Close()

	// add headers
	if source != -1 {
		w.Header().Set("Content-Type", "image/jpeg")
	} else {
		w.Header().Set("Content-Type", "image/svg+xml")
	}

	// Cache the image for up to 10 minutes
	w.Header().Set("Cache-Control", "public, max-age=600")

	// set status code
	w.WriteHeader(200)

	// copy image to response
	_, err = io.Copy(w, img)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to copy image file", r.URL.Path, "SiteImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "failed to copy image file", err)
		return
	}

	parentSpan.AddEvent(
		"site-images",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "SiteImages", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK, nil)
}

func (s *HTTPServer) GitImages(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "git-images-http")
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

	// attempt to retrieve target id from url
	vars := mux.Vars(r)
	idString, ok := vars["id"]
	if !ok {
		// handle error internally
		s.handleError(w, "no id found in path", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusMethodNotAllowed, "invalid path", nil)
		return
	}

	// parse id to int
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to parse id to int", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusUnprocessableEntity, "invalid id", err)
		return
	}

	// check if the called path is for a post
	post := strings.HasPrefix(r.URL.Path, "/static/git/p")

	// get path of image in repo
	path := strings.ReplaceAll(gitImagePathCleaner.ReplaceAllString(r.URL.Path, ""), "/static/git/", "")

	// ensure that path is image type
	ext := filepath.Ext(path)
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" &&
		ext != ".gif" && ext != ".svg" && ext != ".webp" && ext != ".ico" {
		// handle error internally
		s.handleError(w, "invalid image type", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusUnprocessableEntity, "invalid image type", nil)
		return
	}

	// execute core function logic
	imgBytes, err := core.GitImages(ctx, callingUser.(*models.User), s.tiDB, id, post, path, s.vscClient)
	if err != nil {
		if err.Error() == "not found" {
			s.handleError(w, "GitImages not found", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusNotFound, "not found", err)
			return
		}
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"errorMessage": err})
		// handle error internally
		s.handleError(w, "GitImages core failed", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	// add headers
	w.Header().Set("Content-Type", "image/*")

	// Cache the image for up to 10 minutes
	w.Header().Set("Cache-Control", "public, max-age=600")

	// set status code
	w.WriteHeader(200)

	// copy image to response
	_, err = w.Write(imgBytes)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to write image file", r.URL.Path, "GitImages", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "failed to copy image file", err)
		return
	}

	parentSpan.AddEvent(
		"git-images-http",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "GitImages", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK, nil)
}

func (s *HTTPServer) GetGeneratedImage(w http.ResponseWriter, r *http.Request) {
	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// return if calling user was not retrieved in authentication
	if callingUser == nil {
		s.handleError(w, "calling user missing from context", r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, fmt.Sprintf("%v", callingUser.(*models.User).ID), http.StatusInternalServerError, "internal server error occurred", nil)
		return
	}

	callingId := fmt.Sprintf("%v", callingUser.(*models.User).ID)
	callingUserName := callingUser.(*models.User).UserName

	// attempt to retrieve image id from url
	vars := mux.Vars(r)
	idString, ok := vars["id"]
	if !ok {
		// handle error internally
		s.handleError(w, "no id found in path", r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusMethodNotAllowed, "invalid path", nil)
		return
	}

	// parse id to int
	id, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to parse id to int", r.URL.Path, "GenerateProjectImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusUnprocessableEntity, "invalid id", err)
		return
	}

	// execute core function logic
	img, err := core.GetGeneratedImage(callingUser.(*models.User), id, s.storageEngine)
	if err != nil {
		if err.Error() == "not found" {
			s.handleError(w, "GeneratedImage not found", r.URL.Path, "GetGeneratedImage", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), callingUserName, callingId, http.StatusNotFound, "not found", err)
			return
		}
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", map[string]interface{}{"errorMessage": err})
		// handle error internally
		s.handleError(w, "GetGeneratedImage core failed", r.URL.Path, "GetGeneratedImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	if img == nil {
		s.handleError(w, "GetGeneratedImage Failed: img is nil", r.URL.Path, "GetGeneratedImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUserName, callingId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// defer closure of image file
	defer img.Close()

	// add headers
	w.Header().Set("Content-Type", "image/jpeg")

	// Cache the image for up to 10 minutes
	w.Header().Set("Cache-Control", "public, max-age=600")

	// set status code
	w.WriteHeader(200)

	// copy image to response
	_, err = io.Copy(w, img)
	if err != nil {
		// handle error internally
		s.handleError(w, "failed to copy image file", r.URL.Path, "GetGeneratedImage", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusInternalServerError, "failed to copy image file", err)
		return
	}

	// log successful function execution
	s.logger.LogDebugExternalAPI("function execution successful", r.URL.Path, "GetGeneratedImage", r.Method,
		r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), callingUser.(*models.User).UserName, callingId, http.StatusOK, nil)
}
