package database

import (
	"davet.link/configs/configslog"
	"davet.link/database/migrations"
	"davet.link/database/seeders"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Initialize(db *gorm.DB, migrate bool, seed bool) {
	if !migrate && !seed {
		configslog.SLog.Info("Migrate veya seed bayrağı belirtilmedi, işlem yapılmayacak.")
		return
	}

	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			configslog.Log.Fatal("Veritabanı başlatma işlemi başarısız oldu, geri alındı (panic)", zap.Any("panic_info", r))
		}
		if tx.Error != nil && tx.Error != gorm.ErrInvalidTransaction {
			configslog.SLog.Warn("Başlatma sırasında hata oluştuğu için işlem geri alınıyor.")
			tx.Rollback()
		}
	}()

	configslog.SLog.Info("Veritabanı başlatma işlemi başlıyor...")

	if migrate {
		configslog.SLog.Info("Migrasyonlar çalıştırılıyor...")
		if err := RunMigrationsInOrder(tx); err != nil {
			tx.Rollback()
			configslog.Log.Fatal("Migrasyon başarısız oldu", zap.Error(err))
		}
		configslog.SLog.Info("Migrasyonlar tamamlandı.")
	} else {
		configslog.SLog.Info("Migrate bayrağı belirtilmedi, migrasyon adımı atlanıyor.")
	}

	if seed {
		configslog.SLog.Info("Seeder'lar çalıştırılıyor...")
		if err := CheckAndRunSeeders(tx); err != nil {
			tx.Rollback()
			configslog.Log.Fatal("Seeding başarısız oldu", zap.Error(err))
		}
		configslog.SLog.Info("Seeder'lar tamamlandı.")
	} else {
		configslog.SLog.Info("Seed bayrağı belirtilmedi, seeder adımı atlanıyor.")
	}

	configslog.SLog.Info("İşlem commit ediliyor...")
	if err := tx.Commit().Error; err != nil {
		configslog.Log.Fatal("Commit başarısız oldu", zap.Error(err))
	}

	configslog.SLog.Info("Veritabanı başlatma işlemi başarıyla tamamlandı")
}

func RunMigrationsInOrder(db *gorm.DB) error {
	configslog.SLog.Info(" -> User migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateUsersTable(db); err != nil {
		configslog.Log.Error("Users tablosu migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> User migrasyonları tamamlandı.")

	configslog.SLog.Info("Tüm migrasyonlar başarıyla çalıştırıldı.")
	return nil
}

func CheckAndRunSeeders(db *gorm.DB) error {
	systemUser := seeders.GetSystemUserConfig()
	var existingUser models.User
	result := db.Where("account = ? AND type = ?", systemUser.Account, models.Dashboard).First(&existingUser)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			configslog.SLog.Info("Sistem kullanıcısı oluşturuluyor: %s (%s)...", systemUser.Name, systemUser.Account)
			if err := seeders.SeedSystemUser(db); err != nil {
				configslog.Log.Error("Sistem kullanıcısı seed edilemedi", zap.Error(err))
				return err
			}
			configslog.SLog.Info(" -> Sistem kullanıcısı oluşturuldu.")
		} else {
			configslog.Log.Error("Sistem kullanıcısı kontrol edilirken hata", zap.Error(result.Error))
			return result.Error
		}
	} else {
		configslog.SLog.Info("Sistem kullanıcısı '%s' (%s) zaten mevcut, oluşturma adımı atlanıyor.",
			existingUser.Name, existingUser.Account)
		configslog.SLog.Info("Mevcut sistem kullanıcısı '%s' için güncelleme kontrolü yapılıyor...", existingUser.Account)
		if err := seeders.SeedSystemUser(db); err != nil {
			configslog.Log.Error("Mevcut sistem kullanıcısı güncellenirken/kontrol edilirken hata", zap.Error(err))
			return err
		}

	}
	return nil
}
