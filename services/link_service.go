package services

import (
	"context" // Context eklendi
	"errors"
	"fmt" // Hata formatlama için
	"strings"

	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/repositories" // Random string için

	"go.uber.org/zap"
	"gorm.io/gorm" // ErrRecordNotFound
)

// LinkServiceError özel servis hataları
type LinkServiceError string

func (e LinkServiceError) Error() string { return string(e) }

const (
	ErrLinkNotFound            LinkServiceError = "link bulunamadı"
	ErrLinkCreationFailedServ  LinkServiceError = "link oluşturulamadı" // Repo'dan farklı isim
	ErrLinkKeyGenerationFailed LinkServiceError = "benzersiz link anahtarı üretilemedi"
	ErrLinkDeletionFailedServ  LinkServiceError = "link silinemedi" // Repo'dan farklı isim
	ErrLinkUpdateFailedServ    LinkServiceError = "link güncellenemedi"
	ErrLinkInvalidInput        LinkServiceError = "geçersiz link girdisi"
)

// ILinkService link işlemleri için arayüz.
type ILinkService interface {
	CreateLink(ctx context.Context, creatorUserID uint, typeID uint) (*models.Link, error)       // Context ve CreatorID eklendi
	GetLinkByKey(ctx context.Context, key string) (*models.Link, error)                          // Context eklendi
	GetLinkByID(ctx context.Context, id uint) (*models.Link, error)                              // Context eklendi
	UpdateLinkTarget(ctx context.Context, updatingUserID uint, linkID uint, targetID uint) error // Context ve UserID eklendi
	DeleteLink(ctx context.Context, deletingUserID uint, linkID uint) error                      // Context ve UserID eklendi
	// KeyExists(ctx context.Context, key string) (bool, error) // Repo içinde kalsın
}

// LinkService ILinkService arayüzünü uygular.
type LinkService struct {
	repo repositories.ILinkRepository
	// db *gorm.DB // Transaction gerekirse burada da olabilir
}

// NewLinkService yeni bir LinkService örneği oluşturur.
func NewLinkService() ILinkService {
	return &LinkService{
		repo: repositories.NewLinkRepository(),
		// db: configs.GetDB(),
	}
}

// CreateLink yeni bir link oluşturur (TargetID başlangıçta 0 olur).
// Key üretimi modelin BeforeCreate hook'unda yapılır.
func (s *LinkService) CreateLink(ctx context.Context, creatorUserID uint, typeID uint) (*models.Link, error) {
	if typeID == 0 || creatorUserID == 0 {
		return nil, fmt.Errorf("%w: geçersiz typeID veya creatorUserID", ErrLinkInvalidInput)
	}

	// Context'e user ID ekle (BaseModel hook'u için)
	ctxWithUser := context.WithValue(ctx, models.contextUserIDKey, creatorUserID)

	link := &models.Link{
		TypeID:        typeID,
		CreatorUserID: creatorUserID, // Explicit olarak da atayalım
		// Key:        (BeforeCreate üretecek)
		// TargetID:   0 (varsayılan)
	}

	// Repository'ye context'i gönder
	if err := s.repo.Create(ctxWithUser, link); err != nil {
		// Model hook'undan veya DB'den gelen hata olabilir
		configslog.Log.Error("Link oluşturulurken repository hatası", zap.Error(err), zap.Uint("typeID", typeID), zap.Uint("creatorUserID", creatorUserID))
		// Unique key çakışması hatasını yakalayıp tekrar denemek yerine hata döndürelim (BeforeCreate denedi)
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			// Bu hata BeforeCreate'in denemelerine rağmen oluştuysa ciddi bir sorun var demektir.
			configslog.Log.Error("Link key çakışması devam ediyor, işlem başarısız.", zap.Uint("creatorUserID", creatorUserID))
			return nil, ErrLinkKeyGenerationFailed // Daha uygun bir hata
		}
		return nil, ErrLinkCreationFailedServ
	}

	// Oluşturulan linki (ID ve Key ile birlikte) döndür
	// Repo'dan tekrar çekmeye gerek yok, Create işlemi ID'yi doldurur.
	// Ama Key'i loglamak için çekmek gerekebilir veya hook loglayabilir.
	configslog.SLog.Infof("Link başarıyla oluşturuldu: ID %d, Key: %s (Oluşturan: %d)", link.ID, link.Key, creatorUserID)
	return link, nil
}

// GetLinkByKey public anahtar ile linki alır.
func (s *LinkService) GetLinkByKey(ctx context.Context, key string) (*models.Link, error) {
	link, err := s.repo.FindByKey(ctx, key) // Context'i ilet
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLinkNotFound // Servis hatası döndür
		}
		// Repo zaten logladı, hatayı yukarı taşı
		return nil, err
	}
	return link, nil
}

// GetLinkByID ID ile linki alır.
func (s *LinkService) GetLinkByID(ctx context.Context, id uint) (*models.Link, error) {
	link, err := s.repo.FindByID(ctx, id) // Context'i ilet
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrLinkNotFound
		}
		return nil, err
	}
	return link, nil
}

// UpdateLinkTarget bir linkin TargetID'sini günceller.
func (s *LinkService) UpdateLinkTarget(ctx context.Context, updatingUserID uint, linkID uint, targetID uint) error {
	if linkID == 0 || targetID == 0 || updatingUserID == 0 {
		return fmt.Errorf("%w: geçersiz linkID, targetID veya updatingUserID", ErrLinkInvalidInput)
	}

	// Güncelleme verisi
	updateData := map[string]interface{}{"target_id": targetID}

	// Repository'ye context ve updatingUserID'yi gönder (BeforeUpdate hook'u için)
	err := s.repo.Update(ctx, linkID, updateData, updatingUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			configslog.Log.Warn("Link TargetID güncellenemedi: Link bulunamadı", zap.Uint("link_id", linkID))
			return ErrLinkNotFound // Hata dönüşümü
		}
		configslog.Log.Error("Link TargetID güncellenirken repository hatası", zap.Uint("link_id", linkID), zap.Uint("target_id", targetID), zap.Error(err))
		return ErrLinkUpdateFailedServ
	}
	configslog.SLog.Infof("Link TargetID başarıyla güncellendi: LinkID %d, TargetID %d (Güncelleyen: %d)", linkID, targetID, updatingUserID)
	return nil
}

// DeleteLink bir linki siler. İlişkili hizmetin zaten silinmiş olması beklenir.
func (s *LinkService) DeleteLink(ctx context.Context, deletingUserID uint, linkID uint) error {
	if linkID == 0 || deletingUserID == 0 {
		return fmt.Errorf("%w: geçersiz linkID veya deletingUserID", ErrLinkInvalidInput)
	}

	// Önce linki bulalım ki loglama yapabilelim (opsiyonel)
	link, err := s.repo.FindByID(ctx, linkID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLinkNotFound
		}
		return err // Diğer DB hataları
	}

	// Repository'ye context ve deletingUserID'yi gönder
	err = s.repo.Delete(ctx, link, deletingUserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrLinkNotFound
		} // Zaten silinmiş veya yok
		configslog.Log.Error("Link silinirken repository hatası", zap.Uint("link_id", linkID), zap.Error(err))
		return ErrLinkDeletionFailedServ
	}
	configslog.SLog.Infof("Link başarıyla silindi: ID %d, Key: %s (Silen: %d)", linkID, link.Key, deletingUserID)
	return nil
}

// Arayüz uyumluluğu kontrolü
var _ ILinkService = (*LinkService)(nil)
