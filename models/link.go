// models/link.go
package models

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"

	"davet.link/configs/configslog" // Loglama

	"go.uber.org/zap" // Loglama
	"gorm.io/gorm"
)

// Link struct tanımı...
type Link struct {
	BaseModel
	Key           string `gorm:"type:varchar(20);uniqueIndex;not null"`
	TypeID        uint   `gorm:"not null;index"`
	TargetID      uint   `gorm:"not null;index:idx_link_target"`
	CreatorUserID uint   `gorm:"index;not null"`
	Type          Type   `gorm:"foreignKey:TypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Creator       User   `gorm:"foreignKey:CreatorUserID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}

// BeforeCreate GORM hook'u, Link kaydı oluşturulmadan önce çalışır.
// Otomatik olarak 20 haneli *benzersiz* bir Key üretir ve atar.
func (l *Link) BeforeCreate(tx *gorm.DB) (err error) {
	// Eğer Key manuel olarak atanmışsa (boş değilse), dokunma.
	// Bu durumun normalde olmaması gerekir, anahtar üretimi otomatik olmalı.
	if l.Key != "" {
		configslog.SLog.Warnf("Link BeforeCreate: Key manuel olarak atanmış (%s), otomatik üretim atlanıyor.", l.Key)
		return nil
	}

	// Benzersiz Key üretme denemeleri için sayaç ve limit
	maxAttempts := 10 // Makul bir deneme sayısı (20 haneli alfanümerikte çakışma çok çok düşük)
	for i := 0; i < maxAttempts; i++ {
		// 1. Güvenli 20 haneli alfanümerik anahtar üret
		generatedKey, keyErr := generateSecureRandomStringForLink(20)
		if keyErr != nil {
			configslog.Log.Error("Link BeforeCreate: Güvenli anahtar üretilemedi", zap.Error(keyErr), zap.Int("attempt", i+1))
			// Bu kritik bir hata, random kaynak sorunu olabilir. İşlemi durdur.
			return fmt.Errorf("güvenli anahtar üretilemedi: %w", keyErr)
		}

		// 2. Üretilen Key'in veritabanında zaten var olup olmadığını KONTROL ET
		// Bu kontrol transaction (tx) üzerinden yapılmalı.
		var count int64
		// Sadece 'key' sütununa göre count alıyoruz.
		countErr := tx.Model(&Link{}).Where("key = ?", generatedKey).Count(&count).Error
		if countErr != nil {
			// Veritabanı kontrol hatası
			configslog.Log.Error("Link BeforeCreate: Anahtar benzersizlik kontrolü başarısız",
				zap.String("attemptedKey", generatedKey),
				zap.Int("attempt", i+1),
				zap.Error(countErr))
			return fmt.Errorf("anahtar kontrol edilemedi: %w", countErr) // Devam etmek riskli
		}

		// 3. Eğer count 0 ise, anahtar benzersizdir. Ata ve fonksiyondan çık.
		if count == 0 {
			l.Key = generatedKey
			configslog.SLog.Debugf("Link BeforeCreate: Benzersiz anahtar üretildi ve atandı (Deneme %d)", i+1)
			return nil // Başarılı, hook sonlandı
		}

		// 4. Anahtar zaten varsa (çakışma), logla ve döngü devam eder (yeni anahtar üretilir).
		configslog.Log.Warn("Link BeforeCreate: Anahtar çakışması tespit edildi, yeniden deneniyor...",
			zap.String("collidedKey", generatedKey),
			zap.Int("attempt", i+1))
	}

	// Eğer döngü maksimum deneme sayısına ulaştıysa ve hala benzersiz anahtar bulunamadıysa,
	// bu çok olağandışı bir durumdur ama yine de hata döndürmek gerekir.
	configslog.Log.Error("Link BeforeCreate: Maksimum deneme sayısına ulaşıldı, benzersiz anahtar üretilemedi.")
	return errors.New("benzersiz link anahtarı üretilemedi (maksimum deneme aşıldı)")
}

// BeforeUpdate GORM hook'u - Key alanının değiştirilmesini engeller.
func (l *Link) BeforeUpdate(tx *gorm.DB) (err error) {
	// ... (önceki gibi) ...
	if tx.Statement.Changed("Key") {
		configslog.Log.Warn("Link BeforeUpdate: Key alanını değiştirme girişimi engellendi.", zap.Uint("linkID", l.ID))
		return errors.New("link anahtarı (Key) değiştirilemez")
	}
	return nil
}

// --- Yardımcı Fonksiyonlar (Sadece bu dosya içinde) ---
const charsetForLink = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func generateSecureRandomStringForLink(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("length must be positive")
	}
	b := make([]byte, length)
	max := big.NewInt(int64(len(charsetForLink)))
	for i := range b {
		randIndex, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", fmt.Errorf("random index generation failed: %w", err)
		}
		b[i] = charsetForLink[randIndex.Int64()]
	}
	return string(b), nil
}
