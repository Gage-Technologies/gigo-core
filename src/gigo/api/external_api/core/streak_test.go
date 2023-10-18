package core

//func TestCheckElapsedStreakTime(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	testSnowflakeNode, err := snowflake.NewNode(1)
//	if err != nil {
//		log.Panicf("Error: Init() : %v", err)
//	}
//
//	testUser, err := models.CreateUser(testSnowflakeNode.Generate().Int64(), "sixtyNoine", "bussin",
//		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
//		nil, nil, "Meta", "test",
//		testSnowflakeNode.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, 0)
//	if err != nil {
//		t.Error("\nInitialize testUser failed\n    Error: ", err)
//		return
//	}
//
//	statements, err := testUser.ToSQLNative()
//	if err != nil {
//		t.Error("\nInitialize testUser failed\n    Error: ", err)
//		return
//	}
//
//	for _, statement := range statements {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
//			return
//		}
//	}
//
//	// load user timezone
//	timeLocation, err := time.LoadLocation(testUser.Timezone)
//	if err != nil {
//		t.Error("\nConvert user timezone failed\n    Error: ", err)
//		return
//	}
//
//	// calculate the beginning of the day for the user
//	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)
//
//	yesterdayTime := beginningOfDayUserTz.Add(-24 * time.Hour)
//
//	fmt.Println("test date: ", yesterdayTime)
//
//	dailyUsage := new(models.DailyUsage)
//	dailyUsage.StartTime = time.Now().In(timeLocation).Add(-24 * time.Hour)
//	fmt.Println("start date for interval: ", dailyUsage.StartTime.UTC())
//	dailyUsage.EndTime = nil
//	dailyUsage.OpenSession = 0
//
//	// create empty user stats model
//	userStatsModel1, err := models.CreateUserStats(testSnowflakeNode.Generate().Int64(), testUser.ID, 0,
//		true, 3, 3, time.Minute*0, time.Minute*0, 3,
//		3, beginningOfDayUserTz, []*models.DailyUsage{dailyUsage})
//	if err != nil {
//		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
//	}
//
//	if userStatsModel1 == nil {
//		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
//	}
//
//	// insert empty model
//	statements = userStatsModel1.ToSQLNative()
//	for _, statement := range statements {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("Failed to initialize first user stats: %v\n", err)
//		}
//	}
//
//	// create empty user stats model
//	userStatsModel2, err := models.CreateUserStats(testSnowflakeNode.Generate().Int64(), testUser.ID, 0,
//		true, 2, 2, time.Minute*0, time.Minute*0, 3,
//		2, yesterdayTime, []*models.DailyUsage{dailyUsage})
//	if err != nil {
//		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
//	}
//
//	if userStatsModel2 == nil {
//		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
//	}
//
//	// insert empty model
//	statements = userStatsModel2.ToSQLNative()
//	for _, statement := range statements {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("Failed to initialize first user stats: %v\n", err)
//		}
//	}
//
//	// create empty user stats model
//	userStatsModel3, err := models.CreateUserStats(testSnowflakeNode.Generate().Int64(), testUser.ID, 0,
//		true, 1, 1, time.Minute*0, time.Minute*0, 1,
//		1, yesterdayTime.Add(-24*time.Hour), []*models.DailyUsage{dailyUsage})
//	if err != nil {
//		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
//	}
//
//	if userStatsModel3 == nil {
//		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
//	}
//
//	// insert empty model
//	statements = userStatsModel2.ToSQLNative()
//	for _, statement := range statements {
//		_, err = testTiDB.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("Failed to initialize first user stats: %v\n", err)
//		}
//	}
//
//	isStreakActivated, currentStreak, weekInReview, elapsedTimeToday, err := CheckElapsedStreakTime(testTiDB, testUser.ID, testUser.Timezone)
//	if err != nil {
//		t.Error("\nCheckElapsedStreakTime failed\n    Error: ", err)
//		return
//	}
//
//	if elapsedTimeToday == nil {
//		t.Error("\nCheckElapsedStreakTime failed\n    Error: elapsedTimeToday is nil")
//		return
//	}
//
//	if isStreakActivated != true {
//		t.Error("\nCheckElapsedStreakTime failed\n    Error: streak is not activated")
//		return
//	}
//
//	if currentStreak != 3 {
//		t.Error("\nCheckElapsedStreakTime failed\n    Error: current streak is not 3")
//		return
//	}
//
//	for k, v := range weekInReview {
//		fmt.Printf("day: %v  - %v\n", k, v)
//	}
//
//}
