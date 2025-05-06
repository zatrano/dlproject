// database/migrations/rsvp_migration.go
package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MigrateRSVPTable InvitationRSVP modeli için tabloyu oluşturur/günceller.
func MigrateRSVPTable(db *gorm.DB) error {
	configslog.SLog.Info("Migrating invitation_rsvps table...")
	// Invitation ve InvitationGuest tabloları zaten var olmalı (FK için).
	err := db.AutoMigrate(&models.InvitationRSVP{})
	if err != nil {
		configslog.Log.Error("Failed to migrate invitation_rsvps table", zap.Error(err))
		return err
	}

	// Opsiyonel: Eğer GORM nullable sütun üzerinde unique index oluşturmazsa, manuel ekle:
	/*
		if !db.Migrator().HasIndex(&models.InvitationRSVP{}, "idx_rsvp_inv_guest_unique") {
			// PostgreSQL için nullable unique index (GuestID null değilse kontrol et)
			sql := `CREATE UNIQUE INDEX IF NOT EXISTS idx_rsvp_inv_guest_unique
					 ON invitation_rsvps (invitation_id, invitation_guest_id)
					 WHERE invitation_guest_id IS NOT NULL;`
			if err := db.Exec(sql).Error; err != nil {
				configslog.Log.Error("Failed to create unique nullable index on invitation_rsvps", zap.Error(err))
				// Bu hatayı tolere edebilir miyiz? Belki evet, kontrol serviste yapılır.
			} else {
				configslog.SLog.Info("Unique nullable index idx_rsvp_inv_guest_unique created.")
			}
		}
	*/

	configslog.SLog.Info("Invitation_rsvps table migrated successfully")
	return nil
}
