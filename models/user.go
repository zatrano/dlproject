package models

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

type UserType string

const (
	Dashboard UserType = "dashboard"
	Panel     UserType = "panel"
)

func (UserType) GormDataType() string {
	return "user_type"
}
func (UserType) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	if db.Dialector.Name() == "postgres" {
		return "user_type"
	}
	return "varchar(10)"
}

type User struct {
	BaseModel
	Name     string   `gorm:"size:100;not null;index"`
	Account  string   `gorm:"size:100;unique;not null"`
	Password string   `gorm:"size:255;not null"`
	Status   bool     `gorm:"default:true;index"`
	Type     UserType `gorm:"type:user_type;not null;default:'panel';index"`
}

func (u *User) CheckPassword(password string) error {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
}

func (u *User) SetPassword(password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = string(hashedPassword)
	return nil
}
