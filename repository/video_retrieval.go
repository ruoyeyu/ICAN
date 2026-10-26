package repository

import (
	"errors"
	"github.com/google/uuid"
	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Additive migration: old task, account, configuration and media tables are untouched.
func migrateVideoRetrievalV1(db *gorm.DB) error {
	if !db.Migrator().HasTable(&model.VideoRetrieval{}) {
		if err := db.Migrator().CreateTable(&model.VideoRetrieval{}); err != nil {
			return err
		}
	}
	// A stopped process cannot finish an in-memory transfer. Retry remains explicit.
	return db.Model(&model.VideoRetrieval{}).Where("status IN ?", []string{"fetching", "saving"}).Updates(map[string]any{"status": "failed", "error_detail": "服务已重启，视频取回中断，请继续取回"}).Error
}

func GetVideoRetrieval(userID, taskID string) (model.VideoRetrieval, error) {
	db, err := DB()
	if err != nil {
		return model.VideoRetrieval{}, err
	}
	var result model.VideoRetrieval
	err = db.First(&result, "user_id = ? AND task_id = ?", userID, taskID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.VideoRetrieval{}, nil
	}
	return result, err
}

func ClaimVideoRetrieval(userID, taskID string, retryAttempt *uint64) (result model.VideoRetrieval, claimed bool, err error) {
	db, err := DB()
	if err != nil {
		return
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		candidate := model.VideoRetrieval{TaskID: taskID, UserID: userID, ObjectID: uuid.NewString(), Status: "fetching", Attempt: 1}
		insert := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate)
		if insert.Error != nil {
			return insert.Error
		}
		claimed = insert.RowsAffected == 1
		if !claimed && retryAttempt != nil {
			update := tx.Model(&model.VideoRetrieval{}).Where("user_id = ? AND task_id = ? AND status = ? AND attempt = ?", userID, taskID, "failed", *retryAttempt).Updates(map[string]any{"status": "fetching", "attempt": gorm.Expr("attempt + 1"), "error_detail": ""})
			if update.Error != nil {
				return update.Error
			}
			claimed = update.RowsAffected == 1
		}
		return tx.First(&result, "user_id = ? AND task_id = ?", userID, taskID).Error
	})
	return
}

func UpdateVideoRetrieval(result model.VideoRetrieval, status, detail string) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Model(&model.VideoRetrieval{}).Where("user_id = ? AND task_id = ? AND attempt = ? AND status IN ?", result.UserID, result.TaskID, result.Attempt, []string{"fetching", "saving"}).Updates(map[string]any{"status": status, "error_detail": detail}).Error
}

func CompleteVideoRetrieval(result model.VideoRetrieval, object model.StorageObject) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		updated := tx.Model(&model.VideoRetrieval{}).Where("user_id = ? AND task_id = ? AND attempt = ? AND status = ?", result.UserID, result.TaskID, result.Attempt, "saving").Updates(map[string]any{"status": "ready", "error_detail": ""})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errors.New("视频取回状态已变化")
		}
		return tx.Save(&object).Error
	})
}

func InvalidateMissingVideoResult(result model.VideoRetrieval) error {
	db, err := DB()
	if err != nil {
		return err
	}
	return db.Model(&model.VideoRetrieval{}).Where("user_id = ? AND task_id = ? AND status = ? AND attempt = ?", result.UserID, result.TaskID, "ready", result.Attempt).Updates(map[string]any{"status": "failed", "error_detail": "已保存的视频不存在，请继续取回"}).Error
}
