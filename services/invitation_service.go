package services

import (
	"context"
	"errors"
	"fmt"
	"time" // Zaman validasyonu için

	"davet.link/configs"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/queryparams"
	"davet.link/repositories"
	"golang.org/x/crypto/bcrypt"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause" // Lock için
	// "golang.org/x/crypto/bcrypt" // Şifre hashleme (gerekirse)
)

// InvitationServiceError özel servis hataları
type InvitationServiceError string

func (e InvitationServiceError) Error() string { return string(e) }

const (
	ErrInvitationNotFound          InvitationServiceError = "davetiye bulunamadı"
	ErrInvitationCreationFailed    InvitationServiceError = "davetiye oluşturulamadı"
	ErrInvitationUpdateFailed      InvitationServiceError = "davetiye güncellenemedi"
	ErrInvitationDeletionFailed    InvitationServiceError = "davetiye silinemedi"
	ErrInvitationForbidden         InvitationServiceError = "bu işlem için yetkiniz yok"
	ErrInvInvalidInput             InvitationServiceError = "geçersiz girdi verisi"
	ErrInvitationTitleRequired     InvitationServiceError = "davetiye başlığı zorunludur"
	ErrInvitationEventTimeRequired InvitationServiceError = "etkinlik zamanı zorunludur"
	ErrInvLinkCreationFailed       InvitationServiceError = "davetiye için link oluşturulamadı" // Genel link hatası yerine
	ErrInvTypeNotFound             InvitationServiceError = "davetiye hizmet türü bulunamadı"   // Genel tip hatası yerine
	ErrInvLinkUpdateFailed         InvitationServiceError = "davetiye linki güncellenemedi"
	ErrInvLinkDeletionFailed       InvitationServiceError = "davetiye linki silinemedi"
	ErrInvPasswordHashingFailed    InvitationServiceError = "davetiye şifresi oluşturulamadı"
)

// IInvitationService davetiye işlemleri için arayüz.
type IInvitationService interface {
	CreateInvitation(ctx context.Context, creatorUserID uint, orgID *uint, detailData models.InvitationDetail) (*models.Invitation, error)
	GetInvitationByID(ctx context.Context, id uint, requestingUserID uint) (*models.Invitation, error)
	GetInvitationByKey(ctx context.Context, key string) (*models.Invitation, error)
	GetInvitationsForUser(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error)
	GetAllInvitationsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) // Admin için
	UpdateInvitation(ctx context.Context, id uint, updatingUserID uint, detailData models.InvitationDetail, isEnabled bool) error
	DeleteInvitation(ctx context.Context, id uint, deletingUserID uint) error
	GetInvitationCountForUser(ctx context.Context, creatorUserID uint) (int64, error)
	GetAllInvitationsCount(ctx context.Context) (int64, error) // Admin için
}

// InvitationService IInvitationService arayüzünü uygular.
type InvitationService struct {
	repo        repositories.IInvitationRepository
	linkService ILinkService // Bağımlılıklar
	typeService ITypeService
	userService IUserService
	db          *gorm.DB // Transaction için
}

// NewInvitationService yeni bir InvitationService örneği oluşturur (DI ile).
func NewInvitationService() IInvitationService {
	// Gerçek uygulamada DI kullanın
	return &InvitationService{
		repo:        repositories.NewInvitationRepository(),
		linkService: NewLinkService(),
		typeService: NewTypeService(),
		userService: NewUserService(),
		db:          configs.GetDB(),
	}
}

// --- Yardımcı Metodlar ---

// ValidateInvitationDetail temel validasyonları yapar.
func ValidateInvitationDetail(detail models.InvitationDetail) error {
	if detail.Title == "" {
		return ErrInvitationTitleRequired
	}
	if detail.EventDateTime.IsZero() {
		return ErrInvitationEventTimeRequired
	}
	// RSVPDeadline, ExpiresAt'dan önce olmalı (eğer ikisi de varsa)
	if detail.RSVPDeadline != nil && detail.ExpiresAt != nil && detail.RSVPDeadline.After(*detail.ExpiresAt) {
		return fmt.Errorf("%w: LCV son tarihi, etkinlik son geçerlilik tarihinden sonra olamaz", ErrInvInvalidInput)
	}
	// MaxPlusOnes negatif olamaz
	if detail.MaxPlusOnes < 0 {
		return fmt.Errorf("%w: Ek kişi sayısı negatif olamaz", ErrInvInvalidInput)
	}
	return nil
}

// contextWithUserID (önceki gibi)
func contextWithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, models.contextUserIDKey, userID)
}

// --- Servis Metodları ---

// CreateInvitation yeni bir davetiye oluşturur.
func (s *InvitationService) CreateInvitation(ctx context.Context, creatorUserID uint, orgID *uint, detailData models.InvitationDetail) (*models.Invitation, error) {
	// 1. Validasyon
	if err := ValidateInvitationDetail(detailData); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvInvalidInput, err)
	}
	if creatorUserID == 0 {
		return nil, fmt.Errorf("%w: Geçersiz oluşturan kullanıcı ID", ErrInvInvalidInput)
	}

	// 2. Type ID
	invitationType, err := s.typeService.GetTypeByName(models.TypeNameInvitation)
	if err != nil {
		return nil, ErrInvTypeNotFound
	}

	// 3. Şifre Hashleme (eğer varsa)
	if detailData.PasswordHash != "" {
		hashedPasswordBytes, hashErr := bcrypt.GenerateFromPassword([]byte(detailData.PasswordHash), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, ErrInvPasswordHashingFailed
		}
		detailData.PasswordHash = string(hashedPasswordBytes)
	}

	// 4. Transaction
	var createdInvitation *models.Invitation
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, creatorUserID)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		invitationRepoTx := repositories.NewInvitationRepositoryTx(tx)

		// a. Link Oluştur (Servis metodunu kullanalım)
		link, err := s.linkService.CreateLink(txCtx, creatorUserID, invitationType.ID)
		if err != nil {
			return ErrInvLinkCreationFailed
		} // Hata tipi çevrildi

		// b. Invitation ve Detail Oluştur
		invitation := models.Invitation{
			LinkID:         link.ID,
			CreatorUserID:  creatorUserID,
			OrganizationID: orgID,
			IsEnabled:      true,
			Detail:         detailData,
		}
		if err := invitationRepoTx.Create(txCtx, &invitation); err != nil {
			return ErrInvitationCreationFailed
		}

		// c. Link TargetID Güncelle
		if err := linkRepoTx.Update(txCtx, link.ID, map[string]interface{}{"target_id": invitation.ID}, creatorUserID); err != nil {
			return ErrInvLinkUpdateFailed
		}

		// d. Oluşturulan nesneyi hazırla
		invitation.Link = *link // Linki ekle
		if invitation.Link.Type.ID == 0 {
			invitation.Link.Type = *invitationType
		}
		createdInvitation = &invitation
		return nil
	})

	if txErr != nil {
		return nil, txErr
	} // Rollback zaten yapıldı
	configslog.SLog.Infof("Davetiye başarıyla oluşturuldu: ID %d, Başlık: %s, LinkKey: %s", createdInvitation.ID, createdInvitation.Detail.Title, createdInvitation.Link.Key)
	return createdInvitation, nil
}

// GetInvitationByID belirli bir davetiyeyi ID ve kullanıcı yetkisine göre getirir.
func (s *InvitationService) GetInvitationByID(ctx context.Context, id uint, requestingUserID uint) (*models.Invitation, error) {
	invitation, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}

	// Yetki Kontrolü
	requestingUser, userErr := s.userService.GetUserByID(ctx, requestingUserID)
	if userErr != nil {
		return nil, ErrInvitationForbidden
	}
	if !requestingUser.IsSystem && invitation.CreatorUserID != requestingUserID {
		return nil, ErrInvitationForbidden
	}

	return invitation, nil
}

// GetInvitationByKey public link anahtarı ile davetiyeyi getirir.
func (s *InvitationService) GetInvitationByKey(ctx context.Context, key string) (*models.Invitation, error) {
	if key == "" {
		return nil, ErrInvitationNotFound
	}

	link, err := s.linkService.GetLinkByKey(ctx, key)
	if err != nil {
		if errors.Is(err, services.ErrLinkNotFound) {
			return nil, ErrInvitationNotFound
		}
		return nil, err
	}
	if link.Type.Name != models.TypeNameInvitation {
		return nil, ErrInvitationNotFound
	}

	invitation, err := s.repo.FindByID(ctx, link.TargetID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrInvitationNotFound
		}
		configslog.Log.Error("GetInvitationByKey: Tutarsız veri (Link var, Invitation yok)", zap.Uint("linkID", link.ID), zap.Uint("targetID", link.TargetID))
		return nil, err
	}

	if !invitation.IsEnabled {
		return nil, ErrInvitationNotFound
	}
	if invitation.Detail.ExpiresAt != nil && time.Now().UTC().After(*invitation.Detail.ExpiresAt) {
		return nil, ErrInvitationNotFound // Süresi dolmuş
	}

	// Şifre kontrolü handler'da

	return invitation, nil
}

// GetInvitationsForUser kullanıcıya ait davetiyeleri sayfalayarak getirir.
func (s *InvitationService) GetInvitationsForUser(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	if creatorUserID == 0 {
		return nil, errors.New("geçersiz kullanıcı ID")
	}
	params.Validate() // Sayfalama limitleri

	invitations, totalCount, err := s.repo.FindAllByUserIDPaginated(ctx, creatorUserID, params)
	if err != nil {
		return nil, err
	} // Repo loglar

	totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	result := &queryparams.PaginatedResult{
		Data: invitations,
		Meta: queryparams.PaginationMeta{
			CurrentPage: params.Page, PerPage: params.PerPage,
			TotalItems: totalCount, TotalPages: totalPages,
		},
	}
	return result, nil
}

// GetAllInvitationsPaginated tüm davetiyeleri sayfalayarak getirir (Admin için).
func (s *InvitationService) GetAllInvitationsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	params.Validate()
	// Repo'da bu metodun implemente edilmesi gerekiyor.
	// invitations, totalCount, err := s.repo.FindAllPaginated(ctx, params)
	// if err != nil { return nil, err }
	// totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	// result := &queryparams.PaginatedResult{ Data: invitations, Meta: utils.PaginationMeta{...} }
	// return result, nil
	return nil, errors.New("GetAllInvitationsPaginated henüz implemente edilmedi") // Geçici
}

// UpdateInvitation mevcut bir davetiyeyi ve detaylarını günceller.
func (s *InvitationService) UpdateInvitation(ctx context.Context, id uint, updatingUserID uint, detailData models.InvitationDetail, isEnabled bool) error {
	// 1. Validasyon
	if err := ValidateInvitationDetail(detailData); err != nil {
		return fmt.Errorf("%w: %v", ErrInvInvalidInput, err)
	}
	if id == 0 || updatingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya güncelleyen kullanıcı ID", ErrInvInvalidInput)
	}

	// 2. Transaction
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, updatingUserID)
		invitationRepoTx := repositories.NewInvitationRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli al
		var existingInvitation models.Invitation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Detail").First(&existingInvitation, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvitationNotFound
			}
			return err
		}

		// b. Yetki Kontrolü
		requestingUser, userErr := userRepoTx.FindByID(txCtx, updatingUserID)
		if userErr != nil {
			return ErrInvitationForbidden
		}
		if !requestingUser.IsSystem && existingInvitation.CreatorUserID != updatingUserID {
			return ErrInvitationForbidden
		}

		// c. Ana model güncelle
		existingInvitation.IsEnabled = isEnabled
		// UpdatedBy hook tarafından ayarlanacak

		// d. Detay model güncelle
		existingDetail := existingInvitation.Detail
		// Alanları kopyala
		existingDetail.Title = detailData.Title
		existingDetail.Description = detailData.Description
		// ... (tüm diğer InvitationDetail alanları) ...
		existingDetail.ShowGuestList = detailData.ShowGuestList

		// Şifre hashleme (eğer yeni şifre varsa)
		if detailData.PasswordHash != "" {
			// Eski hash ile karşılaştırmaya gerek yok, bcrypt farklı hash üretir.
			// Sadece uzunluk kontrolü yapabiliriz.
			// if len(detailData.PasswordHash) < 6 { return ErrPasswordTooShort } // Gerekirse
			hashedPasswordBytes, hashErr := bcrypt.GenerateFromPassword([]byte(detailData.PasswordHash), bcrypt.DefaultCost)
			if hashErr != nil {
				return ErrInvPasswordHashingFailed
			}
			existingDetail.PasswordHash = string(hashedPasswordBytes)
		} // Boşsa mevcut hash korunur.

		// e. Detail'i kaydet
		if err := invitationRepoTx.UpdateDetail(txCtx, &existingDetail); err != nil {
			return ErrInvitationUpdateFailed
		}
		// f. Invitation'ı kaydet
		if err := invitationRepoTx.Update(txCtx, &existingInvitation); err != nil {
			return ErrInvitationUpdateFailed
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("UpdateInvitation transaction failed", zap.Uint("id", id), zap.Uint("userID", updatingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Davetiye başarıyla güncellendi: ID %d (Güncelleyen: %d)", id, updatingUserID)
	return nil
}

// DeleteInvitation bir davetiyeyi ve ilişkili linkini siler.
func (s *InvitationService) DeleteInvitation(ctx context.Context, id uint, deletingUserID uint) error {
	if id == 0 || deletingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya silen kullanıcı ID", ErrInvInvalidInput)
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, deletingUserID)
		invitationRepoTx := repositories.NewInvitationRepositoryTx(tx)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli al ve yetki kontrolü
		var invitationToDelete models.Invitation
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Link").First(&invitationToDelete, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvitationNotFound
			}
			return err
		}

		requestingUser, userErr := userRepoTx.FindByID(txCtx, deletingUserID)
		if userErr != nil {
			return ErrInvitationForbidden
		}
		if !requestingUser.IsSystem && invitationToDelete.CreatorUserID != deletingUserID {
			return ErrInvitationForbidden
		}

		// b. İlişkili Link'i al
		linkToDelete := invitationToDelete.Link
		if linkToDelete.ID == 0 { /* Loglama? */
			return ErrInvLinkDeletionFailed
		} // Link olmalı

		// c. Önce Invitation'ı sil (Detail cascade ile silinir)
		// Repo Delete metodu context ve deletedByUserID almalı
		if err := invitationRepoTx.Delete(txCtx, &invitationToDelete, deletingUserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInvitationNotFound
			} // Zaten silinmişse
			return ErrInvitationDeletionFailed
		}

		// d. Sonra Link'i sil
		if err := linkRepoTx.Delete(txCtx, &linkToDelete, deletingUserID); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) { // Link zaten yoksa sorun değil
				return ErrInvLinkDeletionFailed
			}
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("DeleteInvitation transaction failed", zap.Uint("id", id), zap.Uint("userID", deletingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Davetiye ve ilişkili link başarıyla silindi: Invitation ID %d (Silen: %d)", id, deletingUserID)
	return nil
}

// GetInvitationCountForUser kullanıcıya ait davetiye sayısını alır.
func (s *InvitationService) GetInvitationCountForUser(ctx context.Context, creatorUserID uint) (int64, error) {
	count, err := s.repo.CountByUserID(ctx, creatorUserID)
	if err != nil {
		configslog.Log.Error("Kullanıcı davetiye sayısı alınırken hata", zap.Uint("creatorUserID", creatorUserID), zap.Error(err))
		return 0, err
	}
	return count, nil
}

// GetAllInvitationsCount tüm davetiyelerin sayısını alır (Admin için).
func (s *InvitationService) GetAllInvitationsCount(ctx context.Context) (int64, error) {
	count, err := s.repo.CountAll(ctx) // Repo'da CountAll metodu olmalı
	if err != nil {
		configslog.Log.Error("Tüm davetiye sayısı alınırken hata", zap.Error(err))
		return 0, err
	}
	return count, nil
}

var _ IInvitationService = (*InvitationService)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewInvitationRepositoryTx(tx *gorm.DB) repositories.IInvitationRepository {
	base := repositories.NewBaseRepository[models.Invitation](tx)
	// base.SetAllowedSortColumns(...)
	return &repositories.InvitationRepository{Db: tx, Base: base}
}
