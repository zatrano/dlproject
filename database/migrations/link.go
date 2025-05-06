package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MigrateLinksTable(db *gorm.DB) error {
	configslog.SLog.Info("Migrating links table...")
	err := db.AutoMigrate(&models.Link{})
	if err != nil {
		configslog.Log.Error("Failed to migrate links table", zap.Error(err))
		return err
	}
	configslog.SLog.Info("Links table migrated successfully")
	return nil
}
