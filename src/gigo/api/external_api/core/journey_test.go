package core

import (
	"context"
	"github.com/bwmarrin/snowflake"
	ti "github.com/gage-technologies/gigo-lib/db"
	"github.com/gage-technologies/gigo-lib/db/models"
	"log"
	"testing"
)

func TestSaveJourneyInfo(t *testing.T) {
	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql", "gigo-dev", "gigo-dev", "gigo_dev_test")
	if err != nil {
		t.Fatal("Initialize test database failed:", err)
	}

	testSnowflakeNode, err := snowflake.NewNode(1)
	if err != nil {
		log.Panicf("Error: Init() : %v", err)
	}

	// Create a user
	user, err := models.CreateUser(420, "test", "", "", "", models.UserStatusPremium, "", nil, nil, "", "", 0, "None", models.UserStart{}, "America/Chicago", models.AvatarSettings{}, 0)
	if err != nil {
		t.Errorf("\nTestSaveJourneyInfo failed\n    Error: %v\n", err)
		return
	}

	journeyInfo := models.JourneyInfo{
		ID:               1,
		UserID:           69420,
		LearningGoal:     "Hobby",
		SelectedLanguage: 5,
		EndGoal:          "Fullstack",
		ExperienceLevel:  "Intermediate",
		FamiliarityIDE:   "Some Experience",
		FamiliarityLinux: "Some Experience",
		Tried:            "Tried",
		TriedOnline:      "Tried",
		AptitudeLevel:    "Intermediate",
	}

	saveJourney, err := SaveJourneyInfo(context.Background(), testTiDB, testSnowflakeNode, user, journeyInfo)
	if err != nil {
		t.Errorf("SaveJourneyInfo() error = %v", err)
		return
	}

	expectedMessage := "journey info saved"
	if msg, ok := saveJourney["message"].(string); ok {
		if msg != expectedMessage {
			t.Errorf("SaveJourneyInfo() = %v, want %v", msg, expectedMessage)
		}
	} else {
		t.Errorf("SaveJourneyInfo() did not return a message")
	}

}
