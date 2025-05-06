package models

import (
	"time"
)

// FormDetail formun detaylarını içerir.
type FormDetail struct {
	BaseModel      // Gömülü BaseModel
	FormID    uint `gorm:"uniqueIndex;not null"` // forms.id FK

	// Detay Alanları (Örnekler - GORM tipleri eklendi)
	Title               string     `gorm:"type:varchar(255);not null"`
	Description         string     `gorm:"type:text"`
	SubmissionLimit     *int       `gorm:"type:integer"`           // Nullable integer
	LimitPerUser        *int       `gorm:"type:integer"`           // Nullable integer
	ClosesAt            *time.Time `gorm:"index;type:timestamptz"` // Nullable timestamptz
	ConfirmationMessage string     `gorm:"type:text"`
	RedirectURLOnSubmit string     `gorm:"type:varchar(500)"`
	NotifyOnSubmitEmail string     `gorm:"type:text"`
	RequiresLogin       bool       `gorm:"type:boolean;default:false"`
	PasswordHash        string     `gorm:"type:varchar(255)"`
}
