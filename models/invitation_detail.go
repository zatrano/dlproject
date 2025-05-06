package models

import (
	"time"
)

// InvitationDetail davetiyenin detaylarını içerir.
type InvitationDetail struct {
	BaseModel         // Gömülü BaseModel
	InvitationID uint `gorm:"uniqueIndex;not null"` // invitations.id FK

	// Detay Alanları (Örnekler - GORM tipleri eklendi)
	Title         string     `gorm:"type:varchar(255);not null"`
	Description   string     `gorm:"type:text"`
	EventDateTime time.Time  `gorm:"index;type:timestamptz"` // timestamptz önerilir
	Timezone      string     `gorm:"type:varchar(50);default:'UTC'"`
	LocationText  string     `gorm:"type:varchar(255)"`
	LocationURL   string     `gorm:"type:varchar(500)"`
	Theme         string     `gorm:"type:varchar(50)"`
	PasswordHash  string     `gorm:"type:varchar(255)"`
	ExpiresAt     *time.Time `gorm:"index;type:timestamptz"` // Nullable timestamptz
	RSVPDeadline  *time.Time `gorm:"index;type:timestamptz"` // Nullable timestamptz
	AllowPlusOnes bool       `gorm:"type:boolean;default:true"`
	MaxPlusOnes   int        `gorm:"type:integer;default:1"` // integer daha uygun olabilir
	ShowGuestList bool       `gorm:"type:boolean;default:false"`
}
