package services

import (
	"context" // Context eklendi
	"errors"
	"fmt"
	"time"

	"davet.link/configs"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/queryparams"
	"davet.link/repositories"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt" // Şifre hashleme için
	"gorm.io/gorm"
	"gorm.io/gorm/clause" // Lock için
)

// AppointmentServiceError özel servis hataları
type AppointmentServiceError string

func (e AppointmentServiceError) Error() string { return string(e) }

// Hata sabitleri
const (
	ErrAppointmentNotFound         AppointmentServiceError = "randevu hizmeti bulunamadı"
	ErrAppointmentCreationFailed   AppointmentServiceError = "randevu hizmeti oluşturulamadı"
	ErrAppointmentUpdateFailed     AppointmentServiceError = "randevu hizmeti güncellenemedi"
	ErrAppointmentDeletionFailed   AppointmentServiceError = "randevu hizmeti silinemedi"
	ErrAppointmentForbidden        AppointmentServiceError = "bu işlem için yetkiniz yok"
	ErrAppInvalidInput             AppointmentServiceError = "geçersiz girdi verisi"
	ErrAppointmentNameRequired     AppointmentServiceError = "randevu hizmet adı zorunludur"
	ErrAppointmentDurationRequired AppointmentServiceError = "randevu süresi (dakika) pozitif bir sayı olmalıdır"
	ErrAppPasswordHashingFailed    AppointmentServiceError = "şifre oluşturulurken hata oluştu" // Yeni eklendi
	ErrAppPasswordUpdateFailed     AppointmentServiceError = "şifre güncellenirken hata oluştu" // Yeni eklendi
	// Genel hatalar (Link, Type, User)
	ErrAppGenericLinkError AppointmentServiceError = "link işlemi sırasında hata"
	ErrAppGenericTypeError AppointmentServiceError = "hizmet türü işlemi sırasında hata"
	ErrAppGenericUserError AppointmentServiceError = "kullanıcı işlemi sırasında hata"
)

// IAppointmentService randevu hizmeti işlemleri için arayüz.
type IAppointmentService interface {
	CreateAppointment(ctx context.Context, providerUserID uint, orgID *uint, detailData models.AppointmentDetail) (*models.Appointment, error) // Detail struct olarak
	GetAppointmentByID(ctx context.Context, id uint, requestingUserID uint) (*models.Appointment, error)
	GetAppointmentByKey(ctx context.Context, key string) (*models.Appointment, error)
	GetAppointmentsForUser(ctx context.Context, providerUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error)
	UpdateAppointment(ctx context.Context, id uint, updatingUserID uint, detailData models.AppointmentDetail, isEnabled bool) error
	DeleteAppointment(ctx context.Context, id uint, deletingUserID uint) error
	GetAppointmentCountForUser(ctx context.Context, providerUserID uint) (int64, error)
	// GetAllAppointmentsPaginated(ctx context.Context, params queryparams.ListParams) (*queryparams.PaginatedResult, error) // Admin için
}

// AppointmentService IAppointmentService arayüzünü uygular.
type AppointmentService struct {
	repo        repositories.IAppointmentRepository
	linkService ILinkService // DI ile verilmeli
	typeService ITypeService // DI ile verilmeli
	userService IUserService // DI ile verilmeli
	db          *gorm.DB     // Transaction için
}

// NewAppointmentService yeni bir AppointmentService örneği oluşturur.
func NewAppointmentService() IAppointmentService {
	// Gerçek uygulamada DI framework veya manuel injection kullanın
	return &AppointmentService{
		repo:        repositories.NewAppointmentRepository(),
		linkService: NewLinkService(),
		typeService: NewTypeService(),
		userService: NewUserService(),
		db:          configs.GetDB(),
	}
}

// --- Yardımcı Metodlar ---

// ValidateAppointmentDetail temel validasyonları yapar.
func ValidateAppointmentDetail(detail models.AppointmentDetail) error {
	if detail.Name == "" {
		return ErrAppointmentNameRequired
	}
	if detail.DurationMinutes <= 0 {
		return ErrAppointmentDurationRequired
	}
	if detail.BookingLeadTime < 0 {
		return fmt.Errorf("%w: rezervasyon öncesi süre negatif olamaz", ErrAppInvalidInput)
	}
	if detail.BookingHorizonDays <= 0 {
		return fmt.Errorf("%w: rezervasyon ufku pozitif olmalı", ErrAppInvalidInput)
	}
	if detail.BufferTimeBefore < 0 || detail.BufferTimeAfter < 0 {
		return fmt.Errorf("%w: tampon süreler negatif olamaz", ErrAppInvalidInput)
	}
	// TODO: Price format, Currency code, ColorCode format vb.
	return nil
}

// contextWithUserID (BaseModel hook'ları için).
func contextWithUserID(ctx context.Context, userID uint) context.Context {
	return context.WithValue(ctx, models.contextUserIDKey, userID)
}

// --- Servis Metodları ---

// CreateAppointment yeni bir randevu hizmeti, detayları ve linkini oluşturur.
func (s *AppointmentService) CreateAppointment(ctx context.Context, providerUserID uint, orgID *uint, detailData models.AppointmentDetail) (*models.Appointment, error) {
	// 1. Validasyon
	if err := ValidateAppointmentDetail(detailData); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrAppInvalidInput, err)
	}
	if providerUserID == 0 {
		return nil, fmt.Errorf("%w: Geçersiz sağlayıcı kullanıcı ID", ErrAppInvalidInput)
	}

	// 2. Type ID'sini al
	appointmentType, err := s.typeService.GetTypeByName(models.TypeNameAppointment)
	if err != nil {
		return nil, ErrAppGenericTypeError
	} // Servis zaten loglamış olmalı

	// 3. Transaction Başlat
	var createdAppointment *models.Appointment
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, providerUserID) // İşlemi yapan kullanıcı
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		appointmentRepoTx := repositories.NewAppointmentRepositoryTx(tx)

		// a. Link Oluştur (Service katmanındaki CreateLink'i kullanalım)
		// CreateLink zaten context ve creatorUserID alıyor olmalı
		link, err := s.linkService.CreateLink(txCtx, providerUserID, appointmentType.ID)
		if err != nil {
			// CreateLink hatayı loglamış olmalı
			return ErrAppGenericLinkError // Daha genel hata
		}

		// b. Appointment ve Detail Oluştur
		appointment := models.Appointment{
			LinkID:         link.ID,
			ProviderUserID: providerUserID,
			OrganizationID: orgID,
			IsEnabled:      true,
			Detail:         detailData, // Gelen Detail verisi
		}
		// Repo Create metodu context'i hook için kullanır
		if err := appointmentRepoTx.Create(txCtx, &appointment); err != nil {
			configslog.Log.Error("Appointment oluşturulurken transaction hatası", zap.Error(err))
			return ErrAppointmentCreationFailed
		}

		// c. Link'in TargetID'sini güncelle (Link servisini kullanalım)
		// UpdateLinkTarget context ve updatingUserID alıyor olmalı
		if err := s.linkService.UpdateLinkTarget(txCtx, providerUserID, link.ID, appointment.ID); err != nil {
			// UpdateLinkTarget hatayı loglamış olmalı
			return ErrAppGenericLinkError
		}

		// d. Başarılı: Oluşturulan kaydı döndürmek üzere hazırla
		//    Tekrar sorgulamaya gerek yok, nesneyi güncelleyelim.
		appointment.Link = *link // Oluşturulan linki ekle (Type bilgisi de olmalı)
		if appointment.Link.Type.ID == 0 {
			appointment.Link.Type = *appointmentType
		}
		createdAppointment = &appointment

		return nil // Commit
	})

	if txErr != nil {
		// Transaction rollback oldu, link oluşturulduysa onu silmeye gerek yok, işlem geri alındı.
		configslog.Log.Error("CreateAppointment transaction failed", zap.Error(txErr))
		return nil, txErr // Orijinal hatayı döndür
	}

	configslog.SLog.Infof("Randevu hizmeti başarıyla oluşturuldu: ID %d, Adı: %s, LinkKey: %s", createdAppointment.ID, createdAppointment.Detail.Name, createdAppointment.Link.Key)
	return createdAppointment, nil
}

// GetAppointmentByID belirli bir randevu hizmetini ID ve kullanıcı yetkisine göre getirir.
func (s *AppointmentService) GetAppointmentByID(ctx context.Context, id uint, requestingUserID uint) (*models.Appointment, error) {
	appointment, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrAppointmentNotFound
		}
		return nil, err // Repo zaten loglamış olmalı
	}

	// Yetki Kontrolü
	requestingUser, userErr := s.userService.GetUserByID(ctx, requestingUserID)
	if userErr != nil {
		return nil, ErrAppointmentForbidden
	} // İstek yapan kullanıcı bulunamadı
	if !requestingUser.IsSystem && appointment.ProviderUserID != requestingUserID {
		return nil, ErrAppointmentForbidden // Sahibi değil ve admin değil
	}

	return appointment, nil
}

// GetAppointmentByKey public link anahtarı ile randevu hizmetini getirir.
func (s *AppointmentService) GetAppointmentByKey(ctx context.Context, key string) (*models.Appointment, error) {
	if key == "" {
		return nil, ErrAppointmentNotFound
	}

	link, err := s.linkService.GetLinkByKey(ctx, key)
	if err != nil {
		if errors.Is(err, services.ErrLinkNotFound) {
			return nil, ErrAppointmentNotFound
		}
		return nil, err // Diğer link hataları
	}

	if link.Type.Name != models.TypeNameAppointment {
		return nil, ErrAppointmentNotFound
	}

	// TargetID ile randevu hizmetini bul (public olduğu için userID gerekmez)
	appointment, err := s.repo.FindByID(ctx, link.TargetID)
	if err != nil {
		if errors.Is(err, repositories.ErrNotFound) {
			return nil, ErrAppointmentNotFound
		}
		return nil, err
	}

	// Aktiflik ve süre kontrolü
	if !appointment.IsEnabled {
		return nil, ErrAppointmentNotFound
	}
	if appointment.Detail.ExpiresAt != nil && time.Now().UTC().After(*appointment.Detail.ExpiresAt) {
		return nil, ErrAppointmentNotFound // Süresi dolmuş
	}

	// Şifre kontrolü handler'da

	return appointment, nil
}

// GetAppointmentsForUser kullanıcıya (sağlayıcıya) ait randevu hizmetlerini sayfalayarak getirir.
func (s *AppointmentService) GetAppointmentsForUser(ctx context.Context, providerUserID uint, params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	if providerUserID == 0 {
		return nil, errors.New("geçersiz sağlayıcı kullanıcı ID")
	}
	// Parametre validasyonu
	params.Validate()

	appointments, totalCount, err := s.repo.FindAllByUserIDPaginated(ctx, providerUserID, params)
	if err != nil {
		configslog.Log.Error("Kullanıcı randevu hizmetleri alınırken hata", zap.Uint("providerUserID", providerUserID), zap.Error(err))
		return nil, err
	}

	totalPages := queryparams.CalculateTotalPages(totalCount, params.PerPage)
	result := &queryparams.PaginatedResult{
		Data: appointments,
		Meta: queryparams.PaginationMeta{
			CurrentPage: params.Page, PerPage: params.PerPage,
			TotalItems: totalCount, TotalPages: totalPages,
		},
	}
	return result, nil
}

// UpdateAppointment mevcut bir randevu hizmetini ve detaylarını günceller.
func (s *AppointmentService) UpdateAppointment(ctx context.Context, id uint, updatingUserID uint, detailData models.AppointmentDetail, isEnabled bool) error {
	// 1. Giriş validasyonu
	if err := ValidateAppointmentDetail(detailData); err != nil {
		return fmt.Errorf("%w: %v", ErrAppInvalidInput, err)
	}
	if id == 0 || updatingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya güncelleyen kullanıcı ID", ErrAppInvalidInput)
	}

	// 2. Transaction başlat
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, updatingUserID)
		appointmentRepoTx := repositories.NewAppointmentRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx) // Yetki için

		// a. Kaydı kilitli al
		var existingAppointment models.Appointment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Detail").First(&existingAppointment, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAppointmentNotFound
			}
			return err
		}

		// b. Yetki Kontrolü
		requestingUser, userErr := userRepoTx.FindByID(txCtx, updatingUserID)
		if userErr != nil {
			return ErrAppointmentForbidden
		}
		if !requestingUser.IsSystem && existingAppointment.ProviderUserID != updatingUserID {
			return ErrAppointmentForbidden
		}

		// c. Ana model güncelle
		existingAppointment.IsEnabled = isEnabled
		// UpdatedBy hook tarafından ayarlanacak

		// d. Detay model güncelle
		existingDetail := existingAppointment.Detail
		// ... (detailData'dan existingDetail'e tüm alanları kopyala) ...
		existingDetail.Name = detailData.Name
		existingDetail.Description = detailData.Description
		existingDetail.DurationMinutes = detailData.DurationMinutes
		// ... (diğer tüm AppointmentDetail alanları) ...
		existingDetail.ExpiresAt = detailData.ExpiresAt
		// Şifre hashleme (eğer değiştiyse)
		if detailData.PasswordHash != "" { // Formdan hashlenmemiş şifre gelmeli
			// Eski şifreyle aynı mı kontrolü yapılmamalı (zaten hashlenmemiş)
			if len(detailData.PasswordHash) < 6 {
				return ErrAuthPasswordTooShort
			} // Yeni kural
			hashedPasswordBytes, hashErr := bcrypt.GenerateFromPassword([]byte(detailData.PasswordHash), bcrypt.DefaultCost)
			if hashErr != nil {
				return ErrAppPasswordHashingFailed
			}
			existingDetail.PasswordHash = string(hashedPasswordBytes)
		} // Boş geldiyse mevcut hash korunur

		// e. Detail'i kaydet (repo metodu ile - context'i kullanır)
		if err := appointmentRepoTx.UpdateDetail(txCtx, &existingDetail); err != nil {
			return ErrAppointmentUpdateFailed
		}
		// f. Appointment'ı kaydet (repo metodu ile - context'i kullanır)
		if err := appointmentRepoTx.Update(txCtx, &existingAppointment); err != nil {
			return ErrAppointmentUpdateFailed
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("UpdateAppointment transaction failed", zap.Uint("id", id), zap.Uint("userID", updatingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Randevu hizmeti başarıyla güncellendi: ID %d (Güncelleyen: %d)", id, updatingUserID)
	return nil
}

// DeleteAppointment bir randevu hizmetini ve ilişkili linkini siler.
func (s *AppointmentService) DeleteAppointment(ctx context.Context, id uint, deletingUserID uint) error {
	if id == 0 || deletingUserID == 0 {
		return fmt.Errorf("%w: Geçersiz ID veya silen kullanıcı ID", ErrAppInvalidInput)
	}

	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		txCtx := contextWithUserID(ctx, deletingUserID)
		appointmentRepoTx := repositories.NewAppointmentRepositoryTx(tx)
		linkRepoTx := repositories.NewLinkRepositoryTx(tx)
		userRepoTx := repositories.NewUserRepositoryTx(tx)

		// a. Kaydı kilitli al ve yetki kontrolü yap
		var appointmentToDelete models.Appointment
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Preload("Link").First(&appointmentToDelete, id).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAppointmentNotFound
			}
			return err
		}

		requestingUser, userErr := userRepoTx.FindByID(txCtx, deletingUserID)
		if userErr != nil {
			return ErrAppointmentForbidden
		}
		if !requestingUser.IsSystem && appointmentToDelete.ProviderUserID != deletingUserID {
			return ErrAppointmentForbidden
		}

		// b. İlişkili Link'i al
		linkToDelete := appointmentToDelete.Link
		if linkToDelete.ID == 0 { /* Loglama? Devam et? */
			configslog.Log.Warn("DeleteAppointment: İlişkili link bulunamadı.", zap.Uint("appointmentID", id))
		}

		// c. Appointment'ı sil (repo Delete metodu context ve userID alır)
		// Repo'nun Delete'i DeletedBy'ı ayarlamalı.
		if err := appointmentRepoTx.Delete(txCtx, &appointmentToDelete, deletingUserID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAppointmentNotFound
			} // Zaten silinmişse
			return ErrAppointmentDeletionFailed
		}

		// d. Link'i sil (eğer varsa)
		if linkToDelete.ID != 0 {
			if err := linkRepoTx.Delete(txCtx, &linkToDelete, deletingUserID); err != nil {
				// Link silinemezse ne yapmalı? Transaction geri alınacak.
				return ErrAppGenericLinkError
			}
		}

		return nil // Commit
	})

	if txErr != nil {
		configslog.Log.Error("DeleteAppointment transaction failed", zap.Uint("id", id), zap.Uint("userID", deletingUserID), zap.Error(txErr))
		return txErr
	}
	configslog.SLog.Infof("Randevu hizmeti ve ilişkili link başarıyla silindi: Appointment ID %d (Silen: %d)", id, deletingUserID)
	return nil
}

// GetAppointmentCountForUser kullanıcıya (sağlayıcıya) ait randevu hizmeti sayısını alır.
func (s *AppointmentService) GetAppointmentCountForUser(ctx context.Context, providerUserID uint) (int64, error) {
	count, err := s.repo.CountByUserID(ctx, providerUserID) // Repo metodu context almalı
	if err != nil {
		configslog.Log.Error("Kullanıcı randevu hizmeti sayısı alınırken hata", zap.Uint("providerUserID", providerUserID), zap.Error(err))
		return 0, err
	}
	return count, nil
}

var _ IAppointmentService = (*AppointmentService)(nil)
