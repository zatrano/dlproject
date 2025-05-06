package models

// Form online formun ana kaydıdır.
type Form struct {
	BaseModel            // Gömülü BaseModel
	LinkID         uint  `gorm:"uniqueIndex;not null"`
	CreatorUserID  uint  `gorm:"index;not null"`
	OrganizationID *uint `gorm:"index"`
	IsEnabled      bool  `gorm:"default:true;index"`

	// GORM İlişkileri
	Link Link `gorm:"foreignKey:LinkID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Creator User `gorm:"foreignKey:CreatorUserID"` // İsteğe bağlı
	// Organization Organization `gorm:"foreignKey:OrganizationID"` // İsteğe bağlı
	Detail FormDetail `gorm:"foreignKey:FormID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
