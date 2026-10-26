package repository

import (
	"path/filepath"
	"testing"

	"github.com/libtnb/sqlite"
	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
)

type invitationV0 struct {
	ID        string `gorm:"primaryKey"`
	CodeHash  string `gorm:"uniqueIndex"`
	CreatedBy string
	CreatedAt string
	ExpiresAt int64
	UsedBy    string
	Revoked   bool
}

func (invitationV0) TableName() string { return "invitations" }

func TestInvitationsV1BackupMigration(t *testing.T) {
	before, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "before.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err = before.AutoMigrate(&invitationV0{}); err != nil {
		t.Fatal(err)
	}
	rows := []invitationV0{
		{ID: "active", CodeHash: "original-active-hash", CreatedBy: "admin", CreatedAt: "unchanged", ExpiresAt: 9999999999},
		{ID: "revoked", CodeHash: "revoked-hash", Revoked: true},
		{ID: "used", CodeHash: "used-hash", UsedBy: "existing-account"},
		{ID: "used-revoked", CodeHash: "used-revoked-hash", UsedBy: "existing-account-2", Revoked: true},
	}
	if err = before.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "backup.db")
	if err = before.Exec("VACUUM INTO ?", backup).Error; err != nil {
		t.Fatal(err)
	}
	after, err := gorm.Open(sqlite.Open(backup), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = migrateInvitationsV1(after); err != nil {
			t.Fatal(err)
		}
	}
	if !after.Migrator().HasColumn(&model.Invitation{}, "EncryptedCode") || before.Migrator().HasColumn(&model.Invitation{}, "EncryptedCode") {
		t.Fatal("wrong migration target")
	}
	var oldRows []invitationV0
	if err = before.Order("id").Find(&oldRows).Error; err != nil || len(oldRows) != 4 {
		t.Fatal("source backup changed")
	}
	for _, old := range rows {
		var got invitationV0
		err = after.First(&got, "id = ?", old.ID).Error
		if old.ID == "revoked" {
			if err != gorm.ErrRecordNotFound {
				t.Fatal("revoked unused code retained")
			}
			continue
		}
		if err != nil || got != old {
			t.Fatal("legacy/used invitation changed or old-shape rollback read failed")
		}
		var current model.Invitation
		if err = after.First(&current, "id = ?", old.ID).Error; err != nil || current.EncryptedCode != "" {
			t.Fatal("invented original code")
		}
	}
}
