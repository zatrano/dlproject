package handlers // handlers/panel paketi

import (
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/services"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	// "gorm.io/gorm" // Gerekli değil
)

// PanelAppointmentHandler kullanıcının kendi randevu hizmetleri için handler.
type PanelAppointmentHandler struct {
	service services.IAppointmentService
}

// NewPanelAppointmentHandler yeni bir PanelAppointmentHandler örneği oluşturur.
func NewPanelAppointmentHandler() *PanelAppointmentHandler {
	return &PanelAppointmentHandler{
		service: services.NewAppointmentService(),
	}
}

// ListAppointments kullanıcının kendi randevu hizmetlerini listeler.
func (h *PanelAppointmentHandler) ListAppointments(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint) // ProviderUserID olarak kullanılacak
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /*...*/ params = queryparams.DefaultListParams("created_at") }
	params.Validate()

	paginatedResult, err := h.service.GetAppointmentsForUser(c.UserContext(), userID, params)

	renderData := fiber.Map{
		"Title":     "Randevu Hizmetlerim",
		"Result":    paginatedResult,
		"Params":    params,
	}
    renderer.SetFlashMessages(renderData, flashmessages.GetFlashMessages(c)) // Flash mesajları ekle

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Randevu hizmetleri listelenirken bir hata oluştu."
		renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Appointment{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Panel - ListAppointments Error", zap.Uint("userID", userID), zap.Error(err))
	}
	// View: panel/appointments/list.html
	return renderer.Render(c, "panel/appointments/list", "layouts/panel_layout", renderData, http.StatusOK)
}

// ShowCreateAppointment yeni randevu hizmeti oluşturma formunu gösterir.
func (h *PanelAppointmentHandler) ShowCreateAppointment(c *fiber.Ctx) error {
    formData := flashmessages.GetFlashFormData(c)
	// View: panel/appointments/create.html
	return renderer.Render(c, "panel/appointments/create", "layouts/panel_layout", fiber.Map{
		"Title":     "Yeni Randevu Hizmeti Oluştur",
        "FormData": formData,
	})
}

// CreateAppointment yeni randevu hizmeti oluşturur (giriş yapmış kullanıcı adına).
func (h *PanelAppointmentHandler) CreateAppointment(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint) // ProviderUserID olarak kullanılacak
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	var detail models.AppointmentDetail
	if err := c.BodyParser(&detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
		_ = flashmessages.SetFlashFormData(c, detail) // Hatalı veriyi flash'a kaydet
		return c.Redirect("/panel/appointments/create", fiber.StatusSeeOther)
	}

	if err := services.ValidateAppointmentDetail(detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/appointments/create", fiber.StatusSeeOther)
	}

	_, err := h.service.CreateAppointment(c.UserContext(), userID, nil, detail) // OrgID nil
	if err != nil {
		configslog.Log.Error("Panel - CreateAppointment Error", zap.Uint("providerUserID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Oluşturma hatası: "+err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/appointments/create", fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Randevu hizmeti başarıyla oluşturuldu.")
	return c.Redirect("/panel/appointments", fiber.StatusFound) // Panel listesine yönlendir
}

// ShowUpdateAppointment randevu hizmeti düzenleme formunu gösterir (sadece kendi hizmeti ise).
func (h *PanelAppointmentHandler) ShowUpdateAppointment(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/appointments")}
	appointmentID := uint(id)

	// Servis ID ve userID ile çağrılır (yetki kontrolü yapar)
	appointment, err := h.service.GetAppointmentByID(c.UserContext(), appointmentID, userID)
	if err != nil {
		errMsg := "Randevu hizmeti bulunamadı veya bu hizmeti düzenleme yetkiniz yok."
		if !errors.Is(err, services.ErrAppointmentNotFound) && !errors.Is(err, services.ErrAppointmentForbidden) {
			 errMsg = "Randevu hizmeti bilgileri alınırken bir hata oluştu."
			 configslog.Log.Error("Panel - ShowUpdateAppointment Error", zap.Uint("id", appointmentID), zap.Uint("userID", userID), zap.Error(err))
		}
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/panel/appointments")
	}

    formData := flashmessages.GetFlashFormData(c)

	// View: panel/appointments/update.html
	return renderer.Render(c, "panel/appointments/update", "layouts/panel_layout", fiber.Map{
		"Title":       "Randevu Hizmetini Düzenle",
		"Appointment": appointment,
		"Detail":      appointment.Detail,
        "FormData":    formData,
	})
}

// UpdateAppointment randevu hizmeti bilgilerini günceller (sadece kendi hizmeti ise).
func (h *PanelAppointmentHandler) UpdateAppointment(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/appointments")}
	appointmentID := uint(id)
	redirectPathOnError := fmt.Sprintf("/panel/appointments/update/%d", appointmentID)

	var detailUpdates models.AppointmentDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        _ = flashmessages.SetFlashFormData(c, detailUpdates)
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

	if err := services.ValidateAppointmentDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	// Servisi çağır (userID ile, servis yetki kontrolü yapar)
	err = h.service.UpdateAppointment(c.UserContext(), appointmentID, userID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrAppointmentNotFound) || errors.Is(err, services.ErrAppointmentForbidden) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/panel/appointments")
		}
        configslog.Log.Error("Panel - UpdateAppointment Error", zap.Uint("id", appointmentID), zap.Uint("userID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Randevu hizmeti başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Başarı sonrası formu tekrar göster
}

// DeleteAppointment randevu hizmetini siler (sadece kendi hizmeti ise).
func (h *PanelAppointmentHandler) DeleteAppointment(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/appointments")}
	appointmentID := uint(id)

	// Servisi çağır (userID ile, servis yetki kontrolü yapar)
	err = h.service.DeleteAppointment(c.UserContext(), appointmentID, userID)
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
        if !errors.Is(err, services.ErrAppointmentNotFound) && !errors.Is(err, services.ErrAppointmentForbidden) {
             configslog.Log.Error("Panel - DeleteAppointment Error", zap.Uint("id", appointmentID), zap.Uint("userID", userID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Randevu hizmeti başarıyla silindi.")
	}
	return c.Redirect("/panel/appointments", fiber.StatusSeeOther) // Her durumda listeye dön
}