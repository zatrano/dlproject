package migrations

import (
	"errors"

	"davet.link/configs/configslog"
	"davet.link/models"

	"gorm.io/gorm"
)

func MigrateUsersTable(db *gorm.DB) error {
	configslog.SLog.Info("User tablosu için enum tipi kontrol ediliyor...")

	dropEnumQuery := `DROP TYPE IF EXISTS user_type;`
	rawDB, err := db.DB()
	if err != nil {
		return errors.New("DB instance alınamadı: " + err.Error())
	}

	_, err = rawDB.Exec(dropEnumQuery)
	if err != nil {
		return errors.New("user_type enum silinemedi: " + err.Error())
	}
	configslog.SLog.Info("user_type enum başarıyla silindi.")

	createEnum := `CREATE TYPE user_type AS ENUM ('dashboard', 'panel');`
	_, err = rawDB.Exec(createEnum)
	if err != nil {
		return errors.New("user_type enum oluşturulamadı: " + err.Error())
	}
	configslog.SLog.Info("user_type enum başarıyla oluşturuldu.")

	configslog.SLog.Info("User tablosu migrate ediliyor...")
	if err := db.AutoMigrate(&models.User{}); err != nil {
		return errors.New("User tablosu migrate edilemedi: " + err.Error())
	}

	configslog.SLog.Info("User tablosu migrate işlemi tamamlandı.")
	return nil
}
