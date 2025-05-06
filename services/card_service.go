// services/card_service.go
package services

import (
	"context"
	"errors"
	"fmt"

	// Zaman validasyonu için
	"davet.link/configs"            // DB transaction için
	"davet.link/configs/configslog" // Loglama için
	"davet.link/models"
	"davet.link/pkg/queryparams" // Pagination için
	"davet.link/repositories"
	"davet.link/utils" // Yardımcı fonksiyonlar için

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause" // Lock için
)

// CardServiceError özel servis hataları
type CardServiceError string

func (e CardServiceError) Error() string { return string(e) }

const (
	ErrCardNotFound       CardServiceError = "kartvizit bulunamadı"
	ErrCardCreationFailed CardServiceError = "kartvizit oluşturulamadı"
	ErrCardUpdateFailed   CardServiceError = "kartvizit güncellenemedi"
	ErrCardDeletionFailed CardServiceError = "kartvizit silinemedi"
	ErrCardForbidden      CardServiceError = "bu işlem için yetkiniz yok"
	ErrCrdInvalidInput    CardServiceError = "geçersiz girdi verisi"
	ErrCardNameRequired   CardServiceError = "isim ve soyisim zorunludur"
	// Genel hatalar
	ErrCrdLinkCreationFailed CardServiceError = "kartvizit için link oluşturulamadı"
	ErrCrdTypeNotFound       CardServiceError = "kartvizit hizmet türü bulunamadı"
	ErrCrdLinkUpdateFailed   CardServiceError = "kartvizit linki güncellenemedi"
	ErrCrdLinkDeletionFailed CardServiceError = "kartvizit linki silinemedi"
)

// ICardService kartvizit işlemleri için arayüz.
type ICardService interface {
	// BaseRepository tarzı parametreler
	CreateCard(ctx context.Context, creatorUserID uint, detailData models.CardDetail) (*models.Card, error)
	GetCardByID(ctx context.Context, id uint, requestingUserID uint) (*models.Card, error) // Yetki kontrolü ile
	GetCardByKey(ctx context.Context, key string) (*models.Card, error)                    // Public erişim
	GetCardsForUserPaginated(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error)
	UpdateCard(ctx context.Context, id uint, updatingUserID uint, detailData models.CardDetail, isEnabled bool) error
	DeleteCard(ctx context.Context, id uint, deletingUserID uint) error
	GetCardCountForUser(ctx context.Context, creatorUserID uint) (int64, error)
	// GetAllCardsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) // Admin için
}

// CardService ICardService arayüzünü uygular.
type CardService struct {
	repo     repositories.ICardRepository
	linkRepo repositories.ILinkRepository // Link işlemleri için ayrı repo
	typeRepo repositories.ITypeRepository // Type işlemleri için ayrı repo
	userRepo repositories.IUserRepository // Yetki kontrolü için User repo
	db       *gorm.DB                     // Transaction yönetimi için
}

// NewCardService yeni bir CardService örneği oluşturur.
// Bağımlılıklar (repository'ler) constructor injection ile verilmeli (best practice).
func NewCardService() ICardService {
	// Şimdilik direkt oluşturalım
	db := configs.GetDB()
	return &CardService{
		repo:     repositories.NewCardRepository(),
		linkRepo: repositories.NewLinkRepository(),
		typeRepo: repositories.NewTypeRepository(),
		userRepo: repositories.NewUserRepository(),
		db:       db,
	}
}

// --- Yardımcı Metodlar ---

// ValidateCardDetail temel validasyonları yapar.
func ValidateCardDetail(detail models.CardDetail) error {
	if detail.FirstName == "" || detail.LastName == "" {
		return ErrCardNameRequired
	}
	// TODO: Email, URL format kontrolleri eklenebilir
	return nil
}

// contextWithUserID context'e user_id ekler (BaseModel hook'ları için).
func contextWithUserID(ctx context.Context, userID uint) context.Context {
	// models paketindeki context key'i kullanalım
	return context.WithValue(ctx, models.contextUserIDKey, userID)
}

// --- Servis Metodları ---

// CreateCard yeni bir kartvizit, detayları ve linkini TEK BİR TRANSACTION içinde oluşturur.
func (s *CardService) CreateCard(ctx context.Context, creatorUserID uint, detailData models.CardDetail) (*models.Card, error) {
	// 1. Girdi Validasyonu
	if err := ValidateCardDetail(detailData); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCrdInvalidInput, err)
	}
	if creatorUserID == 0 {
		return nil, fmt.Errorf("%w: Geçersiz oluşturan kullanıcı ID", ErrCrdInvalidInput)
	}

	// 2. Card Type ID'sini al
	cardType, err := s.typeRepo.FindByName(models.TypeNameCard)
	if err != nil {
		configslog.Log.Error("Kartvizit tipi bulunamadı", zap.String("type_name", models.TypeNameCard), zap.Error(err))
		return nil, ErrCrdTypeNotFound
	}

	// 3. Transaction Başlat
	var createdCard *models.Card
	var generatedKey string // Link key'ini dışarı almak için

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		// Transaction context'i (işlemi yapan kullanıcıyı içerir)
		txCtx := contextWithUserID(ctx, creatorUserID)
		// Transaction'lı repo'lar veya direkt tx kullan
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		cardRepoTx := repositories.NewCardRepositoryTx(tx)

		// a. Benzersiz Link Key Üret (Servis Katmanında)
		//    BeforeCreate hook'u yerine burada yapalım ki transaction içinde kontrol edilsin
		//    ve üretilen key'i loglayabilelim.
		var linkKey string
		maxKeyAttempts := 5
		for i := 0; i < maxKeyAttempts; i++ {
			keyAttempt, keyErr := utils.GenerateSecureRandomString(20) // 20 haneli
			if keyErr != nil {
				return ErrCrdLinkCreationFailed
			}

			// Key veritabanında var mı? (Transaction içinde)
			var count int64
			if countErr := tx.Model(&models.Link{}).Where("key = ?", keyAttempt).Count(&count).Error; countErr != nil {
				configslog.Log.Error("Link key benzersizlik kontrolü hatası", zap.Error(countErr))
				return ErrCrdLinkCreationFailed // DB hatası
			}
			if count == 0 {
				linkKey = keyAttempt
				break // Benzersiz key bulundu
			}
			configslog.Log.Warn("Link key çakışması, yeniden deneniyor...", zap.String("key", keyAttempt))
		}
		if linkKey == "" {
			return ErrCrdLinkCreationFailed
		} // Üretilemedi
		generatedKey = linkKey // Dış değişkene ata

		// b. Link Oluştur (Key ile, henüz TargetID yok)
		link := models.Link{
			Key:           linkKey,
			TypeID:        cardType.ID,
			CreatorUserID: creatorUserID,
		}
		if err := linkRepoTx.Create(txCtx, &link); err != nil {
			return ErrCrdLinkCreationFailed
		}

		// c. Card ve CardDetail Oluştur
		card := models.Card{
			LinkID:        link.ID,
			CreatorUserID: creatorUserID,
			// OrganizationID: orgID, // Bu parametre fonksiyona eklenmeli
			IsEnabled: true,
			Detail:    detailData,
		}
		// Create metodu hem card hem detail kaydeder ve hook'ları çalıştırır
		if err := cardRepoTx.CreateCard(txCtx, &card); err != nil {
			return ErrCardCreationFailed
		}

		// d. Link'in TargetID'sini güncelle
		updateData := map[string]interface{}{"target_id": card.ID}
		if err := linkRepoTx.Update(txCtx, link.ID, updateData, creatorUserID); err != nil { // UpdatedBy için creatorUserID
			return ErrCrdLinkUpdateFailed
		}

		// e. Başarılı: Oluşturulan kaydı dışarıdaki değişkene ata
		//    Tekrar sorgulamak yerine mevcut nesneyi güncelleyelim
		card.Link = link           // Linki ekle
		card.Link.Type = *cardType // Type bilgisini ekle
		createdCard = &card

		return nil // Commit transaction
	})

	if txErr != nil {
		// Transaction başarısız oldu, loglama zaten yapıldı.
		return nil, txErr
	}

	configslog.SLog.Infof("Kartvizit ve link başarıyla oluşturuldu: CardID %d, LinkKey: %s", createdCard.ID, generatedKey)
	return createdCard, nil
}

// GetCardByID belirli bir kartviziti ID ve kullanıcı yetkisine göre getirir.
func (s *CardService) GetCardByID(ctx context.Context, id uint, requestingUserID uint) (*models.Card, error) {
	card, err := s.repo.GetCardByID(id) // Repo Preload yapıyor
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrCardNotFound
		} // Repo hatasını servis hatasına çevir
		configslog.Log.Error("GetCardByID: Repo error", zap.Uint("id", id), zap.Error(err))
		return nil, err // Diğer hataları olduğu gibi döndür
	}

	// Yetki Kontrolü
	requestingUser, userErr := s.userRepo.FindByID(ctx, requestingUserID)
	if userErr != nil {
		configslog.Log.Error("GetCardByID Yetki Kontrolü: Kullanıcı bulunamadı", zap.Uint("userID", requestingUserID), zap.Error(userErr))
		return nil, ErrCardForbidden
	}
	if !requestingUser.IsSystem && card.CreatorUserID != requestingUserID {
		configslog.Log.Warn("Yetkisiz kartvizit erişim denemesi", zap.Uint("cardID", id), zap.Uint("userID", requestingUserID), zap.Uint("ownerID", card.CreatorUserID))
		return nil, ErrCardForbidden
	}

	return card, nil
}

// GetCardByKey public link anahtarı ile kartviziti getirir.
func (s *CardService) GetCardByKey(ctx context.Context, key string) (*models.Card, error) {
	if key == "" {
		return nil, ErrCardNotFound
	} // Boş key geçersiz

	// 1. Linki bul
	link, err := s.linkRepo.FindByKey(key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCardNotFound
		}
		configslog.Log.Error("GetCardByKey: FindByKey error", zap.String("key", key), zap.Error(err))
		return nil, err
	}

	// 2. Link tipini kontrol et
	if link.Type.Name != models.TypeNameCard {
		configslog.Log.Warn("Yanlış tipte link anahtarı ile kartvizit arandı", zap.String("key", key), zap.String("link_type", link.Type.Name))
		return nil, ErrCardNotFound
	}

	// 3. TargetID ile kartviziti bul (Context ile değil, public erişim)
	card, err := s.repo.GetCardByID(link.TargetID) // GetCardByID Detail ve Link.Type'ı preload eder
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			configslog.Log.Error("Tutarsız veri: Link var ama kartvizit yok", zap.Uint("link_id", link.ID), zap.Uint("target_id", link.TargetID))
			return nil, ErrCardNotFound
		}
		configslog.Log.Error("GetCardByKey: GetCardByID error", zap.Uint("target_id", link.TargetID), zap.Error(err))
		return nil, err
	}

	// 4. Kartvizit aktif mi?
	if !card.IsEnabled {
		configslog.Log.Info("Pasif kartvizit erişim denemesi", zap.String("key", key), zap.Uint("card_id", card.ID))
		return nil, ErrCardNotFound
	}

	// 5. Şifre kontrolü handler'da yapılmalı.

	return card, nil
}

// GetCardsForUserPaginated kullanıcıya ait kartvizitleri sayfalayarak getirir.
func (s *CardService) GetCardsForUserPaginated(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	if creatorUserID == 0 {
		return nil, errors.New("geçersiz kullanıcı ID")
	}
	// Parametre validasyonu
	if params.Page <= 0 {
		params.Page = queryparams.DefaultPage
	}
	if params.PerPage <= 0 || params.PerPage > queryparams.MaxPerPage {
		params.PerPage = queryparams.DefaultPerPage
	}
	if params.SortBy == "" {
		params.SortBy = "created_at"
	} // Varsayılan
	if params.OrderBy == "" {
		params.OrderBy = "desc"
	}

	cards, totalCount, err := s.repo.FindAllCardsByUserIDPaginated(creatorUserID, params) // Özel repo metodu
	if err != nil {
		configslog.Log.Error("Kullanıcı kartvizitleri alınırken hata", zap.Uint("creatorUserID", creatorUserID), zap.Error(err))
		return nil, err
	}

	totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	result := &queryparams.PaginatedResult{
		Data: cards,
		Meta: queryparams.PaginationMeta{
			CurrentPage: params.Page, PerPage: params.PerPage,
			TotalItems: totalCount, TotalPages: totalPages,
		},
	}
	return result, nil
}

// UpdateCard mevcut bir kartviziti ve detaylarını günceller.
func (s *CardService) UpdateCard(ctx context.Context, id uint, updatingUserID uint, detailData models.CardDetail, isEnabled bool) error {
	// 1. Giriş validasyonu
	if err := ValidateCardDetail(detailData); err != nil {
		return fmt.Errorf("%w: %v", ErrCrdInvalidInput, err)
	}
	if id == 0 || updatingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya güncelleyen kullanıcı ID", ErrCrdInvalidInput)
	}

	// 2. Transaction başlat
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		// Transaction context'i (UpdatedBy için)
		txCtx := contextWithUserID(ctx, updatingUserID)
		// Transaction'lı repo'lar
		cardRepoTx := repositories.NewCardRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Mevcut kaydı kilitli olarak al
		var existingCard models.Card
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Detail").First(&existingCard, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCardNotFound
			}
			configslog.Log.Error("UpdateCard: Kayıt bulunamadı (kilitli)", zap.Uint("id", id), zap.Error(err))
			return err
		}

		// b. Yetki Kontrolü
		requestingUser, userErr := userRepoTx.FindByID(txCtx, updatingUserID)
		if userErr != nil {
			return ErrCardForbidden
		}
		if !requestingUser.IsSystem && existingCard.CreatorUserID != updatingUserID {
			return ErrCardForbidden
		}

		// c. Ana model verilerini güncelle
		existingCard.IsEnabled = isEnabled
		// CreatorUserID değiştirilmez.

		// d. Detay model verilerini güncelle
		existingDetail := existingCard.Detail
		// Tüm alanları kopyala
		existingDetail.Prefix = detailData.Prefix
		existingDetail.FirstName = detailData.FirstName
		existingDetail.LastName = detailData.LastName
		existingDetail.Suffix = detailData.Suffix
		existingDetail.Title = detailData.Title
		existingDetail.Company = detailData.Company
		existingDetail.Department = detailData.Department
		existingDetail.Bio = detailData.Bio
		existingDetail.Email = detailData.Email
		existingDetail.PhoneNumber = detailData.PhoneNumber
		existingDetail.Website = detailData.Website
		existingDetail.Address = detailData.Address
		existingDetail.LinkedInURL = detailData.LinkedInURL
		existingDetail.TwitterURL = detailData.TwitterURL
		existingDetail.GitHubURL = detailData.GitHubURL
		existingDetail.InstagramURL = detailData.InstagramURL
		existingDetail.ProfilePictureURL = detailData.ProfilePictureURL
		existingDetail.LogoURL = detailData.LogoURL
		existingDetail.Theme = detailData.Theme
		existingDetail.PrimaryColor = detailData.PrimaryColor
		existingDetail.SecondaryColor = detailData.SecondaryColor
		existingDetail.AllowSaveContact = detailData.AllowSaveContact

		// e. Önce Detail'i kaydet (context ile)
		// Repo UpdateDetail metodu Save kullanıyor, hook'lar çalışır.
		if err := cardRepoTx.UpdateDetail(txCtx, &existingDetail); err != nil {
			configslog.Log.Error("Kartvizit detayı güncellenirken transaction hatası", zap.Uint("detailID", existingDetail.ID), zap.Error(err))
			return ErrCardUpdateFailed
		}
		// f. Sonra Card'ı kaydet (context ile)
		// Repo Update metodu Save kullanıyor, hook'lar çalışır.
		if err := cardRepoTx.Update(txCtx, &existingCard); err != nil {
			configslog.Log.Error("Kartvizit ana bilgisi güncellenirken transaction hatası", zap.Uint("id", id), zap.Error(err))
			return ErrCardUpdateFailed
		}

		return nil // Transaction başarılı
	})

	if txErr != nil {
		return txErr
	}
	configslog.SLog.Infof("Kartvizit başarıyla güncellendi: ID %d", id)
	return nil
}

// DeleteCard bir kartviziti ve ilişkili linkini siler.
func (s *CardService) DeleteCard(ctx context.Context, id uint, deletingUserID uint) error {
	if id == 0 || deletingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya silen kullanıcı ID", ErrCrdInvalidInput)
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		// Transaction context'i (DeletedBy için)
		txCtx := contextWithUserID(ctx, deletingUserID)
		// Transaction'lı repo'lar
		cardRepoTx := repositories.NewCardRepositoryTx(tx)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli olarak al ve yetki kontrolü yap
		var cardToDelete models.Card
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Link").First(&cardToDelete, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCardNotFound
			}
			configslog.Log.Error("DeleteCard: Kayıt bulunamadı (kilitli)", zap.Uint("id", id), zap.Error(err))
			return err
		}

		requestingUser, userErr := userRepoTx.FindByID(txCtx, deletingUserID)
		if userErr != nil {
			return ErrCardForbidden
		}
		if !requestingUser.IsSystem && cardToDelete.CreatorUserID != deletingUserID {
			return ErrCardForbidden
		}

		// b. İlişkili Link'i al
		linkToDelete := cardToDelete.Link
		if linkToDelete.ID == 0 {
			configslog.Log.Warn("DeleteCard: İlişkili link bulunamadı.", zap.Uint("cardID", id))
			// Link olmasa da kartı silmeye devam edebiliriz.
		}

		// c. Önce Card'ı sil (Detail cascade ile silinir) - Context ile
		// Repo Delete metodu context ve deletedByID almalı (ideal)
		// Şimdilik BaseRepository Delete metodunu çağırıyoruz, o context'i alıyor.
		if err := cardRepoTx.DeleteCard(txCtx, id); err != nil {
			if errors.Is(err, repositories.ErrNotFound) {
				return ErrCardNotFound
			} // Repo hatasını çevir
			configslog.Log.Error("Kartvizit silinirken transaction hatası (Card)", zap.Uint("id", id), zap.Error(err))
			return ErrCardDeletionFailed
		}

		// d. Sonra Link'i sil (eğer bulunduysa) - Context ile
		if linkToDelete.ID != 0 {
			// Repo Delete metodu context ve deletedByID almalı (ideal)
			if err := linkRepoTx.Delete(txCtx, &linkToDelete); err != nil {
				// Link silinemezse ne yapmalı? Hata loglanır, transaction geri alınır.
				configslog.Log.Error("Link silinirken transaction hatası (Card silme sonrası)", zap.Uint("link_id", linkToDelete.ID), zap.Error(err))
				return ErrCrdLinkDeletionFailed
			}
		}

		return nil // Transaction başarılı
	})

	if txErr != nil {
		return txErr
	}
	configslog.SLog.Infof("Kartvizit ve ilişkili link başarıyla silindi: Card ID %d", id)
	return nil
}

// GetCardCountForUser kullanıcıya ait kartvizit sayısını alır.
func (s *CardService) GetCardCountForUser(ctx context.Context, creatorUserID uint) (int64, error) {
	count, err := s.repo.CountCardsByUserID(creatorUserID) // Repo metodu context almıyor, eklenebilir.
	if err != nil {
		configslog.Log.Error("Kullanıcı kartvizit sayısı alınırken hata", zap.Uint("creatorUserID", creatorUserID), zap.Error(err))
		return 0, err
	}
	return count, nil
}

var _ ICardService = (*CardService)(nil)

// --- Transaction'lı Repositoryler için Yardımcı Fonksiyonlar ---
// Bunlar idealde repository paketinde veya ayrı bir yerde olabilir
func NewCardRepositoryTx(tx *gorm.DB) repositories.ICardRepository {
	base := repositories.NewBaseRepository[models.Card](tx) // Base'e tx'i ver
	// base.SetAllowedSortColumns(...) // Sıralama kolonları burada da set edilebilir
	return &repositories.CardRepository{Base: base, Db: tx} // Struct alanları public olmalı veya constructor
}

// func NewLinkRepositoryTx(tx *gorm.DB) repositories.ILinkRepository { ... } // Zaten var
// func NewUserRepositoryTx(tx *gorm.DB) repositories.IUserRepository { ... } // Zaten var
