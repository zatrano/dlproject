// models/invitation.go
package models

import "time" // Detail için

type Invitation struct {
	BaseModel
	LinkID         uint             `gorm:"uniqueIndex;not null"`
	Link           Link             `gorm:"foreignKey:LinkID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	CreatorUserID  uint             `gorm:"index;not null"`
	OrganizationID *uint            `gorm:"index"`
	IsEnabled      bool             `gorm:"default:true;index"`
	Detail         InvitationDetail `gorm:"foreignKey:InvitationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`

	// İlişkiler
	Guests            []InvitationGuest       `gorm:"foreignKey:InvitationID"` // Davetliler
	CustomGuestFields []InvitationCustomField `gorm:"foreignKey:InvitationID"` // Davetliden istenecek özel alanlar
	RSVPs             []InvitationRSVP        `gorm:"foreignKey:InvitationID"` // YENİ: Bu davetiyeye gelen RSVP'ler (One-to-Many)
}

// InvitationDetail (RSVP ile ilgili yeni alanlar eklenebilir - Opsiyonel)
type InvitationDetail struct {
	// ... (önceki alanlar: Title, Description, EventDateTime etc.) ...
	BaseModel
	InvitationID           uint       `gorm:"uniqueIndex;not null"`
	Title                  string     `gorm:"type:varchar(255);not null"`
	Description            string     `gorm:"type:text"`
	EventDateTime          time.Time  `gorm:"index;type:timestamptz"`
	Timezone               string     `gorm:"type:varchar(50);default:'UTC'"`
	LocationText           string     `gorm:"type:varchar(255)"`
	LocationURL            string     `gorm:"type:varchar(500)"`
	Theme                  string     `gorm:"type:varchar(50)"`
	PasswordHash           string     `gorm:"type:varchar(255)"`
	ExpiresAt              *time.Time `gorm:"index;type:timestamptz"`
	RSVPDeadline           *time.Time `gorm:"index;type:timestamptz"`
	AllowPlusOnes          bool       `gorm:"type:boolean;default:true"`
	MaxPlusOnes            int        `gorm:"type:integer;default:1"`
	ShowGuestList          bool       `gorm:"type:boolean;default:false"`
	RequireLoginToRSVP     bool       `gorm:"type:boolean;default:false"` // YENİ: RSVP için giriş zorunlu mu?
	LimitRSVPToOnePerGuest bool       `gorm:"type:boolean;default:true"`  // YENİ: Bir misafir sadece 1 RSVP mi yapabilir?
	CollectGuestNotes      bool       `gorm:"type:boolean;default:true"`  // YENİ: RSVP'de not alanı gösterilsin mi?
}

// InvitationGuest (Değişiklik Gerekmez)
// InvitationCustomField (Değişiklik Gerekmez)
