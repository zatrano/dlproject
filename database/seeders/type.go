package seeders

import (
	"context"
	"errors"

	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const contextUserIDKey = "user_id"

func SeedTypes(db *gorm.DB) error {
	systemUserID := uint(1)
	ctx := context.WithValue(context.Background(), contextUserIDKey, systemUserID)

	typesToSeed := []models.Type{
		{Name: models.TypeNameInvitation, Description: "Dijital Davetiye Hizmeti"},
		{Name: models.TypeNameAppointment, Description: "Online Randevu Hizmeti"},
		{Name: models.TypeNameForm, Description: "Online Form Hizmeti"},
		{Name: models.TypeNameCard, Description: "Dijital Kartvizit Hizmeti"},
	}

	var createdCount int64 = 0
	var errorOccurred bool = false

	configslog.SLog.Info("Hizmet türleri seed işlemi başlıyor...")

	for _, typeToSeed := range typesToSeed {
		var existingType models.Type
		result := db.Where("name = ?", typeToSeed.Name).First(&existingType)

		if result.Error == nil {
			configslog.SLog.Debugf("Hizmet türü '%s' zaten mevcut, oluşturma atlanıyor.", typeToSeed.Name)
			continue
		} else if !errors.Is(result.Error, gorm.ErrRecordNotFound) {
			configslog.Log.Error("Hizmet türü kontrol edilirken veritabanı hatası",
				zap.String("type_name", typeToSeed.Name),
				zap.Error(result.Error),
			)
			errorOccurred = true
			continue
		}

		configslog.SLog.Infof("Hizmet türü '%s' oluşturuluyor...", typeToSeed.Name)

		err := db.WithContext(ctx).Create(&typeToSeed).Error
		if err != nil {
			configslog.Log.Error("Hizmet türü oluşturulamadı",
				zap.String("type_name", typeToSeed.Name),
				zap.Error(err),
			)
			errorOccurred = true
			continue
		}

		configslog.SLog.Infof("Hizmet türü '%s' başarıyla oluşturuldu (ID: %d, Oluşturan: %d).", typeToSeed.Name, typeToSeed.ID, systemUserID)
		createdCount++
	}

	if createdCount > 0 {
		configslog.SLog.Infof("%d adet yeni hizmet türü başarıyla seed edildi.", createdCount)
	} else if !errorOccurred {
		configslog.SLog.Info("Tüm hizmet türleri zaten mevcut, yeni ekleme yapılmadı.")
	}

	if errorOccurred {
		return errors.New("hizmet türleri seed edilirken en az bir hata oluştu")
	}

	configslog.SLog.Info("Hizmet türleri seed işlemi başarıyla tamamlandı.")
	return nil
}
