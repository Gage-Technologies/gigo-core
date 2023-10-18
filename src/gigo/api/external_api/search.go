package external_api

import (
	"fmt"
	"gigo-core/gigo/api/external_api/core"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/network"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (s *HTTPServer) SearchPosts(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-posts-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUserI := r.Context().Value(CtxKeyUser)

	userName := ""
	userId := ""
	var userIdInt int64
	var callingUser *models.User

	// return if calling user was not retrieved in authentication
	if callingUserI == nil {
		userName = network.GetRequestIP(r)
		userId = network.GetRequestIP(r)
	} else {
		callingUser = callingUserI.(*models.User)
		userName = callingUser.UserName
		userId = fmt.Sprintf("%d", callingUser.ID)
		userIdInt = callingUser.ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchPosts", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchPosts", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "languages", reflect.Slice, &langsType, true, userName, userId)
	if !ok {
		return
	}

	var languages []models.ProgrammingLanguage = nil
	if languagesI != nil {
		// create a slice to hold languages loaded from the http parameter
		tempLanguages := make([]models.ProgrammingLanguage, 0)

		// attempt to load parameter from body
		for _, lang := range languagesI.([]interface{}) {
			tempLanguages = append(tempLanguages, models.ProgrammingLanguage(lang.(float64)))
		}
		languages = tempLanguages
	}

	// attempt to load parameter from body
	authorI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "author", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var author *int64 = nil
	if authorI != nil {
		tempAuthor, err := strconv.ParseInt(authorI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", authorI), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		author = &tempAuthor
	}

	// attempt to load ownerCount from body
	attemptMinI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "attempts_min", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var attemptsMin *int64 = nil
	if attemptMinI != nil {
		// parse post ownerCount to integer
		tempAttemptsMin, err := strconv.ParseInt(attemptMinI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", attemptMinI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		attemptsMin = &tempAttemptsMin
	}

	// attempt to load ownerCount from body
	attemptsMaxI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "attempts_max", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var attemptsMax *int64 = nil
	if attemptsMaxI != nil {
		// parse post ownerCount to integer
		tempAttemptsMax, err := strconv.ParseInt(attemptsMaxI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", attemptsMaxI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		attemptsMax = &tempAttemptsMax
	}

	// attempt to load ownerCount from body
	completionsMinI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "completions_min", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var completionsMin *int64 = nil
	if completionsMinI != nil {
		// parse post ownerCount to integer
		tempCompletionsMin, err := strconv.ParseInt(completionsMinI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse completions string to integer: %s", completionsMinI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		completionsMin = &tempCompletionsMin
	}

	// attempt to load ownerCount from body
	completionsMaxI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "completions_max", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var completionsMax *int64 = nil
	if completionsMaxI != nil {
		// parse post ownerCount to integer
		tempCompletionsMax, err := strconv.ParseInt(completionsMaxI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse completions string to integer: %s", completionsMaxI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		completionsMax = &tempCompletionsMax
	}

	// attempt to load ownerCount from body
	coffeeMinI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "coffee_max", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var coffeeMin *int64 = nil
	if coffeeMinI != nil {
		// parse post ownerCount to integer
		tempCoffeeMin, err := strconv.ParseInt(coffeeMinI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse coffee string to integer: %s", coffeeMinI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		coffeeMin = &tempCoffeeMin
	}

	// attempt to load ownerCount from body
	coffeeMaxI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "coffee_max", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var coffeeMax *int64 = nil
	if coffeeMaxI != nil {
		// parse post ownerCount to integer
		tempCoffeeMax, err := strconv.ParseInt(coffeeMaxI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse coffee string to integer: %s", coffeeMaxI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		coffeeMax = &tempCoffeeMax
	}

	// attempt to load ownerCount from body
	viewsMinI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "views_min", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var viewsMin *int64 = nil
	if viewsMinI != nil {
		// parse post ownerCount to integer
		tempViewsMin, err := strconv.ParseInt(viewsMinI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse views string to integer: %s", viewsMinI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		viewsMin = &tempViewsMin
	}

	// attempt to load ownerCount from body
	viewsMaxI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "views_max", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var viewsMax *int64 = nil
	if viewsMaxI != nil {
		// parse post ownerCount to integer
		tempViewsMax, err := strconv.ParseInt(viewsMaxI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to views coffee string to integer: %s", viewsMaxI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		viewsMax = &tempViewsMax
	}

	// attempt to load parameter from body
	tagsType := reflect.String
	tagsI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "tags", reflect.Slice, &tagsType, true, userName, userId)
	if !ok {
		return
	}

	var tagIds []int64 = nil
	if tagsI != nil {
		// create array to hold discussion ids
		tempTags := make([]int64, 0)

		// iterate tag ids interface slice asserting each value as a string and parsing to an int64
		for _, tagId := range tagsI.([]interface{}) {
			// parse tag id to integer
			tId, err := strconv.ParseInt(tagId.(string), 10, 64)
			if err != nil {
				// handle error internally
				s.handleError(w, fmt.Sprintf("failed to convert tag ID string to integer: %s", tagsI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
					network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
				// exit
				return
			}
			// append to outer slice
			tempTags = append(tempTags, tId)
		}
		tagIds = tempTags
	}

	// attempt to load parameter from body
	challengeI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "challenge_type", reflect.Float64, nil, true, userName, userId)
	if !ok {
		return
	}

	// load challenge type if it was passed
	var challenge *models.ChallengeType = nil
	if challengeI != nil {
		tempChallenge := models.ChallengeType(challengeI.(float64))
		challenge = &tempChallenge
	}

	// attempt to load parameter from body
	visibilityI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "visibility_type", reflect.Float64, nil, true, userName, userId)
	if !ok {
		return
	}

	// load challenge type if it was passed
	var visibility *models.PostVisibility = nil
	if visibilityI != nil {
		tempVisibility := models.PostVisibility(visibilityI.(float64))
		visibility = &tempVisibility
	}

	// attempt to load ownerCount from body
	sinceDateI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "since", reflect.Float64, nil, true, userName, userId)
	if !ok {
		return
	}

	var since *time.Time = nil
	if sinceDateI != nil {
		tempSince := time.Unix(int64(sinceDateI.(float64)), 0)
		since = &tempSince
	}

	// attempt to load ownerCount from body
	untilDateI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "until", reflect.Float64, nil, true, userName, userId)
	if !ok {
		return
	}

	var until *time.Time = nil
	if untilDateI != nil {
		tempUntil := time.Unix(int64(untilDateI.(float64)), 0)
		until = &tempUntil
	}

	// attempt to load new username from body
	publishedI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "published", reflect.Bool, nil, true, userName, userId)
	if !ok {
		return
	}

	var published *bool = nil
	if publishedI != nil {
		tempPublished := publishedI.(bool)
		published = &tempPublished
	}

	// attempt to load parameter from body
	tierI, ok := s.loadValue(w, r, reqJson, "SearchPosts", "tier", reflect.Float64, nil, true, userName, userId)
	if !ok {
		return
	}

	var tier *models.TierType = nil
	if tierI != nil {
		tempTier := models.TierType(tierI.(float64))
		tier = &tempTier
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchPosts", "skip", reflect.Float64, nil, false, "not logged in", "0")
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchPosts", "limit", reflect.Float64, nil, false, "not logged in", "0")
	if limit == nil || !ok {
		return
	}

	// attempt to load new username from body
	rawSearchRecId, ok := s.loadValue(w, r, reqJson, "SearchPosts", "search_rec_id", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var searchRecId *int64 = nil
	if rawSearchRecId != nil {
		// parse post ownerCount to integer
		tempSearchRec, err := strconv.ParseInt(rawSearchRecId.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse search rec id string to integer: %s", rawSearchRecId.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		searchRecId = &tempSearchRec
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), "not logged in", "0", http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SearchPosts(
		ctx,
		s.tiDB,
		s.sf,
		s.meili,
		callingUser,
		query.(string),
		languages,
		author,
		attemptsMin,
		attemptsMax,
		completionsMin,
		completionsMax,
		coffeeMin,
		coffeeMax,
		viewsMin,
		viewsMax,
		tagIds,
		challenge,
		visibility,
		since,
		until,
		published,
		tier,
		int(skip.(float64)),
		int(limit.(float64)),
		searchRecId,
		s.logger,
	)

	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchPosts core failed", r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-posts",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchUsers(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-users-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	} else {
		userName = network.GetRequestIP(r)
		userId = network.GetRequestIP(r)
		userIdInt = 0
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchUsers", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchUsers", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchUsers", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchUsers", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.SearchUsers(ctx, s.tiDB, s.meili, query.(string), int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchUsers core failed", r.URL.Path, "SearchUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-users",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchTags(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-tags-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchTags", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchTags", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchTags", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchTags", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchTags", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.SearchTags(ctx, s.meili, query.(string), int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchTags core failed", r.URL.Path, "SearchTags", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-tags",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchTags", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchDiscussions(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-discussion-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchDiscussions", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchDiscussions", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchDiscussions", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchDiscussions", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// attempt to load ownerCount from body
	postIdI, ok := s.loadValue(w, r, reqJson, "SearchDiscussions", "post_id", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var postId *int64 = nil
	if postIdI != nil {
		// parse post ownerCount to integer
		tempPostId, err := strconv.ParseInt(postIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", postIdI.(string)), r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		postId = &tempPostId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchDiscussions", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SearchDiscussions(ctx, s.meili, query.(string), int(skip.(float64)), int(limit.(float64)), postId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchDiscussions core failed", r.URL.Path, "SearchDiscussions", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-discussions",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchDiscussions", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchComments(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-comments-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchComments", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchComments", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchComments", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchComments", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// attempt to load ownerCount from body
	discussionIdI, ok := s.loadValue(w, r, reqJson, "SearchComments", "discussion_id", reflect.String, nil, true, userName, userId)
	if !ok {
		return
	}

	var discussionId *int64 = nil
	if discussionIdI != nil {
		// parse post ownerCount to integer
		tempDiscussionId, err := strconv.ParseInt(discussionIdI.(string), 10, 64)
		if err != nil {
			// handle error internally
			s.handleError(w, fmt.Sprintf("failed to parse attempts string to integer: %s", discussionIdI.(string)), r.URL.Path, "SearchComments", r.Method, r.Context().Value(CtxKeyRequestID),
				network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
			// exit
			return
		}
		discussionId = &tempDiscussionId
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchComments", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// execute core function logic
	res, err := core.SearchComments(ctx, s.meili, query.(string), int(skip.(float64)), int(limit.(float64)), discussionId)
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchComments core failed", r.URL.Path, "SearchComments", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-comments",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", userName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchComments", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchWorkspaceConfigs(w http.ResponseWriter, r *http.Request) {
	ctx, parentSpan := otel.Tracer("gigo-core").Start(r.Context(), "search-workspace-configs-http")
	defer parentSpan.End()

	// retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchWorkspaceConfigs", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// attempt to load new username from body
	query, ok := s.loadValue(w, r, reqJson, "SearchWorkspaceConfigs", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// attempt to load video id from body
	skip, ok := s.loadValue(w, r, reqJson, "SearchWorkspaceConfigs", "skip", reflect.Float64, nil, false, userName, userId)
	if skip == nil || !ok {
		return
	}

	// attempt to load video id from body
	limit, ok := s.loadValue(w, r, reqJson, "SearchWorkspaceConfigs", "limit", reflect.Float64, nil, false, userName, userId)
	if limit == nil || !ok {
		return
	}

	// attempt to load parameter from body
	langsType := reflect.Float64
	languagesI, ok := s.loadValue(w, r, reqJson, "SearchWorkspaceConfigs", "languages", reflect.Slice, &langsType, true, userName, userId)
	if !ok {
		return
	}

	var languages []models.ProgrammingLanguage = nil
	if languagesI != nil {
		// create a slice to hold languages loaded from the http parameter
		tempLanguages := make([]models.ProgrammingLanguage, 0)

		// attempt to load parameter from body
		for _, lang := range languagesI.([]interface{}) {
			tempLanguages = append(tempLanguages, models.ProgrammingLanguage(lang.(float64)))
		}
		languages = tempLanguages
	}

	// attempt to load parameter from body
	tagsType := reflect.Map
	tagsI, ok := s.loadValue(w, r, reqJson, "SearchWorkspaceConfigs", "tags", reflect.Slice, &tagsType, true, userName, userId)
	if tagsI == !ok {
		return
	}

	var tags []int64 = nil
	if tagsI != nil {
		// create a slice to hold tags loaded from the http parameter
		tempTags := make([]int64, 0)

		// iterate through tagsI asserting each value as a map and create a new tag
		for _, tagI := range tagsI.([]interface{}) {
			tag := tagI.(map[string]interface{})

			// load id from tag map
			idI, ok := tag["_id"]
			if !ok {
				s.handleError(w, "missing tag id", r.URL.Path, "SearchWorkspaceConfigs", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					userName, userId, http.StatusUnprocessableEntity,
					"internal server error occurred", fmt.Errorf("missing tag id"))
				return
			}

			// assert id as string and parse into int64
			idS, ok := idI.(string)
			if !ok {
				s.handleError(w, fmt.Sprintf("invalid tag id type: %v", reflect.TypeOf(idS)), r.URL.Path,
					"SearchWorkspaceConfigs", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					userName, userId, http.StatusUnprocessableEntity,
					"invalid tag id type - should be string", nil)
				return
			}

			id, err := strconv.ParseInt(idS, 10, 64)
			if !ok {
				s.handleError(w, "failed to parse tag id as int64", r.URL.Path, "SearchWorkspaceConfigs", r.Method,
					r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r),
					userName, userId, http.StatusInternalServerError,
					"internal server error occurred", err)
				return
			}

			tags = append(tempTags, id)
		}
	}

	// check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchWorkspaceConfigs", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}
	// execute core function logic
	res, err := core.SearchWorkspaceConfigs(ctx, s.tiDB, s.meili, query.(string), languages, tags, int(skip.(float64)), int(limit.(float64)))
	if err != nil {
		// select error message dependent on if there was one returned from the function
		responseMessage := selectErrorResponse("internal server error occurred", res)
		// handle error internally
		s.handleError(w, "SearchWorkspaceConfigs core failed", r.URL.Path, "SearchWorkspaceConfigs", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, responseMessage, err)
		// exit
		return
	}

	parentSpan.AddEvent(
		"search-workspace-configs",
		trace.WithAttributes(
			attribute.Bool("success", true),
			attribute.String("ip", network.GetRequestIP(r)),
			attribute.String("username", callingUser.(*models.User).UserName),
		),
	)

	// return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchWorkspaceConfigs", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SimpleSearchPosts(w http.ResponseWriter, r *http.Request) {
	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// Create variables to hold user data defaulting to anonymous user
	userName := "anon"
	userId := ""
	userIdInt := int64(-1)
	if callingUser != nil {
		userName = callingUser.(*models.User).UserName
		userId = fmt.Sprintf("%d", callingUser.(*models.User).ID)
		userIdInt = callingUser.(*models.User).ID
	}

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchPosts", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// Attempt to load query from body
	query, ok := s.loadValue(w, r, reqJson, "SearchPosts", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// Check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// Return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.SimpleSearchPosts(s.meili, query.(string))
	if err != nil {
		// Handle error internally
		s.handleError(w, "SearchPosts core failed", r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchPosts", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchFriends(w http.ResponseWriter, r *http.Request) {
	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// abort if the user is not logged in
	if callingUser == nil {
		s.handleError(w, "not logged in", r.URL.Path, "SearchFriends", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "not logged in", "0", http.StatusUnauthorized, "You must be logged in to access the GIGO system.", nil)
		return
	}

	// Create variables to hold user data
	userName := callingUser.(*models.User).UserName
	userId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	userIdInt := callingUser.(*models.User).ID

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchFriends", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// Attempt to load query from body
	query, ok := s.loadValue(w, r, reqJson, "SearchFriends", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// Check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// Return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchFriends", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.SearchFriends(r.Context(), s.tiDB, callingUser.(*models.User), query.(string))
	if err != nil {
		// Handle error internally
		s.handleError(w, "SearchFriends core failed", r.URL.Path, "SearchFriends", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchFriends", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}

func (s *HTTPServer) SearchChatUsers(w http.ResponseWriter, r *http.Request) {
	// Retrieve calling user from context
	callingUser := r.Context().Value(CtxKeyUser)

	// abort if the user is not logged in
	if callingUser == nil {
		s.handleError(w, "not logged in", r.URL.Path, "SearchChatUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), "not logged in", "0", http.StatusUnauthorized, "You must be logged in to access the GIGO system.", nil)
		return
	}

	// Create variables to hold user data
	userName := callingUser.(*models.User).UserName
	userId := fmt.Sprintf("%d", callingUser.(*models.User).ID)
	userIdInt := callingUser.(*models.User).ID

	// Attempt to load JSON from request body
	reqJson := s.jsonRequest(w, r, "SearchChatUsers", false, userName, userIdInt)
	if reqJson == nil {
		return
	}

	// Attempt to load query from body
	query, ok := s.loadValue(w, r, reqJson, "SearchChatUsers", "query", reflect.String, nil, false, userName, userId)
	if query == nil || !ok {
		return
	}

	// Attempt to load chat id from body
	chatIdString, ok := s.loadValue(w, r, reqJson, "SearchChatUsers", "chat_id", reflect.String, nil, false, userName, userId)
	if chatIdString == nil || !ok {
		return
	}
	chatId, err := strconv.ParseInt(chatIdString.(string), 10, 64)
	if err != nil {
		s.handleError(w, "failed to parse chat id", r.URL.Path, "SearchChatUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		return
	}

	// Check if this is a test
	if val, ok := reqJson["test"]; ok && (val == true || val == "true") {
		// Return success for test
		s.jsonResponse(r, w, map[string]interface{}{}, r.URL.Path, "SearchChatUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
		return
	}

	// Execute core function logic
	res, err := core.SearchChatUsers(r.Context(), s.tiDB, callingUser.(*models.User), chatId, query.(string))
	if err != nil {
		// Handle error internally
		s.handleError(w, "SearchChatUsers core failed", r.URL.Path, "SearchChatUsers", r.Method, r.Context().Value(CtxKeyRequestID),
			network.GetRequestIP(r), userName, userId, http.StatusInternalServerError, "internal server error occurred", err)
		// Exit
		return
	}

	// Return response
	s.jsonResponse(r, w, res, r.URL.Path, "SearchChatUsers", r.Method, r.Context().Value(CtxKeyRequestID), network.GetRequestIP(r), userName, userId, http.StatusOK)
}
