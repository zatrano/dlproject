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

// PanelFormHandler kullanıcının kendi formları için handler.
type PanelFormHandler struct {
	service services.IFormService
}

// NewPanelFormHandler yeni bir PanelFormHandler örneği oluşturur.
func NewPanelFormHandler() *PanelFormHandler {
	return &PanelFormHandler{
		service: services.NewFormService(),
	}
}

// ListForms kullanıcının kendi formlarını listeler.
func (h *PanelFormHandler) ListForms(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint) // CreatorUserID
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /* ... */ params = queryparams.DefaultListParams("created_at")}
    params.Validate()

	paginatedResult, err := h.service.GetFormsForUser(c.UserContext(), userID, params)

	renderData := fiber.Map{
		"Title":     "Formlarım",
		"Result":    paginatedResult,
		"Params":    params,
	}
    renderer.SetFlashMessages(renderData, flashmessages.GetFlashMessages(c))

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Formlar listelenirken hata."
		renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Form{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Panel - ListForms Error", zap.Uint("userID", userID), zap.Error(err))
	}
	// View: panel/forms/list.html
	return renderer.Render(c, "panel/forms/list", "layouts/panel_layout", renderData, http.StatusOK)
}

// ShowCreateForm yeni form oluşturma formunu gösterir.
func (h *PanelFormHandler) ShowCreateForm(c *fiber.Ctx) error {
    formData := flashmessages.GetFlashFormData(c)
	// View: panel/forms/create.html
	return renderer.Render(c, "panel/forms/create", "layouts/panel_layout", fiber.Map{
		"Title":     "Yeni Form Oluştur",
        "FormData": formData,
	})
}

// CreateForm yeni form oluşturur.
func (h *PanelFormHandler) CreateForm(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint) // CreatorUserID
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	var detail models.FormDetail
	if err := c.BodyParser(&detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/forms/create", fiber.StatusSeeOther)
	}

	if err := services.ValidateFormDetail(detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/forms/create", fiber.StatusSeeOther)
	}

	_, err := h.service.CreateForm(c.UserContext(), userID, nil, detail) // OrgID nil
	if err != nil {
		configslog.Log.Error("Panel - CreateForm Error", zap.Uint("creatorUserID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Oluşturma hatası: "+err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/forms/create", fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Form başarıyla oluşturuldu.")
	return c.Redirect("/panel/forms", fiber.StatusFound)
}

// ShowUpdateForm form düzenleme formunu gösterir.
func (h *PanelFormHandler) ShowUpdateForm(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/forms")}
	formID := uint(id)

	form, err := h.service.GetFormByID(c.UserContext(), formID, userID) // Yetki kontrolü yapar
	if err != nil {
		errMsg := "Form bulunamadı veya bu formu düzenleme yetkiniz yok."
		if !errors.Is(err, services.ErrFormNotFound) && !errors.Is(err, services.ErrFormForbidden) {
			 errMsg = "Form bilgileri alınırken bir hata oluştu."
			 configslog.Log.Error("Panel - ShowUpdateForm Error", zap.Uint("id", formID), zap.Uint("userID", userID), zap.Error(err))
		}
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/panel/forms")
	}

    formData := flashmessages.GetFlashFormData(c)

	// View: panel/forms/update.html
	return renderer.Render(c, "panel/forms/update", "layouts/panel_layout", fiber.Map{
		"Title":     "Formu Düzenle",
		"Form":      form,
		"Detail":    form.Detail,
        "FormData": formData,
	})
}

// UpdateForm form bilgilerini günceller.
func (h *PanelFormHandler) UpdateForm(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/forms")}
	formID := uint(id)
	redirectPathOnError := fmt.Sprintf("/panel/forms/update/%d", formID)

	var detailUpdates models.FormDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        _ = flashmessages.SetFlashFormData(c, detailUpdates)
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

    // Şifre alanını ayrıca işle (boşsa güncelleme)
    passwordInput := c.FormValue("password")
	if passwordInput != "" {
		detailUpdates.PasswordHash = passwordInput // Hashlenecek
	} else {
        // Mevcut hash'i korumak için BodyParser'dan gelen boş değeri yok say
        // Servis katmanı bu kontrolü yapmalı veya biz burada mevcut değeri çekip atamalıyız.
        // Şimdilik servise bırakalım.
        detailUpdates.PasswordHash = "" // Hashlenmeyecek şekilde işaretle? Veya serviste kontrol et.
    }


	if err := services.ValidateFormDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	// Servisi çağır (userID ile, servis yetki kontrolü yapar)
	err = h.service.UpdateForm(c.UserContext(), formID, userID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrFormNotFound) || errors.Is(err, services.ErrFormForbidden) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/panel/forms")
		}
        configslog.Log.Error("Panel - UpdateForm Error", zap.Uint("id", formID), zap.Uint("userID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Form başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Başarı sonrası formu tekrar göster
}

// DeleteForm formu siler.
func (h *PanelFormHandler) DeleteForm(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/forms")}
	formID := uint(id)

	// Servisi çağır (userID ile, servis yetki kontrolü yapar)
	err = h.service.DeleteForm(c.UserContext(), formID, userID)
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
        if !errors.Is(err, services.ErrFormNotFound) && !errors.Is(err, services.ErrFormForbidden) {
             configslog.Log.Error("Panel - DeleteForm Error", zap.Uint("id", formID), zap.Uint("userID", userID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Form başarıyla silindi.")
	}
	return c.Redirect("/panel/forms", fiber.StatusSeeOther) // Her durumda listeye dön
}