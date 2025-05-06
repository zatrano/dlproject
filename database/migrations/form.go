package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MigrateFormsTables(db *gorm.DB) error {
	configslog.SLog.Info("Migrating forms & form_details tables...")
	err := db.AutoMigrate(&models.Form{}, &models.FormDetail{})
	if err != nil {
		configslog.Log.Error("Failed to migrate forms & form_details tables", zap.Error(err))
		return err
	}
	configslog.SLog.Info("Forms & form_details tables migrated successfully")
	return nil
}
