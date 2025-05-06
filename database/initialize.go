package database

import (
	"davet.link/configs/configslog"
	"davet.link/database/migrations"
	"davet.link/database/seeders"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func Initialize(db *gorm.DB, migrate bool, seed bool) {
	if !migrate && !seed {
		configslog.SLog.Info("Migrate veya seed bayrağı belirtilmedi, işlem yapılmayacak.")
		return
	}

	tx := db.Begin()
	if tx.Error != nil {
		configslog.Log.Fatal("Veritabanı transaction başlatılamadı", zap.Error(tx.Error))
		return
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			configslog.Log.Fatal("Veritabanı başlatma işlemi başarısız oldu (panic)", zap.Any("panic_info", r))
		} else if err := tx.Error; err != nil && err != gorm.ErrInvalidTransaction {
			configslog.SLog.Warn("Başlatma sırasında hata oluştuğu için işlem geri alınıyor.", zap.Error(err))
			rbErr := tx.Rollback().Error
			if rbErr != nil && rbErr != gorm.ErrInvalidTransaction {
				configslog.Log.Error("Rollback sırasında ek hata oluştu", zap.Error(rbErr))
			}
		}
	}()

	configslog.SLog.Info("Veritabanı başlatma işlemi başlıyor...")

	if migrate {
		configslog.SLog.Info("Migrasyonlar çalıştırılıyor...")
		if err := RunMigrationsInOrder(tx); err != nil {
			configslog.Log.Error("Migrasyon başarısız oldu", zap.Error(err))
			return
		}
		configslog.SLog.Info("Migrasyonlar tamamlandı.")
	} else {
		configslog.SLog.Info("Migrate bayrağı belirtilmedi, migrasyon adımı atlanıyor.")
	}

	if seed {
		configslog.SLog.Info("Seeder'lar çalıştırılıyor...")
		if err := CheckAndRunSeeders(tx); err != nil {
			configslog.Log.Error("Seeding başarısız oldu", zap.Error(err))
			return
		}
		configslog.SLog.Info("Seeder'lar tamamlandı.")
	} else {
		configslog.SLog.Info("Seed bayrağı belirtilmedi, seeder adımı atlanıyor.")
	}

	configslog.SLog.Info("İşlem commit ediliyor...")
	if err := tx.Commit().Error; err != nil {
		tx.Error = err
		configslog.Log.Error("Commit başarısız oldu", zap.Error(err))
		return
	}

	configslog.SLog.Info("Veritabanı başlatma işlemi başarıyla tamamlandı")
}

func RunMigrationsInOrder(db *gorm.DB) error {
	configslog.SLog.Info("Migrasyonlar sırayla çalıştırılıyor...")

	configslog.SLog.Info(" -> User migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateUsersTable(db); err != nil {
		configslog.Log.Error("Users tablosu migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> User migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Type migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateTypesTable(db); err != nil {
		configslog.Log.Error("Types tablosu migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Type migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Link migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateLinksTable(db); err != nil {
		configslog.Log.Error("Links tablosu migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Link migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Invitation migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateInvitationsTables(db); err != nil {
		configslog.Log.Error("Invitations tabloları migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Invitation migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Invitation RSVP migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateRSVPTable(db); err != nil {
		configslog.Log.Error("Invitations tabloları migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Invitation RSVP migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Appointment migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateAppointmentsTables(db); err != nil {
		configslog.Log.Error("Appointments tabloları migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Appointment migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Form migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateFormsTables(db); err != nil {
		configslog.Log.Error("Forms tabloları migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Form migrasyonları tamamlandı.")

	configslog.SLog.Info(" -> Card migrasyonları çalıştırılıyor...")
	if err := migrations.MigrateCardsTables(db); err != nil {
		configslog.Log.Error("Cards tabloları migrasyonu başarısız oldu", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Card migrasyonları tamamlandı.")

	configslog.SLog.Info("Tüm migrasyonlar başarıyla çalıştırıldı.")
	return nil
}

func CheckAndRunSeeders(db *gorm.DB) error {
	configslog.SLog.Info("Sistem kullanıcısı kontrol ediliyor/oluşturuluyor/güncelleniyor...")
	if err := seeders.SeedSystemUser(db); err != nil {
		configslog.Log.Error("Sistem kullanıcısı seed/update işlemi başarısız", zap.Error(err))
		return err
	}

	configslog.SLog.Info(" -> Type seeder çalıştırılıyor...")
	if err := seeders.SeedTypes(db); err != nil {
		configslog.Log.Error("Types tablosu seed edilemedi", zap.Error(err))
		return err
	}
	configslog.SLog.Info(" -> Type seeder tamamlandı.")

	configslog.SLog.Info("Tüm seeder'lar başarıyla kontrol edildi/çalıştırıldı.")
	return nil
}
