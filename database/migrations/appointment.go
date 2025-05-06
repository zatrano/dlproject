package migrations

import (
	"davet.link/configs/configslog"
	"davet.link/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

func MigrateAppointmentsTables(db *gorm.DB) error {
	configslog.SLog.Info("Migrating appointments & appointment_details tables...")
	err := db.AutoMigrate(&models.Appointment{}, &models.AppointmentDetail{})
	if err != nil {
		configslog.Log.Error("Failed to migrate appointments & appointment_details tables", zap.Error(err))
		return err
	}
	configslog.SLog.Info("Appointments & appointment_details tables migrated successfully")
	return nil
}
