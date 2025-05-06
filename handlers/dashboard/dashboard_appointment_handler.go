package handlers

import (
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/repositories" // Direkt repo kullanımı (ShowUpdate için)
	"davet.link/services"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm" // Hata kontrolü için
)

// AppointmentHandler randevu hizmeti yönetimi için handler (Dashboard).
type AppointmentHandler struct {
	service     services.IAppointmentService
	userService services.IUserService // Admin yetki kontrolü için
}

// NewAppointmentHandler yeni bir AppointmentHandler örneği oluşturur.
func NewAppointmentHandler() *AppointmentHandler {
	return &AppointmentHandler{
		service:     services.NewAppointmentService(),
		userService: services.NewUserService(), // Admin kontrolü için
	}
}

// ListAppointments tüm randevu hizmetlerini listeler (Admin için).
func (h *AppointmentHandler) ListAppointments(c *fiber.Ctx) error {
	flashData, _ := flashmessages.GetFlashMessages(c)
	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /* ... Hata/Varsayılan ... */ params = queryparams.DefaultListParams("created_at")}
    params.Validate() // Sayfa, PerPage limitleri

	// TODO: Servis katmanına GetAllAppointmentsPaginated(params) eklenmeli.
	// Şimdilik geçici olarak hata döndürelim veya boş liste gösterelim.
	// paginatedResult, err := h.service.GetAllAppointmentsPaginated(c.UserContext(), params)
	paginatedResult := &queryparams.PaginatedResult{Data: []models.Appointment{}, Meta: queryparams.PaginationMeta{}} // Geçici boş sonuç
	err := errors.New("tüm randevu hizmetlerini listeleme henüz implemente edilmedi") // Geçici hata

	renderData := fiber.Map{
		"Title":     "Tüm Randevu Hizmetleri",
		"CsrfToken": c.Locals("csrf"),
		"Result":    paginatedResult,
		"Params":    params,
	}
	renderer.SetFlashMessages(renderData, flashData) // Flash mesajları ekle

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Randevu hizmetleri listelenirken hata oluştu."
        renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Appointment{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Dashboard - ListAppointments Error", zap.Error(err))
	}
	// View: dashboard/appointments/list.html
	return renderer.Render(c, "dashboard/appointments/list", "layouts/dashboard_layout", renderData, http.StatusOK)
}

// ShowUpdateAppointment bir randevu hizmetinin düzenleme formunu gösterir (Admin).
func (h *AppointmentHandler) ShowUpdateAppointment(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/appointments")}
	appointmentID := uint(id)

	// Admin herhangi bir kaydı görebilir, direkt repo'dan çekelim.
	// Servisin GetByID metodu yetki kontrolü yapabilir, bu yüzden repo daha uygun.
	appointment, err := repositories.NewAppointmentRepository().FindByID(c.UserContext(), appointmentID) // Context ile
	if err != nil {
		errMsg := "Randevu hizmeti bulunamadı."
		if !errors.Is(err, repositories.ErrNotFound) { // Repo hatasını kontrol et
             errMsg = "Randevu hizmeti bilgileri alınırken hata oluştu."
             configslog.Log.Error("Dashboard - ShowUpdateAppointment Error", zap.Uint("id", appointmentID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/dashboard/appointments")
	}

    formData := flashmessages.GetFlashFormData(c) // Hata sonrası için

	// View: dashboard/appointments/update.html
	return renderer.Render(c, "dashboard/appointments/update", "layouts/dashboard_layout", fiber.Map{
		"Title":       "Randevu Hizmetini Düzenle (Admin)",
		"Appointment": appointment,
		"Detail":      appointment.Detail,
        "FormData":    formData,
	})
}

// UpdateAppointment bir randevu hizmetini günceller (Admin yetkisiyle).
func (h *AppointmentHandler) UpdateAppointment(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") } // Güvenlik

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/appointments")}
	appointmentID := uint(id)
	redirectPathOnError := fmt.Sprintf("/dashboard/appointments/update/%d", appointmentID)

	var detailUpdates models.AppointmentDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        _ = flashmessages.SetFlashFormData(c, detailUpdates) // Parse edilebilen kısmı sakla?
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

	// Validasyon
	if err := services.ValidateAppointmentDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	// Servisi çağır (adminUserID ile, servis admin yetkisini kontrol etmeli)
	err = h.service.UpdateAppointment(c.UserContext(), appointmentID, adminUserID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrAppointmentNotFound) {
			 // Adminin görmeye çalıştığı ama olmayan kayıt
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/dashboard/appointments")
		}
        // Forbidden hatası admin için gelmemeli ama gelirse logla
        if errors.Is(err, services.ErrAppointmentForbidden) {
             configslog.Log.Warn("UpdateAppointment: Admin yetkisi reddedildi?", zap.Uint("id", appointmentID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
        }
		configslog.Log.Error("Dashboard - UpdateAppointment Error", zap.Uint("id", appointmentID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates) // Hata sonrası formu doldur
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Randevu hizmeti başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Başarı sonrası formu tekrar göster
}

// DeleteAppointment bir randevu hizmetini siler (Admin yetkisiyle).
func (h *AppointmentHandler) DeleteAppointment(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/appointments")}
	appointmentID := uint(id)

	// Servisi çağır (adminUserID ile)
	err = h.service.DeleteAppointment(c.UserContext(), appointmentID, adminUserID) // Servis admini tanımalı
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
        if errors.Is(err, services.ErrAppointmentNotFound) {
            errMsg = "Silinecek randevu hizmeti bulunamadı."
        } else {
             configslog.Log.Error("Dashboard - DeleteAppointment Error", zap.Uint("id", appointmentID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Randevu hizmeti başarıyla silindi.")
	}
	return c.Redirect("/dashboard/appointments", fiber.StatusSeeOther) // Listeye dön
}