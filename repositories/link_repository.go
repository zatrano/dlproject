package repositories

import (
	"context" // Context parametresi için
	"errors"

	"davet.link/configs"            // DB bağlantısı için
	"davet.link/configs/configslog" // Loglama için
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ILinkRepository link veritabanı işlemleri için arayüz.
type ILinkRepository interface {
	Create(ctx context.Context, link *models.Link) error                                          // Context eklendi
	FindByID(ctx context.Context, id uint) (*models.Link, error)                                  // Context eklendi
	FindByKey(ctx context.Context, key string) (*models.Link, error)                              // Context eklendi
	KeyExists(ctx context.Context, key string) (bool, error)                                      // Context eklendi
	Update(ctx context.Context, id uint, data map[string]interface{}, updatedByUserID uint) error // Context ve UserID eklendi
	Delete(ctx context.Context, link *models.Link, deletedByUserID uint) error                    // Context ve UserID eklendi
}

// LinkRepository ILinkRepository arayüzünü uygular.
type LinkRepository struct {
	db *gorm.DB
}

// NewLinkRepository yeni bir LinkRepository örneği oluşturur.
func NewLinkRepository() ILinkRepository {
	return &LinkRepository{db: configs.GetDB()}
}

// Context ile çalışan DB örneği döndüren yardımcı fonksiyon
func (r *LinkRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok && tx != nil {
		return tx // Eğer context'te transaction varsa onu kullan
	}
	return r.db.WithContext(ctx) // Yoksa ana bağlantıyı context ile kullan
}

// Create yeni bir link kaydı oluşturur.
// BaseModel BeforeCreate hook'u context'ten user_id alacaktır.
func (r *LinkRepository) Create(ctx context.Context, link *models.Link) error {
	if link == nil {
		return errors.New("oluşturulacak link nil olamaz")
	}
	// Key üretimi modelin BeforeCreate hook'unda yapılır.
	return r.getDB(ctx).Create(link).Error // Context'li DB kullan
}

// FindByID ID ile bir link kaydını bulur (Type ilişkisiyle).
func (r *LinkRepository) FindByID(ctx context.Context, id uint) (*models.Link, error) {
	if id == 0 {
		return nil, errors.New("geçersiz Link ID")
	}
	var link models.Link
	// İlişkili Type bilgisini de çekelim
	err := r.getDB(ctx).Preload("Type").First(&link, id).Error // Context'li DB kullan
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		configslog.Log.Error("LinkRepository.FindByID: DB error", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}
	return &link, nil
}

// FindByKey benzersiz anahtar (key) ile bir link kaydını bulur (Type ilişkisiyle).
func (r *LinkRepository) FindByKey(ctx context.Context, key string) (*models.Link, error) {
	if key == "" {
		return nil, errors.New("aranacak link key'i boş olamaz")
	}
	var link models.Link
	err := r.getDB(ctx).Preload("Type").Where("key = ?", key).First(&link).Error // Context'li DB kullan
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		configslog.Log.Error("LinkRepository.FindByKey: DB error", zap.String("key", key), zap.Error(err))
		return nil, err
	}
	return &link, nil
}

// KeyExists belirli bir anahtarın veritabanında zaten var olup olmadığını kontrol eder.
func (r *LinkRepository) KeyExists(ctx context.Context, key string) (bool, error) {
	if key == "" {
		return false, errors.New("kontrol edilecek link key'i boş olamaz")
	}
	var count int64
	err := r.getDB(ctx).Model(&models.Link{}).Where("key = ?", key).Count(&count).Error // Context'li DB kullan
	if err != nil {
		configslog.Log.Error("LinkRepository.KeyExists: DB error", zap.String("key", key), zap.Error(err))
		return false, err
	}
	return count > 0, nil
}

// Update belirli bir ID'ye sahip linkin verilerini günceller (map ile).
// BaseModel BeforeUpdate hook'u context'ten user_id alacaktır.
func (r *LinkRepository) Update(ctx context.Context, id uint, data map[string]interface{}, updatedByUserID uint) error {
	if id == 0 {
		return errors.New("güncellenecek link ID'si geçersiz")
	}
	if len(data) == 0 {
		return errors.New("güncellenecek veri boş olamaz")
	}
	// Context'e işlemi yapan kullanıcıyı ekleyelim ki hook çalışsın
	ctxWithUser := context.WithValue(ctx, models.contextUserIDKey, updatedByUserID)
	db := r.getDB(ctxWithUser) // UserID eklenmiş context ile DB al

	result := db.Model(&models.Link{}).Where("id = ?", id).Updates(data)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		var exists int64
		countErr := db.Model(&models.Link{}).Where("id = ?", id).Count(&exists).Error
		if countErr == nil && exists == 0 {
			return gorm.ErrRecordNotFound
		}
		// Değişiklik olmasa bile hata dönme
		configslog.SLog.Debug("LinkRepository.Update: Satır etkilenmedi (muhtemelen veri aynı).", zap.Uint("link_id", id))
	}
	return nil
}

// Delete bir link kaydını siler (soft delete).
// BaseModel BeforeDelete hook'u context'ten user_id alabilir (eğer implemente edildiyse).
// GORM soft delete DeletedAt'ı ayarlar. DeletedBy manuel ayarlanmalı.
func (r *LinkRepository) Delete(ctx context.Context, link *models.Link, deletedByUserID uint) error {
	if link == nil || link.ID == 0 {
		return errors.New("silinecek link geçerli değil")
	}
	db := r.getDB(ctx) // Context'li DB al

	// Transaction içinde yapmak daha güvenli
	return db.Transaction(func(tx *gorm.DB) error {
		// 1. DeletedBy alanını güncelle (eğer user ID geçerliyse)
		if deletedByUserID != 0 {
			// Context'e user ID ekleyerek BeforeUpdate hook'unu da tetikleyebiliriz ama
			// sadece DeletedBy'ı güncellemek için UpdateColumn daha iyi.
			if err := tx.Model(link).UpdateColumn("deleted_by", &deletedByUserID).Error; err != nil {
				configslog.Log.Error("LinkRepository.Delete: DeletedBy güncellenemedi", zap.Uint("link_id", link.ID), zap.Error(err))
				return err // Hata varsa transaction rollback olur
			}
		}

		// 2. Soft delete işlemini gerçekleştir (GORM DeletedAt'ı ayarlar)
		result := tx.Delete(link)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// Arayüz uyumluluğu kontrolü
var _ ILinkRepository = (*LinkRepository)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewLinkRepositoryTx(tx *gorm.DB) ILinkRepository {
	return &LinkRepository{db: tx}
}
