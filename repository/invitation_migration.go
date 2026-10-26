package repository

import (
	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
)

// V1 adds recoverable ciphertext without replacing existing invitation hashes.
func migrateInvitationsV1(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.Invitation{}) {
		return db.Migrator().CreateTable(&model.Invitation{})
	}
	if !db.Migrator().HasColumn(&model.Invitation{}, "EncryptedCode") {
		if err := db.Migrator().AddColumn(&model.Invitation{}, "EncryptedCode"); err != nil {
			return err
		}
	}
	// Only old revoked, unused codes are removed; used records remain intact.
	return db.Where("revoked = ? AND used_by = ''", true).Delete(&model.Invitation{}).Error
}
