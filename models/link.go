package models

// Sadece gorm importu yeterli

// Link benzersiz bir 'Key'i belirli bir hizmete bağlar ve sahibini tutar.
type Link struct {
	BaseModel            // Gömülü BaseModel
	Key           string `gorm:"type:varchar(11);uniqueIndex;not null"`
	TypeID        uint   `gorm:"not null;index"`                 // types.id FK
	TargetID      uint   `gorm:"not null;index:idx_link_target"` // Hedef ID
	CreatorUserID uint   `gorm:"index;not null"`                 // users.id FK

	// GORM İlişkileri
	Type    Type `gorm:"foreignKey:TypeID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
	Creator User `gorm:"foreignKey:CreatorUserID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
}
