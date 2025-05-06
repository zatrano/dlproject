package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MigrateInvitationsTables(db *gorm.DB) error {
	configslog.SLog.Info("Migrating invitations & invitation_details tables...")
	err := db.AutoMigrate(&models.Invitation{}, &models.InvitationDetail{})
	if err != nil {
		configslog.Log.Error("Failed to migrate invitations & invitation_details tables", zap.Error(err))
		return err
	}
	configslog.SLog.Info("Invitations & invitation_details tables migrated successfully")
	return nil
}
