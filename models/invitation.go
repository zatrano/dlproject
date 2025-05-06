package models

// Invitation dijital davetiyenin ana kaydıdır.
type Invitation struct {
	BaseModel            // Gömülü BaseModel
	LinkID         uint  `gorm:"uniqueIndex;not null"`
	CreatorUserID  uint  `gorm:"index;not null"`
	OrganizationID *uint `gorm:"index"` // Nullable olabilir
	IsEnabled      bool  `gorm:"default:true;index"`

	// GORM İlişkileri
	Link Link `gorm:"foreignKey:LinkID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	// Creator User `gorm:"foreignKey:CreatorUserID"` // İsteğe bağlı
	// Organization Organization `gorm:"foreignKey:OrganizationID"` // İsteğe bağlı
	Detail InvitationDetail `gorm:"foreignKey:InvitationID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
