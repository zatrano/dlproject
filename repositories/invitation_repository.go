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

// IInvitationRepository davetiye veritabanı işlemleri için arayüz.
type IInvitationRepository interface {
	Create(ctx context.Context, invitation *models.Invitation) error
	FindByID(ctx context.Context, id uint) (*models.Invitation, error)
	FindByLinkID(ctx context.Context, linkID uint) (*models.Invitation, error)
	FindAllByUserIDPaginated(ctx context.Context, userID uint, params queryparams.ListParams) ([]models.Invitation, int64, error)
	FindAllPaginated(ctx context.Context, params queryparams.ListParams) ([]models.Invitation, int64, error) // Admin için tümü
	Update(ctx context.Context, invitation *models.Invitation) error
	UpdateDetail(ctx context.Context, detail *models.InvitationDetail) error
	Delete(ctx context.Context, invitation *models.Invitation, deletedByUserID uint) error
	CountByUserID(ctx context.Context, userID uint) (int64, error)
	CountAll(ctx context.Context) (int64, error) // Admin için tümü
}

// InvitationRepository IInvitationRepository arayüzünü uygular.
type InvitationRepository struct {
	db   *gorm.DB
	base IBaseRepository[models.Invitation] // Generik base repo (opsiyonel)
}

// NewInvitationRepository yeni bir InvitationRepository örneği oluşturur.
func NewInvitationRepository() IInvitationRepository {
	db := configs.GetDB()
	base := NewBaseRepository[models.Invitation](db)
	// Davetiye için izin verilen sıralama sütunları
	base.SetAllowedSortColumns([]string{
		"id", "created_at", "is_enabled", // Ana tablo
		// Detay alanları için GetAll metodunda özel handle gerekir
		// "title", "event_date_time", "rsvp_deadline",
	})
	return &InvitationRepository{db: db, base: base}
}

// Context ile çalışan DB örneği döndüren yardımcı fonksiyon
func (r *InvitationRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok && tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// Create yeni bir davetiye ve detayını oluşturur.
func (r *InvitationRepository) Create(ctx context.Context, invitation *models.Invitation) error {
	if invitation == nil || invitation.LinkID == 0 {
		return errors.New("geçersiz veya eksik link bilgisi olan davetiye oluşturulamaz")
	}
	return r.getDB(ctx).Create(invitation).Error // BeforeCreate hook çalışır
}

// FindByID belirli bir ID'ye sahip davetiyeyi bulur.
func (r *InvitationRepository) FindByID(ctx context.Context, id uint) (*models.Invitation, error) {
	if id == 0 {
		return nil, errors.New("geçersiz Invitation ID")
	}
	var invitation models.Invitation
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").First(&invitation, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		} // Base repo hatası
		configslog.Log.Error("InvitationRepository.FindByID: DB error", zap.Uint("id", id), zap.Error(err))
		return nil, err
	}
	return &invitation, nil
}

// FindByLinkID belirli bir Link ID'sine sahip davetiyeyi bulur.
func (r *InvitationRepository) FindByLinkID(ctx context.Context, linkID uint) (*models.Invitation, error) {
	if linkID == 0 {
		return nil, errors.New("geçersiz Link ID")
	}
	var invitation models.Invitation
	err := r.getDB(ctx).Preload("Detail").Preload("Link.Type").Where("link_id = ?", linkID).First(&invitation).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		configslog.Log.Error("InvitationRepository.FindByLinkID: DB error", zap.Uint("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return &invitation, nil
}

// applyInvitationFilters ortak filtreleme ve sıralama mantığını uygular.
func (r *InvitationRepository) applyInvitationFilters(db *gorm.DB, params queryparams.ListParams) *gorm.DB {
	query := db // Mevcut sorguyu al

	// İsim/Başlık filtresi (Detail tablosundaki Title'a göre)
	needsJoin := false
	if params.Name != "" {
		sqlFragment, args := turkishsearch.SQLFilter("invitation_details.title", params.Name)
		query = query.Joins("JOIN invitation_details ON invitation_details.invitation_id = invitations.id").Where(sqlFragment, args...)
		needsJoin = true
	}
	// Status filtresi (Ana tablodan)
	if params.Status != "" {
		statusBool := params.Status == "true"
		query = query.Where("invitations.is_enabled = ?", statusBool) // Tablo adını belirt
	}

	// Sıralama
	sortBy := params.SortBy
	orderBy := strings.ToLower(params.OrderBy)
	if orderBy != "asc" && orderBy != "desc" {
		orderBy = queryparams.DefaultOrderBy
	}

	allowedSortColumns := map[string]string{
		"id":              "invitations.id",
		"created_at":      "invitations.created_at",
		"is_enabled":      "invitations.is_enabled",
		"title":           "invitation_details.title",
		"event_date_time": "invitation_details.event_date_time",
		"rsvp_deadline":   "invitation_details.rsvp_deadline",
	}
	orderColumn := "invitations.created_at" // Varsayılan
	if dbCol, ok := allowedSortColumns[sortBy]; ok {
		orderColumn = dbCol
		// Detail sütununa göre sıralama için JOIN kontrolü
		if (sortBy == "title" || sortBy == "event_date_time" || sortBy == "rsvp_deadline") && !needsJoin {
			query = query.Joins("JOIN invitation_details ON invitation_details.invitation_id = invitations.id")
		}
	} else {
		configslog.SLog.Warn("Geçersiz Invitation sıralama alanı istendi, varsayılan kullanılıyor.", zap.String("requestedSortBy", sortBy))
		sortBy = "created_at"
		orderColumn = "invitations.created_at"
	}
	query = query.Order(orderColumn + " " + orderBy)

	return query
}

// FindAllByUserIDPaginated belirli bir kullanıcıya ait davetiyeleri sayfalayarak bulur.
func (r *InvitationRepository) FindAllByUserIDPaginated(ctx context.Context, userID uint, params queryparams.ListParams) ([]models.Invitation, int64, error) {
	if userID == 0 {
		return nil, 0, errors.New("geçersiz User ID")
	}
	var invitations []models.Invitation
	var totalCount int64
	db := r.getDB(ctx)

	query := db.Model(&models.Invitation{}).Where("invitations.creator_user_id = ?", userID) // Ana tablo filtresi

	// Ortak filtreleri ve sıralamayı uygula
	query = r.applyInvitationFilters(query, params)

	// Toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		configslog.Log.Error("InvitationRepository.Count (Paginated by User): DB error", zap.Uint("userID", userID), zap.Error(err))
		return nil, 0, err
	}

	if totalCount == 0 {
		return invitations, 0, nil
	}

	// İlişkili verileri preload et
	query = query.Preload("Detail").Preload("Link.Type")

	// Sayfalama
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)

	// Verileri çek (Select gerekebilir eğer JOIN yapıldıysa)
	if params.Name != "" {
		query = query.Select("invitations.*")
	} // Sadece ana tabloyu seç
	err = query.Find(&invitations).Error
	if err != nil {
		configslog.Log.Error("InvitationRepository.Find (Paginated by User): DB error", zap.Uint("userID", userID), zap.Error(err))
		return nil, totalCount, err
	}

	// JOIN yapıldığında Detail verilerini manuel yükle (eğer GORM yapmazsa)
	if params.Name != "" && len(invitations) > 0 { /* ... Detay yükleme ... */
	}

	return invitations, totalCount, nil
}

// FindAllPaginated tüm davetiyeleri sayfalayarak bulur (Admin için).
func (r *InvitationRepository) FindAllPaginated(ctx context.Context, params queryparams.ListParams) ([]models.Invitation, int64, error) {
	var invitations []models.Invitation
	var totalCount int64
	db := r.getDB(ctx)

	query := db.Model(&models.Invitation{}) // Filtre yok

	// Ortak filtreleri ve sıralamayı uygula
	query = r.applyInvitationFilters(query, params)

	// Toplam sayıyı al
	err := query.Count(&totalCount).Error
	if err != nil {
		configslog.Log.Error("InvitationRepository.Count (Paginated All): DB error", zap.Error(err))
		return nil, 0, err
	}

	if totalCount == 0 {
		return invitations, 0, nil
	}

	// Preload, Sayfalama, Find (FindAllByUserIDPaginated ile aynı)
	query = query.Preload("Detail").Preload("Link.Type")
	offset := params.CalculateOffset()
	query = query.Limit(params.PerPage).Offset(offset)
	if params.Name != "" {
		query = query.Select("invitations.*")
	}
	err = query.Find(&invitations).Error
	if err != nil {
		configslog.Log.Error("InvitationRepository.Find (Paginated All): DB error", zap.Error(err))
		return nil, totalCount, err
	}

	if params.Name != "" && len(invitations) > 0 { /* ... Detay yükleme ... */
	}

	return invitations, totalCount, nil
}

// Update sadece ana Invitation modelini günceller (Save kullanarak).
func (r *InvitationRepository) Update(ctx context.Context, invitation *models.Invitation) error {
	if invitation == nil || invitation.ID == 0 {
		return errors.New("güncellenecek davetiye geçerli değil")
	}
	// BeforeUpdate hook'u context'i kullanır
	return r.getDB(ctx).Save(invitation).Error
}

// UpdateDetail sadece InvitationDetail modelini günceller (Save kullanarak).
func (r *InvitationRepository) UpdateDetail(ctx context.Context, detail *models.InvitationDetail) error {
	if detail == nil || detail.ID == 0 {
		return errors.New("güncellenecek davetiye detayı geçerli değil")
	}
	// BeforeUpdate hook'u context'i kullanır
	return r.getDB(ctx).Save(detail).Error
}

// Delete davetiyeyi siler (soft delete).
// Context'ten userID alıp DeletedBy'ı ayarlar.
func (r *InvitationRepository) Delete(ctx context.Context, invitation *models.Invitation, deletedByUserID uint) error {
	if invitation == nil || invitation.ID == 0 {
		return errors.New("silinecek davetiye geçerli değil")
	}
	db := r.getDB(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updateData := map[string]interface{}{"deleted_at": now, "deleted_by": &deletedByUserID}
		result := tx.Model(invitation).Where("id = ? AND deleted_at IS NULL", invitation.ID).Updates(updateData)
		if result.Error != nil {
			configslog.Log.Error("InvitationRepository.Delete: Update sırasında hata", zap.Uint("id", invitation.ID), zap.Error(result.Error))
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// CountByUserID belirli bir kullanıcıya ait davetiye sayısını döndürür.
func (r *InvitationRepository) CountByUserID(ctx context.Context, userID uint) (int64, error) {
	if userID == 0 {
		return 0, errors.New("geçersiz User ID")
	}
	var count int64
	err := r.getDB(ctx).Model(&models.Invitation{}).Where("creator_user_id = ?", userID).Count(&count).Error
	return count, err
}

// CountAll tüm davetiyelerin sayısını döndürür.
func (r *InvitationRepository) CountAll(ctx context.Context) (int64, error) {
	var count int64
	err := r.getDB(ctx).Model(&models.Invitation{}).Count(&count).Error
	return count, err
}

// Arayüz uyumluluğu kontrolü
var _ IInvitationRepository = (*InvitationRepository)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewInvitationRepositoryTx(tx *gorm.DB) IInvitationRepository {
	base := NewBaseRepository[models.Invitation](tx)
	base.SetAllowedSortColumns([]string{"id", "created_at", "is_enabled"})
	return &InvitationRepository{db: tx, base: base}
}
