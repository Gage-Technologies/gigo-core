package utils

//func InitializeDatabases(db *ti.Database, logger logging.Logger) error {
//	logger.Debug("initializing attempt table")
//	err := models.InitializeAttemptTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize attempt table: %v", err)
//	}
//	logger.Debug("attempt table initialized")
//
//	logger.Debug("initializing award table")
//	err = models.InitializeAwardTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize award table: %v", err)
//	}
//	logger.Debug("award table initialized")
//
//	logger.Debug("initializing coffee table")
//	err = models.InitializeCoffeeTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize coffee table: %v", err)
//	}
//	logger.Debug("coffee table initialized")
//
//	logger.Debug("initializing award table")
//	err = models.InitializeAwardTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize award table: %v", err)
//	}
//	logger.Debug("award table initialized")
//
//	logger.Debug("initializing broadcast_event table")
//	err = models.InitializeBroadcastEventTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize broadcast_event table: %v", err)
//	}
//	logger.Debug("broadcast_event table initialized")
//
//	logger.Debug("initializing coffee table")
//	err = models.InitializeCoffeeTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize coffee table: %v", err)
//	}
//	logger.Debug("coffee table initialized")
//
//	logger.Debug("initializing comment table")
//	err = models.InitializeCommentTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize comment table: %v", err)
//	}
//	logger.Debug("comment table initialized")
//
//	logger.Debug("initializing discussion table")
//	err = models.InitializeDiscussionTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize discussion table: %v", err)
//	}
//	logger.Debug("discussion table initialized")
//
//	logger.Debug("initializing post table")
//	err = models.InitializePostTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize post table: %v", err)
//	}
//	logger.Debug("post table initialized")
//
//	logger.Debug("initializing recommended_posts table")
//	err = models.InitializeRecommendedPostTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize recc posts table: %v", err)
//	}
//	logger.Debug("recc posts table initialized")
//
//	logger.Debug("initializing thread comment table")
//	err = models.InitializeThreadCommentTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize thread comment table: %v", err)
//	}
//	logger.Debug("thread comment table initialized")
//
//	logger.Debug("initializing user table")
//	err = models.InitializeUserTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize user table: %v", err)
//	}
//	logger.Debug("user table initialized")
//
//	logger.Debug("initializing user stats table")
//	err = models.InitializeUserStatsTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize user stats table: %v", err)
//	}
//	logger.Debug("user stats table initialized")
//
//	logger.Debug("initializing workspace table")
//	err = models.InitializeWorkspaceTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize workspace table: %v", err)
//	}
//	logger.Debug("workspace table initialized")
//
//	logger.Debug("initializing workspace template table")
//	err = models.InitializeWorkspaceTemplateTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize workspace template table: %v", err)
//	}
//	logger.Debug("workspace template table initialized")
//
//	logger.Debug("initializing follower table")
//	err = models.InitializeFollowerTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize follower table: %v", err)
//	}
//	logger.Debug("follower table initialized")
//
//	logger.Debug("initializing user free premium")
//	err = models.InitializeUserFreePremiumSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize follower table: %v", err)
//	}
//	logger.Debug("user free premium table initialized")
//
//	logger.Debug("initializing tag table")
//	err = models.InitializeTagTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize tag table: %v", err)
//	}
//	logger.Debug("tag table initialized")
//
//	logger.Debug("initializing workspace config table")
//	err = models.InitializeWorkspaceConfigTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize workspace config table: %v", err)
//	}
//	logger.Debug("workspace config table initialized")
//
//	logger.Debug("initializing user session key table")
//	err = models.InitializeUserSessionKeyTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize user session key table: %v", err)
//	}
//	logger.Debug("user session key table initialized")
//
//	logger.Debug("initializing workspace agent table")
//	err = models.InitializeWorkspaceAgentTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize workspace agent table: %v", err)
//	}
//	logger.Debug("workspace agent table initialized")
//
//	logger.Debug("initializing workspace agent stats table")
//	err = models.InitializeWorkspaceAgentStatsTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize workspace agent stats table: %v", err)
//	}
//	logger.Debug("workspace agent stats table initialized")
//
//	logger.Debug("initializing streak xp expiration table")
//	err = models.InitializeStatsXPSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize streak xp expiration table: %v", err)
//	}
//	logger.Debug("streak xp expiration table initialized")
//
//	logger.Debug("initializing nemesis table")
//	err = models.InitializeNemesisTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize nemesis table: %v", err)
//	}
//	logger.Debug("streak nemesis initialized")
//
//	logger.Debug("initializing xp boost table")
//	err = models.InitializeXPBoostTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize xp boost table: %v", err)
//	}
//	logger.Debug("xp boost initialized")
//
//	logger.Debug("initializing rewards table")
//	err = models.InitializeRewardsTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize rewards table: %v", err)
//	}
//	logger.Debug("rewards initialized")
//
//	logger.Debug("initializing implicit rec table")
//	err = models.InitializeImplicitRecTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize implicit rec table: %v", err)
//	}
//	logger.Debug("implicit rec initialized")
//
//	logger.Debug("initializing search rec table")
//	err = models.InitializeSearchRecTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize search rec table: %v", err)
//	}
//	logger.Debug("search rec initialized")
//
//	err = models.InitializeFriendsTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize friends table: %v", err)
//	}
//	logger.Debug("friends initialized")
//
//	err = models.InitializeFriendRequestsTableSQL(db)
//	if err != nil {
//		return fmt.Errorf("failed to initialize friend requests table: %v", err)
//	}
//	logger.Debug("friend requests initialized")
//
//	return nil
//}
