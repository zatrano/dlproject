package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"davet.link/configs"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/queryparams"
	"davet.link/repositories"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt" // Şifre hashleme
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// FormServiceError özel servis hataları
type FormServiceError string

func (e FormServiceError) Error() string { return string(e) }

const (
	ErrFormNotFound             FormServiceError = "form bulunamadı"
	ErrFormCreationFailed       FormServiceError = "form oluşturulamadı"
	ErrFormUpdateFailed         FormServiceError = "form güncellenemedi"
	ErrFormDeletionFailed       FormServiceError = "form silinemedi"
	ErrFormForbidden            FormServiceError = "bu işlem için yetkiniz yok"
	ErrFrmInvalidInput          FormServiceError = "geçersiz girdi verisi"
	ErrFormTitleRequired        FormServiceError = "form başlığı zorunludur"
	ErrFrmLinkCreationFailed    FormServiceError = "form için link oluşturulamadı"
	ErrFrmTypeNotFound          FormServiceError = "form hizmet türü bulunamadı"
	ErrFrmLinkUpdateFailed      FormServiceError = "form linki güncellenemedi"
	ErrFrmLinkDeletionFailed    FormServiceError = "form linki silinemedi"
	ErrFrmPasswordHashingFailed FormServiceError = "form şifresi oluşturulamadı"
)

// IFormService form işlemleri için arayüz.
type IFormService interface {
	CreateForm(ctx context.Context, creatorUserID uint, orgID *uint, detailData models.FormDetail) (*models.Form, error)
	GetFormByID(ctx context.Context, id uint, requestingUserID uint) (*models.Form, error)
	GetFormByKey(ctx context.Context, key string) (*models.Form, error) // Public erişim
	GetFormsForUser(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error)
	GetAllFormsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) // Admin için
	UpdateForm(ctx context.Context, id uint, updatingUserID uint, detailData models.FormDetail, isEnabled bool) error
	DeleteForm(ctx context.Context, id uint, deletingUserID uint) error
	GetFormCountForUser(ctx context.Context, creatorUserID uint) (int64, error)
	GetAllFormsCount(ctx context.Context) (int64, error) // Admin için
	// TODO: SubmitForm, GetFormSubmissions gibi metodlar eklenecek
}

// FormService IFormService arayüzünü uygular.
type FormService struct {
	repo        repositories.IFormRepository
	linkService ILinkService
	typeService ITypeService
	userService IUserService
	db          *gorm.DB
}

// NewFormService yeni bir FormService örneği oluşturur (DI ile).
func NewFormService() IFormService {
	return &FormService{
		repo:        repositories.NewFormRepository(),
		linkService: NewLinkService(),
		typeService: NewTypeService(),
		userService: NewUserService(),
		db:          configs.GetDB(),
	}
}

// --- Yardımcı Metodlar ---

// ValidateFormDetail temel validasyonları yapar.
func ValidateFormDetail(detail models.FormDetail) error {
	if detail.Title == "" {
		return ErrFormTitleRequired
	}
	if detail.SubmissionLimit != nil && *detail.SubmissionLimit < 0 {
		return fmt.Errorf("%w: gönderim limiti negatif olamaz", ErrFrmInvalidInput)
	}
	if detail.LimitPerUser != nil && *detail.LimitPerUser < 0 {
		return fmt.Errorf("%w: kullanıcı başına limit negatif olamaz", ErrFrmInvalidInput)
	}
	// TODO: RedirectURL, NotifyOnSubmitEmail formatları
	return nil
}

// contextWithUserID (önceki gibi)
func contextWithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, models.contextUserIDKey, userID)
}

// --- Servis Metodları ---

// CreateForm yeni bir form, detayları ve linkini oluşturur.
func (s *FormService) CreateForm(ctx context.Context, creatorUserID uint, orgID *uint, detailData models.FormDetail) (*models.Form, error) {
	if err := ValidateFormDetail(detailData); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrFrmInvalidInput, err)
	}
	if creatorUserID == 0 {
		return nil, fmt.Errorf("%w: Geçersiz oluşturan kullanıcı ID", ErrFrmInvalidInput)
	}

	formType, err := s.typeService.GetTypeByName(models.TypeNameForm)
	if err != nil {
		return nil, ErrFrmTypeNotFound
	}

	// Şifre hashleme
	if detailData.PasswordHash != "" {
		hashedPasswordBytes, hashErr := bcrypt.GenerateFromPassword([]byte(detailData.PasswordHash), bcrypt.DefaultCost)
		if hashErr != nil {
			return nil, ErrFrmPasswordHashingFailed
		}
		detailData.PasswordHash = string(hashedPasswordBytes)
	}

	var createdForm *models.Form
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, creatorUserID)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		formRepoTx := repositories.NewFormRepositoryTx(tx)

		// a. Link Oluştur
		link, err := s.linkService.CreateLink(txCtx, creatorUserID, formType.ID)
		if err != nil {
			return ErrFrmLinkCreationFailed
		}

		// b. Form ve Detail Oluştur
		form := models.Form{
			LinkID:         link.ID,
			CreatorUserID:  creatorUserID,
			OrganizationID: orgID,
			IsEnabled:      true,
			Detail:         detailData,
		}
		if err := formRepoTx.Create(txCtx, &form); err != nil {
			return ErrFormCreationFailed
		}

		// c. Link TargetID Güncelle
		updateData := map[string]interface{}{"target_id": form.ID}
		if err := linkRepoTx.Update(txCtx, link.ID, updateData, creatorUserID); err != nil {
			return ErrFrmLinkUpdateFailed
		}

		// d. Sonucu hazırla
		form.Link = *link
		if form.Link.Type.ID == 0 {
			form.Link.Type = *formType
		}
		createdForm = &form
		return nil
	})

	if txErr != nil {
		return nil, txErr
	}
	configslog.SLog.Infof("Form başarıyla oluşturuldu: ID %d, Başlık: %s, LinkKey: %s", createdForm.ID, createdForm.Detail.Title, createdForm.Link.Key)
	return createdForm, nil
}

// GetFormByID belirli bir formu ID ve kullanıcı yetkisine göre getirir.
func (s *FormService) GetFormByID(ctx context.Context, id uint, requestingUserID uint) (*models.Form, error) {
	form, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrFormNotFound
		}
		return nil, err
	}

	// Yetki Kontrolü
	requestingUser, userErr := s.userService.GetUserByID(ctx, requestingUserID)
	if userErr != nil {
		return nil, ErrFormForbidden
	}
	if !requestingUser.IsSystem && form.CreatorUserID != requestingUserID {
		return nil, ErrFormForbidden
	}

	return form, nil
}

// GetFormByKey public link anahtarı ile formu getirir.
func (s *FormService) GetFormByKey(ctx context.Context, key string) (*models.Form, error) {
	if key == "" {
		return nil, ErrFormNotFound
	}

	link, err := s.linkService.GetLinkByKey(ctx, key)
	if err != nil { /* ... NotFound ... */
		return nil, ErrFormNotFound
	}
	if link.Type.Name != models.TypeNameForm {
		return nil, ErrFormNotFound
	}

	form, err := s.repo.FindByID(ctx, link.TargetID)
	if err != nil { /* ... NotFound / Tutarsızlık ... */
		return nil, err
	}

	if !form.IsEnabled {
		return nil, ErrFormNotFound
	}
	if form.Detail.ClosesAt != nil && time.Now().UTC().After(*form.Detail.ClosesAt) {
		return nil, ErrFormNotFound
	} // Veya ErrFormClosed

	// TODO: Şifre kontrolü handler'da
	// TODO: FieldDefinitions Preload edilmeli (repo'da veya burada)

	return form, nil
}

// GetFormsForUser kullanıcıya ait formları sayfalayarak getirir.
func (s *FormService) GetFormsForUser(ctx context.Context, creatorUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	if creatorUserID == 0 {
		return nil, errors.New("geçersiz kullanıcı ID")
	}
	params.Validate()

	forms, totalCount, err := s.repo.FindAllByUserIDPaginated(ctx, creatorUserID, params)
	if err != nil {
		return nil, err
	}

	totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	result := &queryparams.PaginatedResult{
		Data: forms,
		Meta: queryparams.PaginationMeta{ /* ... */ },
	}
	return result, nil
}

// GetAllFormsPaginated tüm formları sayfalayarak getirir (Admin için).
func (s *FormService) GetAllFormsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	params.Validate()
	// TODO: Repository'ye FindAllPaginated metodu eklenmeli
	// forms, totalCount, err := s.repo.FindAllPaginated(ctx, params)
	// if err != nil { return nil, err }
	// totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	// result := &queryparams.PaginatedResult{ Data: forms, Meta: queryparams.PaginationMeta{...} }
	// return result, nil
	return nil, errors.New("GetAllFormsPaginated henüz implemente edilmedi")
}

// UpdateForm mevcut bir formu ve detaylarını günceller.
func (s *FormService) UpdateForm(ctx context.Context, id uint, updatingUserID uint, detailData models.FormDetail, isEnabled bool) error {
	// 1. Validasyon
	if err := ValidateFormDetail(detailData); err != nil {
		return fmt.Errorf("%w: %v", ErrFrmInvalidInput, err)
	}
	if id == 0 || updatingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya güncelleyen kullanıcı ID", ErrFrmInvalidInput)
	}

	// 2. Şifre hashleme (eğer varsa)
	if detailData.PasswordHash != "" {
		hashedPasswordBytes, hashErr := bcrypt.GenerateFromPassword([]byte(detailData.PasswordHash), bcrypt.DefaultCost)
		if hashErr != nil {
			return ErrFrmPasswordHashingFailed
		}
		detailData.PasswordHash = string(hashedPasswordBytes)
	}

	// 3. Transaction
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, updatingUserID)
		formRepoTx := repositories.NewFormRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli al
		var existingForm models.Form
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Detail").First(&existingForm, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFormNotFound
			}
			return err
		}

		// b. Yetki Kontrolü
		requestingUser, userErr := userRepoTx.FindByID(txCtx, updatingUserID)
		if userErr != nil {
			return ErrFormForbidden
		}
		if !requestingUser.IsSystem && existingForm.CreatorUserID != updatingUserID {
			return ErrFormForbidden
		}

		// c. Ana model güncelle
		existingForm.IsEnabled = isEnabled

		// d. Detay model güncelle
		existingDetail := existingForm.Detail
		// ... (detailData'dan existingDetail'e tüm alanları kopyala) ...
		existingDetail.Title = detailData.Title
		existingDetail.Description = detailData.Description
		// ... (SubmissionLimit, ClosesAt, vb.) ...
		// Şifre: Eğer detailData'da hashlenmiş varsa onu kullan, yoksa mevcutu koru.
		if detailData.PasswordHash != "" {
			existingDetail.PasswordHash = detailData.PasswordHash
		}

		// e. Detail'i kaydet
		if err := formRepoTx.UpdateDetail(txCtx, &existingDetail); err != nil {
			return ErrFormUpdateFailed
		}
		// f. Form'u kaydet
		if err := formRepoTx.Update(txCtx, &existingForm); err != nil {
			return ErrFormUpdateFailed
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("UpdateForm transaction failed", zap.Uint("id", id), zap.Uint("userID", updatingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Form başarıyla güncellendi: ID %d (Güncelleyen: %d)", id, updatingUserID)
	return nil
}

// DeleteForm bir formu ve ilişkili linkini siler.
func (s *FormService) DeleteForm(ctx context.Context, id uint, deletingUserID uint) error {
	if id == 0 || deletingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya silen kullanıcı ID", ErrFrmInvalidInput)
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, deletingUserID)
		formRepoTx := repositories.NewFormRepositoryTx(tx)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli al ve yetki kontrolü yap
		var formToDelete models.Form
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Link").First(&formToDelete, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFormNotFound
			}
			return err
		}

		requestingUser, userErr := userRepoTx.FindByID(txCtx, deletingUserID)
		if userErr != nil {
			return ErrFormForbidden
		}
		if !requestingUser.IsSystem && formToDelete.CreatorUserID != deletingUserID {
			return ErrFormForbidden
		}

		// b. İlişkili Link'i al
		linkToDelete := formToDelete.Link
		if linkToDelete.ID == 0 { /* Loglama? Hata? */
			return ErrFrmLinkDeletionFailed
		}

		// c. Formu sil (repo Delete metodu context ve userID alır)
		// Repo Delete metodu ilişkili Detail, FieldDefinitions, Submissions'ı da silmeli (veya burada manuel yapılmalı)
		if err := formRepoTx.Delete(txCtx, &formToDelete, deletingUserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrFormNotFound
			}
			return ErrFormDeletionFailed
		}

		// d. Link'i sil
		if err := linkRepoTx.Delete(txCtx, &linkToDelete, deletingUserID); err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) { // Zaten yoksa sorun değil
				return ErrFrmLinkDeletionFailed
			}
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("DeleteForm transaction failed", zap.Uint("id", id), zap.Uint("userID", deletingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Form ve ilişkili link başarıyla silindi: Form ID %d (Silen: %d)", id, deletingUserID)
	return nil
}

// GetFormCountForUser kullanıcıya ait form sayısını alır.
func (s *FormService) GetFormCountForUser(ctx context.Context, creatorUserID uint) (int64, error) {
	count, err := s.repo.CountByUserID(ctx, creatorUserID)
	if err != nil {
		return 0, err
	} // Repo loglar
	return count, nil
}

// GetAllFormsCount tüm formların sayısını alır (Admin için).
func (s *FormService) GetAllFormsCount(ctx context.Context) (int64, error) {
	count, err := s.repo.CountAll(ctx) // Repo'da CountAll metodu olmalı
	if err != nil {
		configslog.Log.Error("Tüm form sayısı alınırken hata", zap.Error(err))
		return 0, err
	}
	return count, nil
}

var _ IFormService = (*FormService)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewFormRepositoryTx(tx *gorm.DB) repositories.IFormRepository {
	base := repositories.NewBaseRepository[models.Form](tx)
	// base.SetAllowedSortColumns(...)
	return &repositories.FormRepository{Db: tx, Base: base}
}
