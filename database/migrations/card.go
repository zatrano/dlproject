package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MigrateCardsTables(db *gorm.DB) error {
	configslog.SLog.Info("Migrating cards & card_details tables...")
	err := db.AutoMigrate(&models.Card{}, &models.CardDetail{})
	if err != nil {
		configslog.Log.Error("Failed to migrate cards & card_details tables", zap.Error(err))
		return err
	}
	configslog.SLog.Info("Cards & card_details tables migrated successfully")
	return nil
}
