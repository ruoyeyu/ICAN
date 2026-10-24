package repository

import (
	"errors"
	securityconfig "github.com/tigerowo/infinite-canvas/config"
	"time"

	"github.com/tigerowo/infinite-canvas/model"
	"gorm.io/gorm"
)

func userConfigTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

func GetUserConfig(userID string) (model.UserConfig, bool, error) {
	db, err := DB()
	if err != nil {
		return model.UserConfig{}, false, err
	}

	var config model.UserConfig
	err = db.First(&config, "user_id = ?", userID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.UserConfig{}, false, nil
	}
	if err != nil {
		return model.UserConfig{}, false, err
	}
	config.ModelConfig, err = securityconfig.DecryptData(config.ModelConfig, "user:"+userID+":model")
	if err != nil {
		return model.UserConfig{}, false, err
	}
	config.StorageProvider, err = securityconfig.DecryptData(config.StorageProvider, "user:"+userID+":storage")
	if err != nil {
		return model.UserConfig{}, false, err
	}
	return config, true, nil
}

func SaveUserConfig(config model.UserConfig) (model.UserConfig, error) {
	db, err := DB()
	if err != nil {
		return config, err
	}

	config.UpdatedAt = userConfigTimestamp()
	stored := config
	stored.ModelConfig, err = securityconfig.EncryptData(config.ModelConfig, "user:"+config.UserID+":model")
	if err != nil {
		return config, err
	}
	stored.StorageProvider, err = securityconfig.EncryptData(config.StorageProvider, "user:"+config.UserID+":storage")
	if err != nil {
		return config, err
	}
	return config, db.Save(&stored).Error
}
