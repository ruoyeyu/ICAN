package repository

import (
	"errors"
	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
	"time"
)

func SaveLoginSession(session model.LoginSession) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Create(&session).Error
}
func LoginSessionExists(id, userID string) bool {
	db, err := DB()
	if err != nil {
		return false
	}
	var count int64
	return db.Model(&model.LoginSession{}).Where("id = ? AND user_id = ? AND expires_at > ?", id, userID, time.Now().Unix()).Count(&count).Error == nil && count == 1
}
func DeleteLoginSession(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Delete(&model.LoginSession{}, "id = ?", id).Error
}
func TouchUserLogin(id, current string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Model(&model.User{}).Where("id = ?", id).Updates(map[string]any{"last_login_at": current}).Error
}
func SaveInvitation(invitation model.Invitation) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Create(&invitation).Error
}
func ListInvitations() ([]model.Invitation, error) {
	db, err := DB()
	if err != nil {
		return nil, err
	}
	var items []model.Invitation
	err = db.Order("created_at desc").Find(&items).Error
	return items, err
}
func RevokeInvitation(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Model(&model.Invitation{}).Where("id = ?", id).Update("revoked", true).Error
}
func RegisterInvitedUser(user model.User, codeHash string) (model.User, error) {
	db, err := DB()
	if err != nil {
		return user, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		used := tx.Model(&model.Invitation{}).Where("code_hash = ? AND used_by = '' AND revoked = ? AND (expires_at = 0 OR expires_at > ?)", codeHash, false, time.Now().Unix()).Update("used_by", user.ID)
		if used.Error != nil {
			return used.Error
		}
		if used.RowsAffected != 1 {
			return errors.New("邀请码无效、已使用或已过期")
		}
		return tx.Create(&user).Error
	})
	return user, err
}

func RevokeMediaLinks(id string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Model(&model.StorageObject{}).Where("id = ?", id).Update("share_version", gorm.Expr("share_version + 1")).Error
}

func SaveUserVersioned(user model.User, expected uint64) (model.User, error) {
	db, err := DB()
	if err != nil {
		return user, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.User{}).Where("id = ? AND token_version = ?", user.ID, expected).Select("*").Omit("id").Updates(&user)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("用户状态已改变，请重新读取后修改")
		}
		if user.TokenVersion != expected {
			return tx.Delete(&model.LoginSession{}, "user_id = ?", user.ID).Error
		}
		return nil
	})
	return user, err
}
