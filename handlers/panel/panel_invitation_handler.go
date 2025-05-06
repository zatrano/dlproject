package handlers

import (
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/repositories" // Admin'in direkt ID ile çekmesi için
	"davet.link/services"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm" // Hata kontrolü için
)

// InvitationHandler davetiye yönetimi için handler (Dashboard).
type InvitationHandler struct {
	service     services.IInvitationService
	userService services.IUserService // Yetki için
}

// NewInvitationHandler yeni bir InvitationHandler örneği oluşturur.
func NewInvitationHandler() *InvitationHandler {
	return &InvitationHandler{
		service:     services.NewInvitationService(),
		userService: services.NewUserService(), // Yetki kontrolü gerekebilir
	}
}

// ListInvitations tüm davetiyeleri listeler (Admin için).
func (h *InvitationHandler) ListInvitations(c *fiber.Ctx) error {
	flashData, _ := flashmessages.GetFlashMessages(c)
	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /*...*/ params = queryparams.DefaultListParams("created_at")}
    params.Validate()

	// Admin tüm davetiyeleri görmeli
	// TODO: Servis katmanına GetAllInvitationsPaginated eklenmeli
	// paginatedResult, err := h.service.GetAllInvitationsPaginated(c.UserContext(), params)
    paginatedResult := &queryparams.PaginatedResult{Data: []models.Invitation{}, Meta: queryparams.PaginationMeta{}} // Geçici
	err := errors.New("tüm davetiyeleri listeleme henüz implemente edilmedi") // Geçici Hata


	renderData := fiber.Map{
		"Title":     "Tüm Davetiyeler",
		"CsrfToken": c.Locals("csrf"), // CSRF token'ı view'a gönder
		"Result":    paginatedResult,
		"Params":    params,
	}
    renderer.SetFlashMessages(renderData, flashData)

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Davetiyeler listelenirken bir hata oluştu."
        renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Invitation{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Dashboard - ListInvitations Error", zap.Error(err))
	}
	// View: dashboard/invitations/list.html
	return renderer.Render(c, "dashboard/invitations/list", "layouts/dashboard_layout", renderData, http.StatusOK)
}

// ShowUpdateInvitation bir davetiyenin düzenleme formunu gösterir (Admin).
func (h *InvitationHandler) ShowUpdateInvitation(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/invitations")}
	invitationID := uint(id)

	// Admin yetkisiyle direkt repo'dan çekebiliriz
	invitation, err := repositories.NewInvitationRepository().FindByID(c.UserContext(), invitationID)
	if err != nil {
		errMsg := "Davetiye bulunamadı."
		if !errors.Is(err, repositories.ErrNotFound) { // Repo hatası
             errMsg = "Davetiye bilgileri alınırken hata oluştu."
             configslog.Log.Error("Dashboard - ShowUpdateInvitation Error", zap.Uint("id", invitationID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/dashboard/invitations")
	}

    formData := flashmessages.GetFlashFormData(c)

	// View: dashboard/invitations/update.html
	return renderer.Render(c, "dashboard/invitations/update", "layouts/dashboard_layout", fiber.Map{
		"Title":      "Davetiyeyi Düzenle (Admin)",
		"Invitation": invitation,
		"Detail":     invitation.Detail,
        "FormData":   formData,
	})
}

// UpdateInvitation bir davetiyeyi günceller (Admin yetkisiyle).
func (h *InvitationHandler) UpdateInvitation(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/invitations")}
	invitationID := uint(id)
	redirectPathOnError := fmt.Sprintf("/dashboard/invitations/update/%d", invitationID)

	var detailUpdates models.InvitationDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        _ = flashmessages.SetFlashFormData(c, detailUpdates) // Hatalı veriyi de ekle
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

	if err := services.ValidateInvitationDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	// Servisi adminUserID ile çağır (servis admini tanımalı)
	err = h.service.UpdateInvitation(c.UserContext(), invitationID, adminUserID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrInvitationNotFound) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/dashboard/invitations")
		}
        // Admin için Forbidden hatası gelmemeli normalde
        if errors.Is(err, services.ErrInvitationForbidden) {
             configslog.Log.Warn("UpdateInvitation: Admin yetkisi reddedildi?", zap.Uint("id", invitationID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
        }
		configslog.Log.Error("Dashboard - UpdateInvitation Error", zap.Uint("id", invitationID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Davetiye başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Başarı sonrası formu tekrar göster
}

// DeleteInvitation bir davetiyeyi siler (Admin yetkisiyle).
func (h *InvitationHandler) DeleteInvitation(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/invitations")}
	invitationID := uint(id)

	// Servisi adminUserID ile çağır
	err = h.service.DeleteInvitation(c.UserContext(), invitationID, adminUserID) // Servis admini tanımalı
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
		if !errors.Is(err, services.ErrInvitationNotFound) {
            configslog.Log.Error("Dashboard - DeleteInvitation Error", zap.Uint("id", invitationID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Davetiye başarıyla silindi.")
	}
	return c.Redirect("/dashboard/invitations", fiber.StatusSeeOther) // Listeye dön
}