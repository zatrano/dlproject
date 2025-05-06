package models

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	createdByColumn = "created_by"
	updatedByColumn = "updated_by"
	deletedByColumn = "deleted_by"
)

const contextUserIDKey = "user_id"

type BaseModel struct {
	ID        uint `gorm:"primarykey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	CreatedBy uint
	UpdatedBy uint
	DeletedBy *uint `gorm:"column:deleted_by"`
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) (err error) {
	userID, ok := tx.Statement.Context.Value(contextUserIDKey).(uint)
	if ok && userID != 0 {
		b.CreatedBy = userID
		b.UpdatedBy = userID
	} else {
		return errors.New("BeforeCreate: kullanıcı kimliği bulunamadı")
	}
	return nil
}

func (b *BaseModel) BeforeUpdate(tx *gorm.DB) (err error) {
	userID, ok := tx.Statement.Context.Value(contextUserIDKey).(uint)
	if ok && userID != 0 {
		tx.Statement.SetColumn(updatedByColumn, userID)
	} else {
		return errors.New("BeforeUpdate: kullanıcı kimliği bulunamadı")
	}
	return nil
}
