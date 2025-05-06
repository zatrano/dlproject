package repositories

import (
	"context" // Context parametresi için
	"errors"
	"strings"
	"time" // Delete metodu için

	"davet.link/configs"            // DB bağlantısı için
	"davet.link/configs/configslog" // Loglama için
	"davet.link/models"
	"davet.link/pkg/queryparams"   // Pagination için
	"davet.link/pkg/turkishsearch" // Türkçe arama için

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IFormRepository form veritabanı işlemleri için arayüz.
type IFormRepository interface {
	Create(ctx context.Context, form *models.Form) error
	FindByID(ctx context.Context, id uint) (*models.Form, error)
	FindByLinkID(ctx context.Context, linkID uint) (*models.Form, error)
	FindAllByUserIDPaginated(ctx context.Context, userID uint, params queryparams.ListParams) ([]models.Form, int64, error) // CreatorUserID'ye göre
	FindAllPaginated(ctx context.Context, params queryparams.ListParams) ([]models.Form, int64, error)                      // Admin için tümü
	Update(ctx context.Context, form *models.Form) error                                                                    // Ana model
	UpdateDetail(ctx context.Context, detail *models.FormDetail) error                                                      // Detay model
	Delete(ctx context.Context, form *models.Form, deletedByUserID uint) error
	CountByUserID(ctx context.Context, userID uint) (int64, error) // CreatorUserID'ye göre
	CountAll(ctx context.Context) (int64, error)                   // Admin için tümü
	// TODO: İlişkili FormFieldDefinition ve FormSubmission için metodlar eklenebilir.
}

// FormRepository IFormRepository arayüzünü uygular.
type FormRepository struct {
	db   *gorm.DB
	base IBaseRepository[models.Form] // Generik base repo (opsiyonel)
}

// NewFormRepository yeni bir FormRepository örneği oluşturur.
func NewFormRepository() IFormRepository {
	db := configs.GetDB()
	base := NewBaseRepository[models.Form](db)
	// Form için izin verilen sıralama sütunları
	base.SetAllowedSortColumns([]string{
		"id", "created_at", "is_enabled", // Ana tablo
		// "title", "closes_at", // Detail alanları (GetAll metodunda özel handle)
	})
	return &FormRepository{db: db, base: base}
}

// Context ile çalışan DB örneği döndüren yardımcı fonksiyon
func (r *FormRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok && tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// Create yeni bir form ve detayını oluşturur.
func (r *FormRepository) Create(ctx context.Context, form *models.Form) error {
	if form == nil || form.LinkID == 0 {
		return errors.New("geçersiz veya eksik link bilgisi olan form oluşturulamaz")
	}
	return r.getDB(ctx).Create(form).Error // BeforeCreate hook çalışır
}

// FindByID belirli bir ID'ye sahip formu bulur.
func (r *FormRepository) FindByID(ctx context.Context, id uint) (*models.Form, error) {
	if id == 0 {
		return nil, errors.New("geçersiz Form ID")
	}
	var form models.Form
	// TODO: FieldDefinitions ve Submissions gerekirse Preload edilebilir.
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").First(&form, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		} // Base repo hatası
		configslog.Log.Error("FormRepository.FindByID: DB error", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}
	return &form, nil
}

// FindByLinkID belirli bir Link ID'sine sahip formu bulur.
func (r *FormRepository) FindByLinkID(ctx context.Context, linkID uint) (*models.Form, error) {
	if linkID == 0 {
		return nil, errors.New("geçersiz Link ID")
	}
	var form models.Form
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").Where("link_id = ?", linkID).First(&form).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		configslog.Log.Error("FormRepository.FindByLinkID: DB error", zap.Uint("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return &form, nil
}

// applyFormFilters ortak filtreleme ve sıralama mantığını uygular.
func (r *FormRepository) applyFormFilters(db *gorm.DB, params queryparams.ListParams) (*gorm.DB, bool) {
	query := db
	needsJoin := false

	// Başlık filtresi
	if params.Name != "" {
		sqlFragment, args := turkishsearch.SQLFilter("form_details.title", params.Name)
		query = query.Joins("JOIN form_details ON form_details.form_id = forms.id").Where(sqlFragment, args...)
		needsJoin = true
	}
	// Status filtresi
	if params.Status != "" {
		statusBool := params.Status == "true"
		query = query.Where("forms.is_enabled = ?", statusBool)
	}

	// Sıralama
	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}

	allowedSortColumns := map[string]string{
		"id":         "forms.id",
		"created_at": "forms.created_at",
		"is_enabled": "forms.is_enabled",
		"title":      "form_details.title",
		"closes_at":  "form_details.closes_at",
	}
	orderColumn := "forms.created_at" // Varsayılan
	if dbCol, ok := allowedSortColumns[sortBy]; ok {
		orderColumn = dbCol
		if (sortBy == "title" || sortBy == "closes_at") && !needsJoin {
			query = query.Joins("JOIN form_details ON form_details.form_id = forms.id")
		}
	} else {
		configslog.SLog.Warn("Geçersiz Form sıralama alanı istendi, varsayılan kullanılıyor.", zap.String("requestedSortBy", sortBy))
		sortBy = "created_at"
		orderColumn = "forms.created_at"
	}
	query = query.Order(orderColumn + " " + orderBy)

	return query, needsJoin
}

// FindAllByUserIDPaginated belirli bir kullanıcıya ait formları sayfalayarak bulur.
func (r *FormRepository) FindAllByUserIDPaginated(ctx context.Context, creatorUserID uint, params queryparams.ListParams) ([]models.Form, int64, error) {
	if creatorUserID == 0 {
		return nil, 0, errors.New("geçersiz Creator User ID")
	}
	var forms []models.Form
	var totalCount int64
	db := r.getDB(ctx)

	query := db.Model(&models.Form{}).Where("forms.creator_user_id = ?", creatorUserID) // Ana tablo filtresi

	// Ortak filtreleri ve sıralamayı uygula
	query, needsJoin := r.applyFormFilters(query, params)

	// Toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		configslog.Log.Error("FormRepository.Count (Paginated by User): DB error", zap.Uint("creatorUserID", creatorUserID), zap.Error(err))
		return nil, 0, err
	}

	if totalCount == 0 {
		return forms, 0, nil
	}

	// Preload, Sayfalama, Find
	query = query.Preload("Detail").Preload("Link.Type")
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)
	if needsJoin {
		query = query.Select("forms.*")
	} // JOIN varsa ana tabloyu seç
	err = query.Find(&forms).Error
	if err != nil {
		configslog.Log.Error("FormRepository.Find (Paginated by User): DB error", zap.Uint("creatorUserID", creatorUserID), zap.Error(err))
		return nil, totalCount, err
	}

	// Detayları manuel yükle (eğer JOIN sonrası yüklenmediyse)
	if needsJoin && len(forms) > 0 { /* ... Detay yükleme ... */
	}

	return forms, totalCount, nil
}

// FindAllPaginated tüm formları sayfalayarak bulur (Admin için).
func (r *FormRepository) FindAllPaginated(ctx context.Context, params queryparams.ListParams) ([]models.Form, int64, error) {
	var forms []models.Form
	var totalCount int64
	db := r.getDB(ctx)

	query := db.Model(&models.Form{}) // Filtre yok

	// Ortak filtreleri ve sıralamayı uygula
	query, needsJoin := r.applyFormFilters(query, params)

	// Toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		configslog.Log.Error("FormRepository.Count (Paginated All): DB error", zap.Error(err))
		return nil, 0, err
	}

	if totalCount == 0 {
		return forms, 0, nil
	}

	// Preload, Sayfalama, Find
	query = query.Preload("Detail").Preload("Link.Type")
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)
	if needsJoin {
		query = query.Select("forms.*")
	}
	err = query.Find(&forms).Error
	if err != nil {
		configslog.Log.Error("FormRepository.Find (Paginated All): DB error", zap.Error(err))
		return nil, totalCount, err
	}

	if needsJoin && len(forms) > 0 { /* ... Detay yükleme ... */
	}

	return forms, totalCount, nil
}

// Update sadece ana Form modelini günceller.
func (r *FormRepository) Update(ctx context.Context, form *models.Form) error {
	if form == nil || form.ID == 0 {
		return errors.New("güncellenecek form geçerli değil")
	}
	return r.getDB(ctx).Save(form).Error // BeforeUpdate hook çalışır
}

// UpdateDetail sadece FormDetail modelini günceller.
func (r *FormRepository) UpdateDetail(ctx context.Context, detail *models.FormDetail) error {
	if detail == nil || detail.ID == 0 {
		return errors.New("güncellenecek form detayı geçerli değil")
	}
	return r.getDB(ctx).Save(detail).Error // BeforeUpdate hook çalışır
}

// Delete formu siler (soft delete).
// İlişkili Detail (cascade), Link (serviste), FieldDefinitions ve Submissions silinmeli.
func (r *FormRepository) Delete(ctx context.Context, form *models.Form, deletedByUserID uint) error {
	if form == nil || form.ID == 0 {
		return errors.New("silinecek form geçerli değil")
	}
	db := r.getDB(ctx)

	// Transaction içinde DeletedBy'ı ayarla ve soft delete yap
	return db.Transaction(func(tx *gorm.DB) error {
		// TODO: İlişkili FieldDefinitions ve Submissions kayıtlarını sil (önce bunlar)
		// if err := tx.Where("form_id = ?", form.ID).Delete(&models.FormFieldDefinition{}).Error; err != nil { return err }
		// if err := tx.Where("form_id = ?", form.ID).Delete(&models.FormSubmission{}).Error; err != nil { return err }

		// Soft delete için DeletedBy ve DeletedAt'ı ayarla
		now := time.Now().UTC()
		updateData := map[string]interface{}{"deleted_at": now, "deleted_by": &deletedByUserID}
		result := tx.Model(form).Where("id = ? AND deleted_at IS NULL", form.ID).Updates(updateData)
		if result.Error != nil {
			configslog.Log.Error("FormRepository.Delete: Update sırasında hata", zap.Uint("id", form.ID), zap.Error(result.Error))
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		} // Zaten silinmiş veya yok
		return nil
	})
}

// CountByUserID belirli bir kullanıcıya ait form sayısını döndürür.
func (r *FormRepository) CountByUserID(ctx context.Context, creatorUserID uint) (int64, error) {
	if creatorUserID == 0 {
		return 0, errors.New("geçersiz Creator User ID")
	}
	var count int64
	err := r.getDB(ctx).Model(&models.Form{}).Where("creator_user_id = ?", creatorUserID).Count(&count).Error
	return count, err
}

// CountAll tüm formların sayısını döndürür (Admin için).
func (r *FormRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.getDB(ctx).Model(&models.Form{}).Count(&count).Error
	return count, err
}

var _ IFormRepository = (*FormRepository)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewFormRepositoryTx(tx *gorm.DB) IFormRepository {
	base := NewBaseRepository[models.Form](tx)
	base.SetAllowedSortColumns([]string{"id", "created_at", "is_enabled"})
	return &FormRepository{db: tx, base: base}
}
