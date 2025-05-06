package models

import (
	"time"
	// "github.com/shopspring/decimal"
)

// AppointmentDetail randevu hizmetinin detaylarını içerir.
type AppointmentDetail struct {
	BaseModel          // Gömülü BaseModel
	AppointmentID uint `gorm:"uniqueIndex;not null"` // appointments.id FK

	// Detay Alanları (Örnekler - GORM tipleri eklendi)
	Name               string     `gorm:"type:varchar(200);not null"`
	Description        string     `gorm:"type:text"`
	DurationMinutes    int        `gorm:"type:integer;not null"`
	Price              float64    `gorm:"type:numeric(12,2);default:0.00"`
	Currency           string     `gorm:"type:varchar(3);default:'TRY'"`
	RequiresApproval   bool       `gorm:"type:boolean;default:false"`
	BufferTimeBefore   int        `gorm:"type:integer;default:0"`
	BufferTimeAfter    int        `gorm:"type:integer;default:0"`
	BookingLeadTime    int        `gorm:"type:integer;default:60"`
	BookingHorizonDays int        `gorm:"type:integer;default:30"`
	ColorCode          string     `gorm:"type:varchar(7)"`
	CancellationPolicy string     `gorm:"type:text"`
	PasswordHash       string     `gorm:"type:varchar(255)"`
	ExpiresAt          *time.Time `gorm:"index;type:timestamptz"`
}
