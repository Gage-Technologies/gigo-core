package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go.opentelemetry.io/otel"
	"strconv"
	"time"

	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"github.com/gage-technologies/gigo-lib/mq"
	"github.com/kisielk/sqlstruct"
)

type NemesisRequest struct {
	AntagonistUsername  string    `json:"antagonist_username"`
	AntagonistRenown    string    `json:"antagonist_renown"`
	ProtagonistUsername string    `json:"protagonist_username"`
	TimeOfVillainy      time.Time `json:"time_of_villainy"`
	EndTime             time.Time `json:"end_time"`
}

type Battleground struct {
	AntagonistID          string `json:"antagonist_id"`
	AntagonistUsername    string `json:"antagonist_username"`
	AntagonistRenown      string `json:"antagonist_renown"`
	TotalAntagonistXpGain string `json:"antagonist_xp_gain"`
	//DailyAntagonistXpGain []*XPGain `json:"daily_antagonist_xp_gain"`

	ProtagonistID          string `json:"protagonist_id"`
	ProtagonistUsername    string `json:"protagonist_username"`
	ProtagonistRenown      string `json:"protagonist_renown"`
	TotalProtagonistXpGain string `json:"protagonist_xp_gain"`
	//DailyProtagonistXpGain []*XPGain `json:"daily_protagonist_xp_gain"`

	ProtagTowersTaken int64 `json:"protagonist_towers_taken"`
	AntagTowersTaken  int64 `json:"antagonist_towers_taken"`

	ProtagAvgXpGain string `json:"pro_avg"`
	AntagAvgXpGain  string `json:"ant_avg"`

	TimeOfVillainy time.Time  `json:"time_of_villainy"`
	EndTime        *time.Time `json:"end_time"`

	MatchID int64 `json:"match_id"`
}

type History struct {
	MatchID           string `json:"match_id"`
	TowersTakenAntag  string `json:"towers_taken_antag"`
	TowersTakenProtag string `json:"towers_taken_protag"`
	ProtagonistName   string `json:"protagonist_name"`
	ProtagXPGain      string `json:"protagonist_xp_gain"`
	AntagonistName    string `json:"antagonist_name"`
	AntagXPGain       string `json:"antagonist_xp_gain"`
	Victor            string `json:"victor"`
}

type XPGain struct {
	Username string    `json:"username"`
	Date     time.Time `json:"date"`
	Exp      int64     `json:"exp"`
}

type AllUsers struct {
	ID       string `json:"_id"`
	Username string `json:"username"`
}

func DeclareNemesis(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUserID int64, protagID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "declare-nemesis-core")
	defer span.End()
	callerName := "DeclareNemesis"

	// make sure a match is not already in progress
	check, err := db.QueryContext(ctx, &span, &callerName, "SELECT * FROM nemesis WHERE victor IS NULL AND (antagonist_id = ? OR protagonist_id = ?)", callingUserID, callingUserID)
	if err != nil {
		return map[string]interface{}{"declare_nemesis": "core failed"}, fmt.Errorf("declare nemesis core failed.  Error: %v", err)
	}

	// ensure no matches are found
	if check.Next() {
		return map[string]interface{}{"declare_nemesis": "you already have a nemesis in progress"}, nil
	}

	// close rows
	_ = check.Close()

	// find the username of the calling user, who will be the antagonist
	var antagUsername string
	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", callingUserID).Scan(&antagUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to query for antagonist id, err: %v", err.Error())
	}

	// find the username of the selected user, who will be the protagonist
	var protagUsername string
	err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id = ?", protagID).Scan(&protagUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to query for antagonist id, err: %v", err.Error())
	}

	// ensure the callinguser does not declare themselves as a nemesis
	if protagID == callingUserID {
		return nil, fmt.Errorf("protagonist id %d is the same as antagonist id %d", protagID, callingUserID)
	}

	if protagID == 0 {
		return nil, fmt.Errorf("failed to find protagonist id, username: %v", protagID)
	}

	// set the endtime to be a week from now
	endTime := time.Now().AddDate(0, 0, 7)

	nemesis := models.CreateNemesis(sf.Generate().Int64(), callingUserID, antagUsername, protagID, protagUsername, time.Now(), nil, false, &endTime, 0, 0)
	if nemesis == nil {
		return nil, errors.New("failed to create nemesis")
	}

	statements := nemesis.ToSQLNative()
	for _, statement := range statements {
		_, err := db.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return nil, fmt.Errorf("failed to execute statement for nemesis insertion, err: %v", err.Error())
		}
	}

	_, err = CreateNotification(ctx, db, js, sf, protagID, "Declare Nemesis", models.NemesisRequest, &callingUserID)
	if err != nil {
		return nil, fmt.Errorf("DeclareNemesis core. Failed to create notification: %v", err)
	}

	// return map[string]interface{}{"antagonist_renown": fmt.Sprintf("%d", antagRenown), "antagonist_username": antagUsername, "protagonist_username": protagUsername, "end_time": endTime, "time_of_villainy": nemesis.TimeOfVillainy}, nil
	return map[string]interface{}{"declare_nemesis": "request sent"}, nil
}

func AcceptNemesis(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUserID int64, antagonistID int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "accept-nemesis-core")
	defer span.End()
	callerName := "AcceptNemesis"

	// if callingUsername != nemesisReq.ProtagonistUsername {
	//	return fmt.Errorf("cannot accept nemesis request for %v, because it is not the protagonist", callingUsername)
	// }

	// res, err := db.DB.Query("select _id from users where username = ?", antagonistUsername)
	// if err != nil {
	//	return fmt.Errorf("failed to query for antagonist id, err: %v", err.Error())
	// }
	//
	// defer res.Close()
	//
	// var antagID int64
	// for res.Next() {
	//	err = res.Scan(&antagID)
	//	if err != nil {
	//		return fmt.Errorf("failed to scan for antagonist id, err: %v", err.Error())
	//	}
	// }

	endTime := time.Now().AddDate(0, 0, 7)

	_, err := db.ExecContext(ctx, &span, &callerName, "update nemesis set is_accepted = true, end_time = ? where antagonist_id = ? and protagonist_id = ? and victor is null", endTime, antagonistID, callingUserID)
	if err != nil {
		return fmt.Errorf("failed to accept nemesis, err: %v", err.Error())
	}

	var nemesisID int64

	err = db.QueryRowContext(ctx, &span, &callerName, "select _id from nemesis where antagonist_id =? and protagonist_id =? and victor is null", antagonistID, callingUserID).Scan(&nemesisID)
	if err != nil {
		return fmt.Errorf("failed to query for antagonist id, err: %v", err.Error())
	}

	nemesis := models.NemesisHistory{
		ID:                    sf.Generate().Int64(),
		MatchID:               nemesisID,
		AntagonistID:          antagonistID,
		ProtagonistID:         callingUserID,
		ProtagonistTowersHeld: 2,
		AntagonistTowersHeld:  2,
		ProtagonistTotalXP:    0,
		AntagonistTotalXP:     0,
		IsAlerted:             false,
		CreatedAt:             time.Now(),
	}

	statements := nemesis.ToSQLNative()
	for _, statement := range statements {
		_, err := db.ExecContext(ctx, &span, &callerName, statement.Statement, statement.Values...)
		if err != nil {
			return fmt.Errorf("failed to execute statement for nemesis insertion, err: %v", err.Error())
		}
	}

	_, err = CreateNotification(ctx, db, js, sf, antagonistID, "Accept Nemesis Request", models.NemesisRequest, &callingUserID)
	if err != nil {
		return fmt.Errorf("AcceptNemesis core. Failed to create notification: %v", err)
	}

	return nil
}

func DeclineNemesis(ctx context.Context, db *ti.Database, sf *snowflake.Node, js *mq.JetstreamClient, callingUserID int64, antagonistID int64) error {
	// if callingUsername != nemesisReq.ProtagonistUsername {
	//	return fmt.Errorf("cannot decline nemesis request for %v, you are not the protagonist", callingUsername)
	// }
	//
	// res, err := db.DB.Query("select _id from users where username =?", nemesisReq.AntagonistUsername)
	// if err != nil {
	//	return fmt.Errorf("failed to query for antagonist id, err")
	// }
	//
	// defer res.Close()
	//
	// var antagID int64
	// for res.Next() {
	//	err = res.Scan(&antagID)
	//	if err != nil {
	//		return fmt.Errorf("failed to scan for antagonist id, err")
	//	}
	// }

	ctx, span := otel.Tracer("gigo-core").Start(ctx, "decline-nemesis-core")
	defer span.End()
	callerName := "DeclineNemesis"

	_, err := db.ExecContext(ctx, &span, &callerName, "delete from nemesis where antagonist_id = ? and protagonist_id = ? and victor is null and is_accepted = false", antagonistID, callingUserID)
	if err != nil {
		return fmt.Errorf("failed to decline nemesis, err: %v", err.Error())
	}

	_, err = CreateNotification(ctx, db, js, sf, antagonistID, "Decline Nemesis Request", models.NemesisRequest, &callingUserID)
	if err != nil {
		return fmt.Errorf("DeclareNemesis core. Failed to create notification: %v", err)
	}

	return nil
}

func GetActiveNemesis(ctx context.Context, db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-active-nemesis-core")
	defer span.End()
	callerName := "GetActiveNemesis"

	res, err := db.QueryContext(ctx, &span, &callerName, "SELECT * FROM nemesis WHERE victor IS NULL AND (antagonist_id = ? OR protagonist_id = ?) AND is_accepted = true", callingUserID, callingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with user as protagonist, err: %v", err.Error())
	}

	defer res.Close()

	nemesis := make([]*models.NemesisFrontend, 0)

	defer res.Close()

	for res.Next() {
		var nemesisSQL models.Nemesis
		err = sqlstruct.Scan(&nemesisSQL, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Active Project Home core.    Error: %v", err)
		}

		nemesis = append(nemesis, nemesisSQL.ToFrontend())
	}

	return map[string]interface{}{"nemesis": nemesis}, nil
}

// func GetPendingNemesisRequests(db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
//	res, err := db.DB.Query("select * from nemesis where is_accepted = false and protagonist_id =?", callingUserID)
//	if err != nil {
//		return nil, fmt.Errorf("failed to query for nemesis with user as protagonist, err: %v", err.Error())
//	}
//
//	defer res.Close()
//
//	nemesis := make([]*models.NemesisFrontend, 0)
//
//	for res.Next() {
//		neme, err := models.NemesisFromSQLNative(res)
//		if err != nil {
//			return nil, fmt.Errorf("failed to parse nemesis, err: %v", err.Error())
//		}
//		nemesis = append(nemesis, neme.ToFrontend())
//	}
//
//	res, err = db.DB.Query("select * from nemesis where is_accepted = false and antagonist_id =?", callingUserID)
//	if err != nil {
//		return nil, fmt.Errorf("failed to query for nemesis with user as antagonist, err: %v", err.Error())
//	}
//
//	for res.Next() {
//		neme, err := models.NemesisFromSQLNative(res)
//		if err != nil {
//			return nil, fmt.Errorf("failed to parse nemesis, err: %v", err)
//		}
//		nemesis = append(nemesis, neme.ToFrontend())
//	}
//
//	return map[string]interface{}{"nemesis": nemesis}, nil
// }

func GetNemesisBattleground(ctx context.Context, db *ti.Database, matchID int64, antagonistID int64, protagonistID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-nemesis-battleground-core")
	defer span.End()
	calllerName := "GetNemesisBattleground"

	res, err := db.QueryContext(ctx, &span, &calllerName, "select * from nemesis where is_accepted = true and _id = ? and antagonist_id =? and protagonist_id =? and victor is null", matchID, antagonistID, protagonistID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", antagonistID, protagonistID, err)
	}

	defer res.Close()

	var nemesis models.Nemesis

	ok := res.Next()
	if !ok {
		return map[string]interface{}{"message": "No Nemesis History Found"}, err
	}

	// decode row results
	err = sqlstruct.Scan(&nemesis, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", antagonistID, protagonistID, err)
	}

	var totalAntagExp int64
	var totalProtagExp int64

	battle := Battleground{}

	res, err = db.QueryContext(ctx, &span, &calllerName, "select tier, user_name, _id from users where _id in (?, ?)", nemesis.AntagonistID, nemesis.ProtagonistID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for users, err: %v", err)
	}

	for res.Next() {
		var username string
		var renown int
		var id int64
		err = res.Scan(&renown, &username, &id)
		if err != nil {
			return nil, fmt.Errorf("failed to scan for username, err: %v", err)
		}

		if id == nemesis.AntagonistID {
			battle.AntagonistID = fmt.Sprintf("%d", id)
			battle.AntagonistUsername = username
			battle.AntagonistRenown = fmt.Sprintf("%d", renown)
		} else {
			battle.ProtagonistID = fmt.Sprintf("%d", id)
			battle.ProtagonistUsername = username
			battle.ProtagonistRenown = fmt.Sprintf("%d", renown)
		}
	}

	res, err = db.QueryContext(ctx, &span, &calllerName, "select protagonist_towers_held, antagonist_towers_held, protagonist_total_xp, antagonist_total_xp, created_at from nemesis_history where match_id = ?;", matchID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with match_id: %v, err: %v", matchID, err)
	}

	defer res.Close()

	antagXpGains := make([]*XPGain, 0)
	protagXpGains := make([]*XPGain, 0)
	var prevAntXp int64 = 0
	var prevProXp int64 = 0

	for res.Next() {
		var proTower int64
		var antTower int64
		var proXp int64
		var antXp int64
		var date time.Time
		err = res.Scan(&proTower, &antTower, &proXp, &antXp, &date)

		// Calculate XP changes
		antXpChange := antXp - prevAntXp
		proXpChange := proXp - prevProXp

		if err != nil {
			return nil, fmt.Errorf("failed to scan nemesis results into history, err: %v", err)
		}

		if antXpChange > 0 && prevAntXp != 0 {
			antagXpGains = append(antagXpGains, &XPGain{
				Date: date,
				Exp:  antXpChange,
			})
		}

		if proXpChange > 0 && prevProXp != 0 {
			protagXpGains = append(protagXpGains, &XPGain{
				Date: date,
				Exp:  proXpChange,
			})
		}

		prevAntXp = antXp
		prevProXp = proXp

		totalAntagExp = antXp
		totalProtagExp = proXp
	}

	duration := time.Since(nemesis.TimeOfVillainy)
	days := int64(duration.Hours() / 24)
	if days < 1 {
		days = 1
	}
	avgXpGainProtag := fmt.Sprintf("%d", totalProtagExp/days)
	avgXpGainAntag := fmt.Sprintf("%d", totalAntagExp/days)

	battle.TotalProtagonistXpGain = fmt.Sprintf("%d", totalProtagExp)
	battle.TotalAntagonistXpGain = fmt.Sprintf("%d", totalAntagExp)
	battle.EndTime = nemesis.EndTime
	battle.TimeOfVillainy = nemesis.TimeOfVillainy

	battle.AntagTowersTaken = int64(nemesis.AntagonistTowersCaptured)
	battle.ProtagTowersTaken = int64(nemesis.ProtagonistTowersCaptured)

	battle.ProtagAvgXpGain = avgXpGainProtag
	battle.AntagAvgXpGain = avgXpGainAntag
	battle.MatchID = nemesis.ID

	return map[string]interface{}{"battleground": battle}, nil

}

func RecentNemesisBattleground(ctx context.Context, db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "recent-nemesis-battleground-core")
	defer span.End()
	callerName := "RecentNemesisBattleground"

	res, err := db.QueryContext(ctx, &span, &callerName, "select * from nemesis where is_accepted = true and (antagonist_id = ? or protagonist_id = ?) order by time_of_villainy desc limit 1;", callingUserID, callingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", callingUserID, callingUserID, err)
	}

	defer res.Close()

	var nemesis models.Nemesis

	ok := res.Next()
	if !ok {
		return map[string]interface{}{"message": "No Nemesis History Found"}, err
	}

	// decode row results
	err = sqlstruct.Scan(&nemesis, res)
	if err != nil {
		return nil, fmt.Errorf("failed to decode query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", callingUserID, callingUserID, err)
	}

	var totalAntagExp int64
	var totalProtagExp int64

	battle := Battleground{}

	res, err = db.QueryContext(ctx, &span, &callerName, "select tier, user_name, _id from users where _id in (?, ?)", nemesis.AntagonistID, nemesis.ProtagonistID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for users, err: %v", err)
	}

	for res.Next() {
		var username string
		var renown int
		var id int64
		err = res.Scan(&renown, &username, &id)
		if err != nil {
			return nil, fmt.Errorf("failed to scan for username, err: %v", err)
		}

		if id == nemesis.AntagonistID {
			battle.AntagonistID = fmt.Sprintf("%d", id)
			battle.AntagonistUsername = username
			battle.AntagonistRenown = fmt.Sprintf("%d", renown)
		} else {
			battle.ProtagonistID = fmt.Sprintf("%d", id)
			battle.ProtagonistUsername = username
			battle.ProtagonistRenown = fmt.Sprintf("%d", renown)
		}
	}

	res, err = db.QueryContext(ctx, &span, &callerName, "select protagonist_towers_held, antagonist_towers_held, protagonist_total_xp, antagonist_total_xp, created_at from nemesis_history where match_id = ?;", nemesis.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", callingUserID, callingUserID, err)
	}

	defer res.Close()

	antagXpGains := make([]*XPGain, 0)
	protagXpGains := make([]*XPGain, 0)
	var prevAntXp int64 = 0
	var prevProXp int64 = 0

	for res.Next() {
		var proTower int64
		var antTower int64
		var proXp int64
		var antXp int64
		var date time.Time
		err = res.Scan(&proTower, &antTower, &proXp, &antXp, &date)

		// Calculate XP changes
		antXpChange := antXp - prevAntXp
		proXpChange := proXp - prevProXp

		if err != nil {
			return nil, fmt.Errorf("failed to scan nemesis results into history, err: %v", err)
		}

		if antXpChange > 0 && prevAntXp != 0 {
			antagXpGains = append(antagXpGains, &XPGain{
				Date: date,
				Exp:  antXpChange,
			})
		}

		if proXpChange > 0 && prevProXp != 0 {
			protagXpGains = append(protagXpGains, &XPGain{
				Date: date,
				Exp:  proXpChange,
			})
		}

		prevAntXp = antXp
		prevProXp = proXp

		totalAntagExp = antXp
		totalProtagExp = proXp
	}

	duration := time.Since(nemesis.TimeOfVillainy)
	days := int64(duration.Hours() / 24)
	if days < 1 {
		days = 1
	}
	avgXpGainProtag := fmt.Sprintf("%d", totalProtagExp/days)
	avgXpGainAntag := fmt.Sprintf("%d", totalAntagExp/days)

	battle.AntagTowersTaken = int64(nemesis.AntagonistTowersCaptured)
	battle.ProtagTowersTaken = int64(nemesis.ProtagonistTowersCaptured)
	battle.TotalProtagonistXpGain = fmt.Sprintf("%d", totalProtagExp)
	battle.TotalAntagonistXpGain = fmt.Sprintf("%d", totalAntagExp)
	battle.EndTime = nemesis.EndTime
	battle.TimeOfVillainy = nemesis.TimeOfVillainy

	battle.ProtagAvgXpGain = avgXpGainProtag
	battle.AntagAvgXpGain = avgXpGainAntag
	battle.MatchID = nemesis.ID

	victorUsername := ""

	if nemesis.Victor != nil {
		if *nemesis.Victor == nemesis.ProtagonistID {
			victorUsername = nemesis.ProtagonistName
		} else {
			victorUsername = nemesis.AntagonistName
		}
	}

	return map[string]interface{}{"battleground": battle, "victor": victorUsername}, nil

}

func WarHistory(ctx context.Context, db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "war-history-core")
	defer span.End()
	callerName := "WarHistory"
	res, err := db.QueryContext(ctx, &span, &callerName, "select _id, antagonist_name, protagonist_name, victor, protagonist_towers_captured, antagonist_towers_captured from nemesis where (antagonist_id = ? or protagonist_id = ?) and victor is not null;", callingUserID, callingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis with antagonist_id: %v and protagonist_id: %v, err: %v", callingUserID, callingUserID, err)
	}

	defer res.Close()

	history := make([]*History, 0)
	for res.Next() {
		var h History
		err = res.Scan(&h.MatchID, &h.AntagonistName, &h.ProtagonistName, &h.Victor, &h.TowersTakenProtag, &h.TowersTakenAntag)
		if err != nil {
			return nil, fmt.Errorf("failed to scan nemesis results into history, err: %v", err)
		}
		history = append(history, &h)
	}

	for i, h := range history {
		var proXP int64
		var antXP int64
		err = db.QueryRowContext(ctx, &span, &callerName, "select protagonist_total_xp, antagonist_total_xp from nemesis_history where match_id = ? order by created_at desc;", h.MatchID).Scan(&proXP, &antXP)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get xp totals: %v", err)
		}

		history[i].ProtagXPGain = strconv.FormatInt(proXP, 10)
		history[i].AntagXPGain = strconv.FormatInt(antXP, 10)

		var victor string
		err = db.QueryRowContext(ctx, &span, &callerName, "select user_name from users where _id =?", h.Victor).Scan(&victor)
		if err != nil && err != sql.ErrNoRows {
			return nil, fmt.Errorf("failed to get victor username: %v", err)
		}

		history[i].Victor = victor
	}

	return map[string]interface{}{"history": history}, nil
}

func PendingNemesis(ctx context.Context, db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "pending-nemesis-core")
	defer span.End()
	callerName := "PendingNemesis"

	res, err := db.QueryContext(ctx, &span, &callerName, "select * from nemesis where (antagonist_id =? or protagonist_id =?) and is_accepted = 0", callingUserID, callingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to query for pending nemesis requests, err: %v", err)
	}

	defer res.Close()

	nemesis := make([]*models.NemesisFrontend, 0)

	defer res.Close()

	for res.Next() {
		var nemesisSQL models.Nemesis
		err = sqlstruct.Scan(&nemesisSQL, res)
		if err != nil {
			return nil, fmt.Errorf("failed to decode query for resulsts. Pending Nemesis core.    Error: %v", err)
		}

		nemesis = append(nemesis, nemesisSQL.ToFrontend())
	}

	return map[string]interface{}{"pending": nemesis}, nil
}

func DeclareVictor(ctx context.Context, db *ti.Database, matchID int64, victor int64) error {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "declare-victor-core")
	defer span.End()
	callerName := "DeclareVictor"

	_, err := db.ExecContext(ctx, &span, &callerName, "update nemesis set victor = ?, end_time = ? where _id = ? and victor is null", victor, time.Now(), matchID)
	if err != nil {
		return fmt.Errorf("failed to update table with victor. Declaring Victor core.    Error: %v", err)
	}

	return nil
}

func GetAllUsers(ctx context.Context, db *ti.Database, callingUserID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "get-all-users-core")
	defer span.End()
	callerName := "GetAllUsersNemesis"

	tx, err := db.BeginTx(ctx, &span, &callerName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create insert tx: %v", err)
	}

	// defer closure of tx
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, &callerName, "select _id, user_name from users where _id != ?", callingUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %v", err)
	}

	defer rows.Close()

	allUsers := make([]*AllUsers, 0)
	for rows.Next() {
		user := &AllUsers{}
		err = rows.Scan(&user.ID, &user.Username)
		if err != nil {
			return nil, fmt.Errorf("failed to scan friend: %v", err)
		}

		allUsers = append(allUsers, user)
	}

	return map[string]interface{}{"all_users": allUsers}, nil
}

func GetDailyXPGain(ctx context.Context, db *ti.Database, matchID int64) (map[string]interface{}, error) {
	ctx, span := otel.Tracer("gigo-core").Start(ctx, "nemesis-get-daily-xp-gain-core")
	defer span.End()
	callerName := "NemesisDailyXP"

	var antagonistId int64
	var antagonistName string
	var protagonistId int64
	var protagonistName string
	var timeOfVillainy time.Time

	query := "select antagonist_id, antagonist_name, protagonist_id, protagonist_name, time_of_villainy from nemesis where _id = ?"
	err := db.QueryRowContext(ctx, &span, &callerName, query, matchID).Scan(&antagonistId, &antagonistName, &protagonistId, &protagonistName, &timeOfVillainy)
	if err != nil {
		return nil, fmt.Errorf("failed to query for nemesis _id: %v, err: %v", matchID, err)
	}

	ant, err := db.QueryContext(ctx, &span, &callerName, "select date, xp from xp_reasons where user_id = ? and date >= ?", antagonistId, timeOfVillainy)
	if err != nil {
		return nil, fmt.Errorf("failed to query for xp_reasons for nemesis where user_id: %v and time_of_villainy: %v, err: %v", antagonistId, timeOfVillainy, err)
	}
	defer ant.Close()

	xpGainAnt := make([]*XPGain, 0)

	for ant.Next() {
		var date time.Time
		var xp int64

		err = ant.Scan(&date, &xp)

		xpGainAnt = append(xpGainAnt, &XPGain{
			Username: antagonistName,
			Date:     date,
			Exp:      xp,
		})
	}

	pro, err := db.QueryContext(ctx, &span, &callerName, "select date, xp from xp_reasons where user_id = ? and date >= ?", protagonistId, timeOfVillainy)
	if err != nil {
		return nil, fmt.Errorf("failed to query for xp_reasons for nemesis where user_id: %v and time_of_villainy: %v, err: %v", antagonistId, timeOfVillainy, err)
	}
	defer pro.Close()

	xpGainPro := make([]*XPGain, 0)

	for pro.Next() {
		var date time.Time
		var xp int64

		err = pro.Scan(&date, &xp)

		xpGainPro = append(xpGainPro, &XPGain{
			Username: protagonistName,
			Date:     date,
			Exp:      xp,
		})
	}

	return map[string]interface{}{"daily_antagonist": xpGainAnt, "daily_protagonist": xpGainPro}, nil
}
