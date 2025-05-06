package models

type Type struct {
	BaseModel
	Name        string `gorm:"type:varchar(50);uniqueIndex;not null"`
	Description string `gorm:"type:text"`
}

const (
	TypeNameAppointment = "APPOINTMENT"
	TypeNameCard        = "CARD"
	TypeNameForm        = "FORM"
	TypeNameInvitation  = "INVITATION"
)
