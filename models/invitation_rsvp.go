// models/invitation_rsvp.go
package models

import (
	"time"
)

// RSVPStatus olası LCV durumlarını tanımlar.
type RSVPStatus string

const (
	RSVPStatusPending      RSVPStatus = "pending"       // Henüz cevap verilmedi
	RSVPStatusAttending    RSVPStatus = "attending"     // Katılacak
	RSVPStatusNotAttending RSVPStatus = "not_attending" // Katılmayacak
	RSVPStatusMaybe        RSVPStatus = "maybe"         // Belki katılacak
)

// InvitationRSVP bir davetiyeye verilen LCV yanıtını temsil eder.
type InvitationRSVP struct {
	BaseModel         // ID, CreatedAt, UpdatedAt, DeletedAt, CreatedBy, UpdatedBy, DeletedBy
	InvitationID uint `gorm:"not null;index:idx_rsvp_inv_guest,unique"` // Hangi davetiyeye ait? (Guest ile birlikte unique)
	// Invitation   Invitation `gorm:"foreignKey:InvitationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"` // İlişki

	// RSVP'yi kimin yaptığı? İki seçenek var:
	// 1. Eğer davetli listesi (InvitationGuest) kullanıyorsak:
	InvitationGuestID *uint           `gorm:"index:idx_rsvp_inv_guest,unique"`                                             // Hangi davetli? (Nullable, eğer guest yoksa)
	InvitationGuest   InvitationGuest `gorm:"foreignKey:InvitationGuestID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"` // Guest silinirse RSVP kalabilir ama bağlantı kopar

	// 2. VEYA Davetli listesi yoksa/kullanılmıyorsa, misafir bilgilerini burada tutarız:
	// GuestName       string `gorm:"size:150"` // RSVP yapan kişinin adı (GuestID null ise)
	// GuestEmail      string `gorm:"size:150;index"` // RSVP yapanın e-postası (GuestID null ise)
	// GuestPhone      string `gorm:"size:30"`    // RSVP yapanın telefonu (GuestID null ise)
	// => Şimdilik InvitationGuestID kullanalım, daha yapısal.

	Status      RSVPStatus `gorm:"type:varchar(20);not null;default:'pending';index"` // Cevap durumu
	PlusOnes    int        `gorm:"type:integer;default:0"`                            // Yanında getireceği ek kişi sayısı
	Notes       string     `gorm:"type:text"`                                         // Misafirin notu
	RespondedAt *time.Time // Cevap verme zamanı
}
