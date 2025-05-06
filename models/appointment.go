package models

// Appointment randevu hizmetinin ana kaydıdır.
type Appointment struct {
	BaseModel            // Gömülü BaseModel
	LinkID         uint  `gorm:"uniqueIndex;not null"`
	ProviderUserID uint  `gorm:"index;not null"` // Hizmeti veren kullanıcı
	OrganizationID *uint `gorm:"index"`
	IsEnabled      bool  `gorm:"default:true;index"`

	// GORM İlişkileri
	Link Link `gorm:"foreignKey:LinkID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Provider User `gorm:"foreignKey:ProviderUserID"` // İsteğe bağlı
	// Organization Organization `gorm:"foreignKey:OrganizationID"` // İsteğe bağlı
	Detail AppointmentDetail `gorm:"foreignKey:AppointmentID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
