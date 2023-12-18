package streak

import (
	"context"
	"fmt"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/logging"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/now"
	"log"
	"os"
	"testing"
	"time"
)

func TestCreateNewEngine(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
	if streakEngine == nil {
		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
		return
	}

	t.Logf("TestCreateEngine Succeeded\n")
}

func TestStreakEngine_GetUsersYesterdayStats(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
	if streakEngine == nil {
		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
		return
	}

	start := time.Now()

	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "we bussin",
		nil, nil, "Meta", "test",
		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0), nil)
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	statements, err := testUser.ToSQLNative()
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
			return
		}
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(testUser.Timezone)
	if err != nil {
		t.Error("\nConvert user timezone failed\n    Error: ", err)
		return
	}

	// calculate the beginning of the day for the user
	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)
	yesterdayTime := beginningOfDayUserTz.Add(-24 * time.Hour)

	fmt.Println("test date: ", yesterdayTime)

	dailyUsage := new(models.DailyUsage)
	dailyUsage.StartTime = time.Now().In(timeLocation).Add(-24 * time.Hour)
	fmt.Println("start date for interval: ", dailyUsage.StartTime.UTC())
	dailyUsage.EndTime = nil
	dailyUsage.OpenSession = 0

	// create empty user stats model
	userStatsModel, err := models.CreateUserStats(
		streakEngine.snowFlake.Generate().Int64(),
		testUser.ID,
		0,
		false,
		0,
		0,
		time.Minute*0,
		time.Minute*0,
		0,
		0,
		0,
		yesterdayTime,
		time.Now().In(timeLocation).Add(20*time.Minute),
		[]*models.DailyUsage{dailyUsage},
	)
	if err != nil {
		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
	}

	if userStatsModel == nil {
		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
	}

	// insert empty model
	statements = userStatsModel.ToSQLNative()
	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("Failed to initialize first user stats: %v\n", err)
		}
	}

	// create empty user stats model
	userStatsModel2, err := models.CreateUserStats(streakEngine.snowFlake.Generate().Int64(), testUser.ID, 0,
		false, 0, 0, time.Minute*0, time.Minute*0, 0,
		0, 0, yesterdayTime.Add(-24*time.Hour), time.Now().In(timeLocation).Add(20*time.Minute), []*models.DailyUsage{dailyUsage})
	if err != nil {
		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
	}

	if userStatsModel2 == nil {
		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
	}

	// insert empty model
	statements = userStatsModel2.ToSQLNative()
	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("Failed to initialize first user stats: %v\n", err)
		}
	}

	yesterday, err := streakEngine.GetUsersLastStatsDay(context.Background(), testUser.ID)
	if err != nil {
		t.Error("\nUserStartWorkspace failed\n    Error: ", err)
		return
	}

	fmt.Println("execution time: ", time.Since(start))

	fmt.Println("yesterday: ", yesterday)
	fmt.Println("yesterday user stats: ", yesterday.DailyIntervals[0])

	if yesterday.Date != yesterdayTime.UTC() {
		t.Errorf("\nUserStartWorkspace failed\n    Date: %v\n    Expected: %v\n", yesterday.Date, yesterdayTime)
	}

	t.Logf("UserStartWorkspace Succeeded")
}

func TestEngine_initializeFirstUserStats(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
	if streakEngine == nil {
		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
		return
	}

	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
		nil, nil, "Meta", "test",
		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0), nil)
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	statements, err := testUser.ToSQLNative()
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
			return
		}
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(testUser.Timezone)
	if err != nil {
		t.Error("\nConvert user timezone failed\n    Error: ", err)
		return
	}

	// calculate the beginning of the day for the user
	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)

	err = streakEngine.InitializeFirstUserStats(context.Background(), nil, testUser.ID, beginningOfDayUserTz)
	if err != nil {
		t.Error("\nInitializeFirstUserStats failed\n    Error: ", err)
		return
	}

	t.Logf("InitializeFirstUserStats Succeeded")
}

func TestEngine_userStartWorkspace(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
	if streakEngine == nil {
		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
		return
	}

	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
		nil, nil, "Meta", "test",
		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0), nil)
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	statements, err := testUser.ToSQLNative()
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
			return
		}
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(testUser.Timezone)
	if err != nil {
		t.Error("\nConvert user timezone failed\n    Error: ", err)
		return
	}

	// calculate the beginning of the day for the user
	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)

	dailyUsage := new(models.DailyUsage)
	dailyUsage.StartTime = time.Now().In(timeLocation)
	dailyUsage.EndTime = nil
	dailyUsage.OpenSession = 0

	// create empty user stats model
	userStatsModel, err := models.CreateUserStats(streakEngine.snowFlake.Generate().Int64(), testUser.ID, 0,
		false, 0, 0, time.Minute*0, time.Minute*0, 0,
		0, 0, beginningOfDayUserTz, time.Now().In(timeLocation).Add(20*time.Minute), []*models.DailyUsage{dailyUsage})
	if err != nil {
		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
	}

	if userStatsModel == nil {
		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
	}

	// insert empty model
	statements = userStatsModel.ToSQLNative()
	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("Failed to initialize first user stats: %v\n", err)
		}
	}

	err = streakEngine.UserStartWorkspace(context.Background(), testUser.ID)
	if err != nil {
		t.Error("\nUserStartWorkspace failed\n    Error: ", err)
		return
	}

	t.Logf("UserStartWorkspace Succeeded")
}

func TestEngine_userStopWorkspace(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
		"gigo-dev",
		"gigo_test_db")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	// create redis connection
	testRdb := redis.NewClient(&redis.Options{
		Addr:     "gigo-dev-redis:6379",
		Password: "gigo-dev",
		DB:       7,
	})

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	defer os.Remove("test.log")
	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
	if streakEngine == nil {
		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
		return
	}

	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
		nil, nil, "Meta", "test",
		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0), nil)
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	statements, err := testUser.ToSQLNative()
	if err != nil {
		t.Error("\nInitialize testUser failed\n    Error: ", err)
		return
	}

	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
			return
		}
	}

	// load user timezone
	timeLocation, err := time.LoadLocation(testUser.Timezone)
	if err != nil {
		t.Error("\nConvert user timezone failed\n    Error: ", err)
		return
	}

	// calculate the beginning of the day for the user
	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)

	dailyUsage := new(models.DailyUsage)
	dailyUsage.StartTime = time.Now().In(timeLocation)
	dailyUsage.EndTime = nil
	dailyUsage.OpenSession = 0

	// create empty user stats model
	userStatsModel, err := models.CreateUserStats(streakEngine.snowFlake.Generate().Int64(), testUser.ID, 0,
		false, 0, 0, time.Minute*0, time.Minute*0, 0,
		0, 0, beginningOfDayUserTz, beginningOfDayUserTz.Add(time.Hour*24),
		[]*models.DailyUsage{dailyUsage})
	if err != nil {
		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
	}

	if userStatsModel == nil {
		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
	}

	// insert empty model
	statements = userStatsModel.ToSQLNative()
	for _, statement := range statements {
		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
		if err != nil {
			t.Errorf("Failed to initialize first user stats: %v\n", err)
		}
	}

	err = streakEngine.UserStopWorkspace(context.Background(), testUser.ID)
	if err != nil {
		t.Error("\nUserStopWorkspace failed\n    Error: ", err)
		return
	}

	t.Logf("UserStopWorkspace Succeeded")
}

func TestEngine_getUsersYesterdayStats(t *testing.T) {}

// func TestEngine_checkNewDay(t *testing.T) {
// 	if err != nil {
// 		t.Error("\nInitialize testTiDB failed\n    Error: ", err)
// 		return
// 	}
//
//// create redis connection
//testRdb := redis.NewClient(&redis.Options{
//Addr:     "gigo-dev-redis:6379",
//Password: "gigo-dev",
//DB:       7,
//})
//
// 	testSnowflakeNode, err := snowflake.NewNode(1)
// 	if err != nil {
// 		log.Panicf("Error: Init() : %v", err)
// 	}
//
// 	defer os.Remove("test.log")
// 	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
// 	if err != nil {
// 		log.Panicf("Error: Init() : %v", err)
// 	}
//
// 	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
// 	if streakEngine == nil {
// 		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
// 		return
// 	}
//
// 	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
// 		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
// 		nil, nil, "Meta", "test",
// 		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0))
// 	if err != nil {
// 		t.Error("\nInitialize testUser failed\n    Error: ", err)
// 		return
// 	}
//
// 	statements, err := testUser.ToSQLNative()
// 	if err != nil {
// 		t.Error("\nInitialize testUser failed\n    Error: ", err)
// 		return
// 	}
//
// 	for _, statement := range statements {
// 		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
// 		if err != nil {
// 			t.Error("\nInitialize testUser exec failed\n    Error: ", err)
// 			return
// 		}
// 	}
//
// 	// load user timezone
// 	timeLocation, err := time.LoadLocation(testUser.Timezone)
// 	if err != nil {
// 		t.Error("\nConvert user timezone failed\n    Error: ", err)
// 		return
// 	}
//
// 	// calculate the beginning of the day for the user
// 	beginningOfDayUserTz := now.BeginningOfDay().In(timeLocation)
// 	beginningOfYesterdayUserTz := beginningOfDayUserTz.Add(time.Hour * -24)
//
// 	// create empty user stats model
// 	userStatsModel, err := models.CreateUserStats(streakEngine.snowFlake.Generate().Int64(), testUser.ID, 0,
// 		false, 0, 0, time.Minute*0, time.Minute*0, 0,
// 		0, 0, beginningOfYesterdayUserTz, nil)
// 	if err != nil {
// 		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
// 	}
//
// 	if userStatsModel == nil {
// 		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
// 	}
//
// 	//// insert empty model
// 	//statements = userStatsModel.ToSQLNative()
// 	//for _, statement := range statements {
// 	//	_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
// 	//	if err != nil {
// 	//		t.Errorf("Failed to initialize first user stats: %v\n", err)
// 	//	}
// 	//}
//
// 	start := time.Now()
//
// 	newDay, err := streakEngine.CheckNewDay(testUser.ID, testUser.Timezone)
// 	if err != nil {
// 		t.Error("\nCheckNewDay failed\n    Error: ", err)
// 		return
// 	}
//
// 	fmt.Printf("\nExecution Time: %v\n", time.Since(start))
//
// 	if !newDay {
// 		t.Error("\nCheckNewDay failed\n    Error: incorrect value returned from CheckNewDay")
// 		return
// 	}
//
// 	t.Logf("CheckNewDay Succeeded")
//
// }

//func TestEngine_checkStreak(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev",
//		"gigo-dev",
//		"gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	// create redis connection
//	testRdb := redis.NewClient(&redis.Options{
//		Addr:     "gigo-dev-redis:6379",
//		Password: "gigo-dev",
//		DB:       7,
//	})
//
//	testSnowflakeNode, err := snowflake.NewNode(1)
//	if err != nil {
//		log.Panicf("Error: Init() : %v", err)
//	}
//
//	defer os.Remove("test.log")
//	testLogger, err := logging.CreateBasicLogger(logging.NewDefaultBasicLoggerOptions("/tmp/gigo-http-api-test.log"))
//	if err != nil {
//		log.Panicf("Error: Init() : %v", err)
//	}
//
//	streakEngine := NewStreakEngine(testTiDB, testRdb, testSnowflakeNode, testLogger)
//	if streakEngine == nil {
//		t.Error("\nInitialize streakEngine failed\n    Error: ", err)
//		return
//	}
//
//	testUser, err := models.CreateUser(streakEngine.snowFlake.Generate().Int64(), "sixtyNoine", "bussin",
//		"sixtyNoine@gmail.com", "420694204200", models.UserStatusBasic, "bussin",
//		nil, nil, "Meta", "test",
//		streakEngine.snowFlake.Generate().Int64(), "", models.UserStart{}, "US/Central", models.AvatarSettings{}, uint64(0))
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
//		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
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
//	beginningOfYesterdayUserTz := beginningOfDayUserTz.Add(time.Hour * -24)
//
//	// create empty user stats model
//	userStatsModel, err := models.CreateUserStats(streakEngine.snowFlake.Generate().Int64(), testUser.ID, 0,
//		false, 0, 0, time.Minute*0, time.Minute*0, 0,
//		0, 0, beginningOfYesterdayUserTz, time.Now().In(timeLocation).Add(20*time.Minute), nil)
//	if err != nil {
//		t.Errorf("Failed to initialize first user stats: %v\n", err.Error())
//	}
//
//	if userStatsModel == nil {
//		t.Errorf("Failed to initialize first user stats: user stats model is nil\n")
//	}
//
//	// insert empty model
//	statements = userStatsModel.ToSQLNative()
//	for _, statement := range statements {
//		_, err = streakEngine.db.DB.Exec(statement.Statement, statement.Values...)
//		if err != nil {
//			t.Errorf("Failed to initialize first user stats: %v\n", err)
//		}
//	}
//
//	newDay, err := streakEngine.CheckStreak(context.Background(), testUser.ID, testUser.Timezone)
//	if err != nil {
//		t.Error("\nCheckStreak failed\n    Error: ", err)
//		return
//	}
//
//	if newDay {
//		t.Error("\nCheckStreak failed\n    Error: incorrect value returned from CheckStreak")
//		return
//	}
//
//	t.Logf("CheckStreak Succeeded")
//
//}
