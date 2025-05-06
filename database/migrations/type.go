package migrations

import (
	"errors"

	"davet.link/configs/configslog"
	"davet.link/models"

	"gorm.io/gorm"
)

func MigrateTypesTable(db *gorm.DB) error {
	configslog.SLog.Info("Type tablosu migrate ediliyor...")

	if err := db.AutoMigrate(&models.Type{}); err != nil {
		errMsg := "Type tablosu migrate edilemedi: " + err.Error()
		configslog.Log.Error(errMsg)
		return errors.New(errMsg)
	}

	configslog.SLog.Info("Type tablosu migrate işlemi tamamlandı.")
	return nil
}
