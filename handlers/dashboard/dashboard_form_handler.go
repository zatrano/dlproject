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

// FormHandler form yönetimi için handler (Dashboard).
type FormHandler struct {
	service     services.IFormService
	userService services.IUserService
}

// NewFormHandler yeni bir FormHandler örneği oluşturur.
func NewFormHandler() *FormHandler {
	return &FormHandler{
		service:     services.NewFormService(),
		userService: services.NewUserService(),
	}
}

// ListForms tüm formları listeler (Admin için).
func (h *FormHandler) ListForms(c *fiber.Ctx) error {
	flashData, _ := flashmessages.GetFlashMessages(c)
	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /* ... */ params = queryparams.DefaultListParams("created_at")}
    params.Validate()

	// Admin tüm formları görmeli
	// TODO: Servis katmanına GetAllFormsPaginated eklenmeli
	// paginatedResult, err := h.service.GetAllFormsPaginated(c.UserContext(), params)
    paginatedResult := &queryparams.PaginatedResult{Data: []models.Form{}, Meta: queryparams.PaginationMeta{}} // Geçici
	err := errors.New("tüm formları listeleme henüz implemente edilmedi") // Geçici Hata


	renderData := fiber.Map{
		"Title":     "Tüm Formlar",
		"CsrfToken": c.Locals("csrf"),
		"Result":    paginatedResult,
		"Params":    params,
	}
    renderer.SetFlashMessages(renderData, flashData)

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Formlar listelenirken hata oluştu."
        renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Form{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Dashboard - ListForms Error", zap.Error(err))
	}
	// View: dashboard/forms/list.html
	return renderer.Render(c, "dashboard/forms/list", "layouts/dashboard_layout", renderData, http.StatusOK)
}

// ShowUpdateForm bir formun düzenleme formunu gösterir (Admin).
func (h *FormHandler) ShowUpdateForm(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/forms")}
	formID := uint(id)

	// Admin yetkisiyle direkt repo'dan çek
	form, err := repositories.NewFormRepository().FindByID(c.UserContext(), formID)
	if err != nil {
		errMsg := "Form bulunamadı."
		if !errors.Is(err, repositories.ErrNotFound) { // Repo hatası
             errMsg = "Form bilgileri alınırken hata oluştu."
             configslog.Log.Error("Dashboard - ShowUpdateForm Error", zap.Uint("id", formID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/dashboard/forms")
	}

    formData := flashmessages.GetFlashFormData(c)

	// View: dashboard/forms/update.html
	return renderer.Render(c, "dashboard/forms/update", "layouts/dashboard_layout", fiber.Map{
		"Title":     "Formu Düzenle (Admin)",
		"Form":      form,
		"Detail":    form.Detail,
        "FormData":  formData,
	})
}

// UpdateForm bir formu günceller (Admin yetkisiyle).
func (h *FormHandler) UpdateForm(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/forms")}
	formID := uint(id)
	redirectPathOnError := fmt.Sprintf("/dashboard/forms/update/%d", formID)

	var detailUpdates models.FormDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        _ = flashmessages.SetFlashFormData(c, detailUpdates)
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

	// Şifre alanı için özel işlem (boş gelirse null yapmamak için)
	passwordInput := c.FormValue("password") // Formdan ham şifreyi al
	if passwordInput != "" {
		detailUpdates.PasswordHash = passwordInput // Hashlenecek şifreyi ata
	} else {
		// Eğer şifre alanı boş bırakılmışsa, detailUpdates.PasswordHash'in
        // mevcut hash'i ezmemesi için onu boş bırakmalıyız. BodyParser bunu
        // zaten yapar ama emin olmak için kontrol edebiliriz.
        // Ya da servis katmanında eğer detailData.PasswordHash boşsa güncelleme.
	}


	if err := services.ValidateFormDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	// Servisi adminUserID ile çağır
	err = h.service.UpdateForm(c.UserContext(), formID, adminUserID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrFormNotFound) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/dashboard/forms")
		}
        // Admin için Forbidden gelmemeli
		configslog.Log.Error("Dashboard - UpdateForm Error", zap.Uint("id", formID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Form başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Formu tekrar göster
}

// DeleteForm bir formu siler (Admin yetkisiyle).
func (h *FormHandler) DeleteForm(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/forms")}
	formID := uint(id)

	// Servisi adminUserID ile çağır
	err = h.service.DeleteForm(c.UserContext(), formID, adminUserID)
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
        if !errors.Is(err, services.ErrFormNotFound) {
            configslog.Log.Error("Dashboard - DeleteForm Error", zap.Uint("id", formID), zap.Uint("adminUserID", adminUserID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Form başarıyla silindi.")
	}
	return c.Redirect("/dashboard/forms", fiber.StatusSeeOther) // Listeye dön
}