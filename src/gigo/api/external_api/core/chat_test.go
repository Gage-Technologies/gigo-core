package core

//func TestCreateChat(t *testing.T) {
//	testTiDB, err := ti.CreateDatabase("gigo-dev-tidb", "4000", "mysql",
//		"gigo-dev", "gigo-dev", "gigo_test_db")
//	if err != nil {
//		t.Fatal("Initialize test database failed:", err)
//	}
//
//	// Prepare for the test
//	callingUser := &models.User{
//		ID:       1,
//		UserName: "gigo",
//		Timezone: "UTC",
//		Tier:     1,
//	}
//	sf, err := snowflake.NewNode(1)
//	if err != nil {
//		t.Fatal("Initialize snowflake failed:", err)
//	}
//
//	// Cut off list of user id strings
//	usersIdStrList := []string{"1", "2", "3"}
//
//	params := CreateChatParams{
//		Name:     "Test Chat",
//		ChatType: 2,
//		Users:    usersIdStrList,
//	}
//
//	t.Cleanup(func() {
//		_, err := testTiDB.DB.Exec(`DELETE FROM chat`)
//		if err != nil {
//			t.Log("Failed to clean up: ", err)
//		}
//	})
//
//	response, err := CreateChat(context.Background(), testTiDB, sf, callingUser, params)
//
//	if err != nil {
//		t.Errorf("CreateChat() error: %v", err)
//	}
//
//	respChat := response["chat"].(models.Chat)
//	if respChat.Name != "Test Chat" || respChat.Type != 2 || len(respChat.Users) != len(usersIdStrList) {
//		t.Error("CreateChat() does not return the expected chat")
//	}
//	// Add your other tests here
//}
