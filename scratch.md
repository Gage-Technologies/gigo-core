# Scratch

		err = SendWasReferredMessage(ctx, mgKey, mgDomain, newUser.Email, userQuery.UserName)
		if err != nil {
			logger.Errorf("SendReferredFriendMessage failed: %v", err)
		}