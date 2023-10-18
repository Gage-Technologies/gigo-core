package external_api

// Selects the error response message by checking if a response message was passed in the response map
// If no response is passed in the map then the default message is returned
//
//	     defaultMessage    - string, message that will be returned if there is no alternative in responseMessage
//			responseMessage   - map[string]interface{}, response object returned from the core function executed
//
// Returns:
//
//	out               - string, response message to be used in HTTP error response
func selectErrorResponse(defaultMessage string, responseMessage map[string]interface{}) string {
	// return default message if response message is nil
	if responseMessage == nil {
		return defaultMessage
	}

	// attempt to read message from response
	if mes, ok := responseMessage["message"]; ok {
		// attempt to cast response message to string
		message, ok := mes.(string)
		// return response message if cast was successful
		if ok {
			return message
		}
	}

	return defaultMessage
}

// DetermineRoutePermission
//
//	Helper function to check a route against public route permissions.
//
//	Args:
//	    route: string, the route to check against
//	Returns:
//	    RoutePermission, permission status of the passed route
func DetermineRoutePermission(route string) RoutePermission {
	// iterate over the public routes checking if this is public
	for _, publicRoute := range publicRoutes {
		if publicRoute.MatchString(route) {
			return RoutePermissionPublic
		}
	}

	// iterate over the hybrid routes checking if this is hybrid
	for _, hybridRoute := range hybridRoutes {
		if hybridRoute.MatchString(route) {
			return RoutePermissionHybrid
		}
	}

	// default to private
	return RoutePermissionPrivate
}
