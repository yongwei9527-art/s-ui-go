package service

import (
	"strconv"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
)

func TestDeleteTokenOnlyDeletesCurrentUsersToken(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatalf("InitDB() error = %v", err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})

	db := database.GetDB()
	owner := model.User{Username: "token-owner", Password: "unused"}
	other := model.User{Username: "token-other", Password: "unused"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}
	ownerToken := model.Tokens{Token: "owner-token", UserId: owner.Id}
	otherToken := model.Tokens{Token: "other-token", UserId: other.Id}
	if err := db.Create(&ownerToken).Error; err != nil {
		t.Fatalf("create owner token: %v", err)
	}
	if err := db.Create(&otherToken).Error; err != nil {
		t.Fatalf("create other token: %v", err)
	}

	service := UserService{}
	if err := service.DeleteToken(owner.Username, strconv.FormatUint(uint64(otherToken.Id), 10)); err == nil {
		t.Fatal("DeleteToken() deleted another user's token without returning an error")
	}

	var count int64
	if err := db.Model(&model.Tokens{}).Where("id = ?", otherToken.Id).Count(&count).Error; err != nil {
		t.Fatalf("count other token: %v", err)
	}
	if count != 1 {
		t.Fatalf("other user's token count = %d, want 1", count)
	}

	if err := service.DeleteToken(owner.Username, strconv.FormatUint(uint64(ownerToken.Id), 10)); err != nil {
		t.Fatalf("DeleteToken() own token error = %v", err)
	}
	if err := db.Model(&model.Tokens{}).Where("id = ?", ownerToken.Id).Count(&count).Error; err != nil {
		t.Fatalf("count owner token: %v", err)
	}
	if count != 0 {
		t.Fatalf("owner's token count = %d, want 0", count)
	}
}
