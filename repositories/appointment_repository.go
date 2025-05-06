package repositories

import (
	"context" // Context parametresi için
	"errors"
	"strings"
	"time"

	"davet.link/configs"            // DB bağlantısı için
	"davet.link/configs/configslog" // Loglama için
	"davet.link/models"
	"davet.link/pkg/queryparams"   // Pagination için
	"davet.link/pkg/turkishsearch" // Türkçe arama için

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IAppointmentRepository randevu hizmeti veritabanı işlemleri için arayüz.
type IAppointmentRepository interface {
	Create(ctx context.Context, appointment *models.Appointment) error
	FindByID(ctx context.Context, id uint) (*models.Appointment, error)
	FindByLinkID(ctx context.Context, linkID uint) (*models.Appointment, error)
	FindAllByUserIDPaginated(ctx context.Context, providerUserID uint, params queryparams.ListParams) ([]models.Appointment, int64, error)
	Update(ctx context.Context, appointment *models.Appointment) error        // Ana modeli Save ile güncelle
	UpdateDetail(ctx context.Context, detail *models.AppointmentDetail) error // Detay modelini Save ile güncelle
	Delete(ctx context.Context, appointment *models.Appointment, deletedByUserID uint) error
	CountByUserID(ctx context.Context, providerUserID uint) (int64, error)
	// GetAllPaginated(ctx context.Context, params queryparams.ListParams) ([]models.Appointment, int64, error) // Admin için (gerekirse)
}

// AppointmentRepository IAppointmentRepository arayüzünü uygular.
type AppointmentRepository struct {
	db   *gorm.DB
	base IBaseRepository[models.Appointment] // Generik base repo (opsiyonel, direkt db kullanılabilir)
}

// NewAppointmentRepository yeni bir AppointmentRepository örneği oluşturur.
func NewAppointmentRepository() IAppointmentRepository {
	db := configs.GetDB()
	// Base repository'yi burada başlatabilir veya direkt db kullanabiliriz.
	// BaseRepository kullanımı standart CRUD işlemlerinde tekrarı azaltır.
	base := NewBaseRepository[models.Appointment](db)
	base.SetAllowedSortColumns([]string{
		"id", "created_at", "is_enabled", // appointments tablosu
		// Detaydan sıralama için GetAll metodunda özel handle gerekir
		// "name", "duration_minutes", "price",
	})
	return &AppointmentRepository{db: db, base: base}
}

// Context ile çalışan DB örneği döndüren yardımcı fonksiyon
func (r *AppointmentRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok && tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// Create yeni bir randevu hizmeti ve detayını oluşturur.
func (r *AppointmentRepository) Create(ctx context.Context, appointment *models.Appointment) error {
	if appointment == nil || appointment.LinkID == 0 {
		return errors.New("geçersiz veya eksik link bilgisi olan randevu hizmeti oluşturulamaz")
	}
	// Context'i kullanarak Create çağır (BeforeCreate hook'u için)
	return r.getDB(ctx).Create(appointment).Error
}

// FindByID belirli bir ID'ye sahip randevu hizmetini bulur.
func (r *AppointmentRepository) FindByID(ctx context.Context, id uint) (*models.Appointment, error) {
	if id == 0 {
		return nil, errors.New("geçersiz Appointment ID")
	}
	var appointment models.Appointment
	// İlişkili Detail ve Link.Type'ı da yükle
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").First(&appointment, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		} // Base repo hatası
		configslog.Log.Error("AppointmentRepository.FindByID: DB error", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}
	return &appointment, nil
}

// FindByLinkID belirli bir Link ID'sine sahip randevu hizmetini bulur.
func (r *AppointmentRepository) FindByLinkID(ctx context.Context, linkID uint) (*models.Appointment, error) {
	if linkID == 0 {
		return nil, errors.New("geçersiz Link ID")
	}
	var appointment models.Appointment
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").Where("link_id = ?", linkID).First(&appointment).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		configslog.Log.Error("AppointmentRepository.FindByLinkID: DB error", zap.Uint("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return &appointment, nil
}

// FindAllByUserIDPaginated belirli bir sağlayıcıya ait randevu hizmetlerini sayfalayarak bulur.
func (r *AppointmentRepository) FindAllByUserIDPaginated(ctx context.Context, providerUserID uint, params queryparams.ListParams) ([]models.Appointment, int64, error) {
	if providerUserID == 0 {
		return nil, 0, errors.New("geçersiz Provider User ID")
	}
	var appointments []models.Appointment
	var totalCount int64
	db := r.getDB(ctx)

	query := db.Model(&models.Appointment{}).Where("provider_user_id = ?", providerUserID)

	// İsim filtresi (Detail tablosundaki Name'e göre)
	needsJoin := false // JOIN gerekip gerekmediğini takip et
	if params.Name != "" {
		// query = query.Joins("JOIN appointment_details ON appointment_details.appointment_id = appointments.id").
		// 	Where("unaccent(lower(appointment_details.name)) ILIKE unaccent(?)", "%"+params.Name+"%")
		sqlFragment, args := turkishsearch.SQLFilter("appointment_details.name", params.Name)
		query = query.Joins("JOIN appointment_details ON appointment_details.appointment_id = appointments.id").Where(sqlFragment, args...)
		needsJoin = true
	}
	// Status filtresi (Ana tablodan)
	if params.Status != "" {
		statusBool := params.Status == "true"                          // String'i boolean'a çevir
		query = query.Where("appointments.is_enabled = ?", statusBool) // Tablo adını belirt
	}

	// Filtrelenmiş toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		configslog.Log.Error("AppointmentRepository.Count (Paginated): DB error", zap.Uint("providerUserID", providerUserID), zap.Error(err))
		return nil, 0, err
	}

	if totalCount == 0 {
		return appointments, 0, nil
	}

	// Sıralama
	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}

	// İzin verilen sıralama sütunları (tablo adlarıyla birlikte)
	allowedSortColumns := map[string]string{
		"id":               "appointments.id",
		"created_at":       "appointments.created_at",
		"is_enabled":       "appointments.is_enabled",
		"name":             "appointment_details.name",
		"duration_minutes": "appointment_details.duration_minutes",
		"price":            "appointment_details.price",
	}
	orderColumn := "appointments.created_at" // Varsayılan
	if dbCol, ok := allowedSortColumns[sortBy]; ok {
		orderColumn = dbCol
		// Eğer Detail sütununa göre sıralama yapılıyorsa ve JOIN zaten yapılmadıysa, JOIN ekle
		if (sortBy == "name" || sortBy == "duration_minutes" || sortBy == "price") && !needsJoin {
			query = query.Joins("JOIN appointment_details ON appointment_details.appointment_id = appointments.id")
		}
	} else {
		configslog.SLog.Warn("Geçersiz Appointment sıralama alanı istendi, varsayılan kullanılıyor.", zap.String("requestedSortBy", sortBy))
		sortBy = "created_at"
		orderColumn = "appointments.created_at"
	}
	query = query.Order(orderColumn + " " + orderBy)

	// İlişkili verileri preload et
	query = query.Preload("Detail").Preload("Link.Type")

	// Sayfalama
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)

	// Verileri çek (JOIN varsa Select ile ana tabloyu alabiliriz)
	// if needsJoin { query = query.Select("appointments.*") } // Gerekirse
	err = query.Find(&appointments).Error
	if err != nil {
		configslog.Log.Error("AppointmentRepository.Find (Paginated): DB error", zap.Uint("providerUserID", providerUserID), zap.Error(err))
		return nil, totalCount, err
	}

	// JOIN yapıldığında GORM Detail'i otomatik yüklemeyebilir, manuel yükleme (opsiyonel)
	if needsJoin && len(appointments) > 0 { /* ... GetAllCards'daki gibi detay yükleme ... */
	}

	return appointments, totalCount, nil
}

// Update sadece ana Appointment modelini günceller (Save ile).
// BeforeUpdate hook'u context'ten UpdatedBy alır.
func (r *AppointmentRepository) Update(ctx context.Context, appointment *models.Appointment) error {
	if appointment == nil || appointment.ID == 0 {
		return errors.New("güncellenecek randevu hizmeti geçerli değil")
	}
	return r.getDB(ctx).Save(appointment).Error
}

// UpdateDetail sadece AppointmentDetail modelini günceller (Save ile).
// BeforeUpdate hook'u context'ten UpdatedBy alır.
func (r *AppointmentRepository) UpdateDetail(ctx context.Context, detail *models.AppointmentDetail) error {
	if detail == nil || detail.ID == 0 {
		return errors.New("güncellenecek randevu hizmeti detayı geçerli değil")
	}
	return r.getDB(ctx).Save(detail).Error
}

// Delete randevu hizmetini siler (soft delete).
// Context'ten userID alıp DeletedBy'ı ayarlar.
func (r *AppointmentRepository) Delete(ctx context.Context, appointment *models.Appointment, deletedByUserID uint) error {
	if appointment == nil || appointment.ID == 0 {
		return errors.New("silinecek randevu hizmeti geçerli değil")
	}
	db := r.getDB(ctx) // Context'li DB al

	// Transaction içinde DeletedBy'ı ayarla ve soft delete yap
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updateData := map[string]interface{}{"deleted_at": now, "deleted_by": &deletedByUserID}

		// Önce Update ile DeletedBy ve DeletedAt'ı set et
		result := tx.Model(appointment).Where("id = ? AND deleted_at IS NULL", appointment.ID).Updates(updateData)
		if result.Error != nil {
			configslog.Log.Error("AppointmentRepository.Delete: Update sırasında hata", zap.Uint("id", appointment.ID), zap.Error(result.Error))
			return result.Error
		}
		if result.RowsAffected == 0 {
			// Zaten silinmiş veya bulunamadı
			var exists int64
			countErr := tx.Model(&models.Appointment{}).Where("id = ?", appointment.ID).Count(&exists).Error
			if countErr == nil && exists == 0 {
				return gorm.ErrRecordNotFound
			} // Gerçekten yok
			// Zaten silinmişse hata döndürme? Veya özel hata? Şimdilik NotFound.
			configslog.Log.Warn("AppointmentRepository.Delete: Kayıt bulunamadı veya zaten silinmiş.", zap.Uint("id", appointment.ID))
			return gorm.ErrRecordNotFound
		}
		// GORM'un kendi Delete'ini çağırmaya gerek yok, Updates yeterli.
		return nil
	})
}

// CountByUserID belirli bir sağlayıcıya ait randevu hizmeti sayısını döndürür.
func (r *AppointmentRepository) CountByUserID(ctx context.Context, providerUserID uint) (int64, error) {
	if providerUserID == 0 {
		return 0, errors.New("geçersiz Provider User ID")
	}
	var count int64
	err := r.getDB(ctx).Model(&models.Appointment{}).Where("provider_user_id = ?", providerUserID).Count(&count).Error
	return count, err
}

var _ IAppointmentRepository = (*AppointmentRepository)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewAppointmentRepositoryTx(tx *gorm.DB) IAppointmentRepository {
	base := NewBaseRepository[models.Appointment](tx)
	base.SetAllowedSortColumns([]string{"id", "created_at", "is_enabled"}) // Tekrar set edilebilir
	return &AppointmentRepository{db: tx, base: base}
}
