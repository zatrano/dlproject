// repositories/invitation_rsvp_repository.go
package repositories

import (
	"context"
	"errors"
	"time"

	"davet.link/configs"
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// IInvitationRSVPRepository RSVP veritabanı işlemleri için arayüz.
type IInvitationRSVPRepository interface {
	CreateOrUpdate(ctx context.Context, rsvp *models.InvitationRSVP) error // Varsa günceller, yoksa oluşturur
	FindByInvitationAndGuest(ctx context.Context, invitationID uint, guestID uint) (*models.InvitationRSVP, error)
	FindByInvitationID(ctx context.Context, invitationID uint) ([]models.InvitationRSVP, error) // Belirli davetiyenin tüm RSVP'leri
	Delete(ctx context.Context, rsvp *models.InvitationRSVP, deletedByUserID uint) error
	// FindByID(ctx context.Context, id uint) (*models.InvitationRSVP, error) // Gerekirse eklenebilir
}

// InvitationRSVPRepository IInvitationRSVPRepository arayüzünü uygular.
type InvitationRSVPRepository struct {
	db *gorm.DB
}

// NewInvitationRSVPRepository yeni bir InvitationRSVPRepository örneği oluşturur.
func NewInvitationRSVPRepository() IInvitationRSVPRepository {
	return &InvitationRSVPRepository{db: configs.GetDB()}
}

// Context ile çalışan DB örneği
func (r *InvitationRSVPRepository) getDB(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value("tx").(*gorm.DB); ok && tx != nil {
		return tx
	}
	return r.db.WithContext(ctx)
}

// CreateOrUpdate bir RSVP kaydını bulur veya oluşturur/günceller.
// Aynı InvitationID ve InvitationGuestID varsa günceller, yoksa oluşturur.
// Unique index kontrolü yerine bu yaklaşım daha esnek olabilir.
func (r *InvitationRSVPRepository) CreateOrUpdate(ctx context.Context, rsvp *models.InvitationRSVP) error {
	if rsvp == nil || rsvp.InvitationID == 0 || rsvp.InvitationGuestID == nil || *rsvp.InvitationGuestID == 0 {
		// GuestID'nin zorunlu olduğunu varsayıyoruz şimdilik
		return errors.New("geçersiz RSVP verisi (InvitationID veya GuestID eksik)")
	}
	db := r.getDB(ctx) // Context'li DB

	// Assign ile bulursa belirtilen alanları günceller, bulamazsa tüm rsvp verisiyle oluşturur.
	// Where koşulu önemli.
	return db.Where(models.InvitationRSVP{
		InvitationID:      rsvp.InvitationID,
		InvitationGuestID: rsvp.InvitationGuestID,
	}).Assign(models.InvitationRSVP{ // Assign ile güncellenecek/oluşturulacak değerler
		Status:      rsvp.Status,
		PlusOnes:    rsvp.PlusOnes,
		Notes:       rsvp.Notes,
		RespondedAt: rsvp.RespondedAt,
		// CreatedBy/UpdatedBy hook tarafından ayarlanır
	}).FirstOrCreate(rsvp).Error // FirstOrCreate burada hem bulur hem oluşturur/günceller (Assign sayesinde)
}

// FindByInvitationAndGuest belirli bir davetli ve davetiye için RSVP'yi bulur.
func (r *InvitationRSVPRepository) FindByInvitationAndGuest(ctx context.Context, invitationID uint, guestID uint) (*models.InvitationRSVP, error) {
	if invitationID == 0 || guestID == 0 {
		return nil, errors.New("geçersiz ID")
	}
	var rsvp models.InvitationRSVP
	err := r.getDB(ctx).Where("invitation_id = ? AND invitation_guest_id = ?", invitationID, guestID).
		Preload("InvitationGuest"). // İsteğe bağlı olarak Guest bilgisini de alabiliriz
		First(&rsvp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		} // Base repo hatası
		configslog.Log.Error("FindByInvitationAndGuest error", zap.Error(err))
		return nil, err
	}
	return &rsvp, nil
}

// FindByInvitationID belirli bir davetiyeye ait tüm RSVP'leri getirir.
func (r *InvitationRSVPRepository) FindByInvitationID(ctx context.Context, invitationID uint) ([]models.InvitationRSVP, error) {
	if invitationID == 0 {
		return nil, errors.New("geçersiz Invitation ID")
	}
	var rsvps []models.InvitationRSVP
	// Misafir bilgilerini de almak için Preload
	err := r.getDB(ctx).Where("invitation_id = ?", invitationID).
		Preload("InvitationGuest").
		Order("created_at asc"). // Veya responded_at'a göre sırala
		Find(&rsvps).Error
	if err != nil {
		configslog.Log.Error("FindByInvitationID error", zap.Uint("invitationID", invitationID), zap.Error(err))
		return nil, err
	}
	return rsvps, nil
}

// Delete RSVP kaydını siler (soft delete).
func (r *InvitationRSVPRepository) Delete(ctx context.Context, rsvp *models.InvitationRSVP, deletedByUserID uint) error {
	if rsvp == nil || rsvp.ID == 0 {
		return errors.New("geçersiz RSVP")
	}
	db := r.getDB(ctx)

	// Transaction içinde DeletedBy ayarla ve sil
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updateData := map[string]interface{}{"deleted_at": now, "deleted_by": &deletedByUserID}
		result := tx.Model(rsvp).Where("id = ? AND deleted_at IS NULL", rsvp.ID).Updates(updateData)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

var _ IInvitationRSVPRepository = (*InvitationRSVPRepository)(nil)

// Transaction'lı Repository için yardımcı constructor
func NewInvitationRSVPRepositoryTx(tx *gorm.DB) IInvitationRSVPRepository {
	return &InvitationRSVPRepository{db: tx}
}
