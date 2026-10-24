package repository

import (
	"encoding/json"
	"github.com/tigerowo/infinite-canvas/config"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm/clause"
)

// GetSettings 返回 public 和 private 两行配置。
func GetSettings() (model.Settings, error) {
	db, err := DB()
	if err != nil {
		return model.Settings{}, err
	}
	var items []model.Setting
	if err := db.Find(&items).Error; err != nil {
		return model.Settings{}, err
	}
	result := model.Settings{}
	for _, item := range items {
		if item.Key == model.SettingKeyPrivate {
			var sealed string
			if err := json.Unmarshal(item.Value, &sealed); err != nil {
				return result, err
			}
			raw, err := config.DecryptData(sealed, "settings:private")
			if err != nil {
				return result, err
			}
			if err := json.Unmarshal([]byte(raw), &result.Private); err != nil {
				return result, err
			}
		} else if item.Key == model.SettingKeyPublic {
			_ = json.Unmarshal(item.Value, &result.Public)
		}
	}
	return result, nil
}

// SaveSettings 保存 public 和 private 两行配置。
func SaveSettings(settings model.Settings, now string) (model.Settings, error) {
	db, err := DB()
	if err != nil {
		return settings, err
	}
	publicValue, _ := json.Marshal(settings.Public)
	privateValue, _ := json.Marshal(settings.Private)
	sealed, err := config.EncryptData(string(privateValue), "settings:private")
	if err != nil {
		return settings, err
	}
	privateValue, _ = json.Marshal(sealed)
	items := []model.Setting{
		{Key: model.SettingKeyPublic, Value: publicValue, CreatedAt: now, UpdatedAt: now},
		{Key: model.SettingKeyPrivate, Value: privateValue, CreatedAt: now, UpdatedAt: now},
	}
	err = db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&items).Error
	return settings, err
}
