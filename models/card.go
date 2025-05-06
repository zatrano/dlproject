package models

// Card dijital kartvizitin ana kaydıdır.
type Card struct {
	BaseModel
	LinkID         uint  `gorm:"uniqueIndex;not null"`
	CreatorUserID  uint  `gorm:"index;not null"`
	OrganizationID *uint `gorm:"index"`              // Opsiyonel
	IsEnabled      bool  `gorm:"default:true;index"` // Kartvizit aktif mi?

	// GORM İlişkileri
	Link Link `gorm:"foreignKey:LinkID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Creator User `gorm:"foreignKey:CreatorUserID"`
	// Organization Organization `gorm:"foreignKey:OrganizationID"`
	Detail CardDetail `gorm:"foreignKey:CardID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
