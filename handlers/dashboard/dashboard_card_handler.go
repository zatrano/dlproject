package handlers

import (
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/services"
    "davet.link/repositories" // Admin'in direkt ID ile çekmesi için
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// CardHandler kartvizit yönetimi için handler (Dashboard).
type CardHandler struct {
	service     services.ICardService
	userService services.IUserService // Admin yetki kontrolü için (opsiyonel)
}

// NewCardHandler yeni bir CardHandler örneği oluşturur.
func NewCardHandler() *CardHandler {
	return &CardHandler{
		service:     services.NewCardService(),
		userService: services.NewUserService(),
	}
}

// ListCards tüm kartvizitleri listeler (Admin için).
func (h *CardHandler) ListCards(c *fiber.Ctx) error {
	// Admin yetkisi middleware tarafından kontrol edildi.
	flashData, _ := utils.GetFlashMessages(c)
	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil { /* ... */ params = queryparams.DefaultListParams("created_at")}
    params.Validate()

	// TODO: Servise GetAllCardsPaginated(params) eklenmeli
	// paginatedResult, err := h.service.GetAllCardsPaginated(c.UserContext(), params)
    paginatedResult := &queryparams.PaginatedResult{Data: []models.Card{}, Meta: queryparams.PaginationMeta{}} // Geçici
	err := errors.New("servis metodu henüz implemente edilmedi: GetAllCardsPaginated") // Geçici Hata

	renderData := fiber.Map{
		"Title":     "Tüm Kartvizitler",
		"CsrfToken": c.Locals("csrf"),
		"Result":    paginatedResult,
		"Params":    params,
	}
    // Flash mesajlarını render datasına ekle
    renderer.SetFlashMessages(renderData, flashData)

	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Kartvizitler listelenirken hata oluştu."
        renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Card{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Dashboard - ListCards Error", zap.Error(err))
	}
	// View: dashboard/cards/list.html
	return renderer.Render(c, "dashboard/cards/list", "layouts/dashboard_layout", renderData, http.StatusOK)
}

// ShowUpdateCard bir kartvizitin düzenleme formunu gösterir (Admin).
func (h *CardHandler) ShowUpdateCard(c *fiber.Ctx) error {
	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/cards")}
	cardID := uint(id)

	// Admin herhangi bir kartviziti görebilmeli, yetki kontrolü userID olmadan yapılmalı.
	// Servis katmanı AdminGetByID gibi bir metod sağlayabilir veya repo direkt kullanılabilir.
	// Direkt repo kullanalım (servis yetki kontrolü yapmasın diye)
	card, err := repositories.NewCardRepository().FindByID(cardID) // Contextsiz repo çağrısı (sadece admin erişir)
	if err != nil {
		errMsg := "Kartvizit bulunamadı."
		if !errors.Is(err, repositories.ErrNotFound) { // Repo hatasını kontrol et
             errMsg = "Kartvizit bilgileri alınırken hata oluştu."
             configslog.Log.Error("Dashboard - ShowUpdateCard Error", zap.Uint("id", cardID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/dashboard/cards")
	}

    formData := flashmessages.GetFlashFormData(c)

	// View: dashboard/cards/update.html
	return renderer.Render(c, "dashboard/cards/update", "layouts/dashboard_layout", fiber.Map{
		"Title":     "Kartviziti Düzenle (Admin)",
		"Card":      card,
		"Detail":    card.Detail,
        "FormData": formData,
	})
}

// UpdateCard bir kartviziti günceller (Admin yetkisiyle).
func (h *CardHandler) UpdateCard(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") } // Güvenlik

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/cards")}
	cardID := uint(id)
	redirectPathOnError := fmt.Sprintf("/dashboard/cards/update/%d", cardID)

	var detailUpdates models.CardDetail
	if err := c.BodyParser(&detailUpdates); err != nil { /*...*/
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false")
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"

	if err := services.ValidateCardDetail(detailUpdates); err != nil { /*...*/
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
        _ = flashmessages.SetFlashFormData(c, detailUpdates)
        return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }

	// Servisi adminUserID ile çağır (servis IsSystem kontrolü yapmalı)
	err = h.service.UpdateCard(c.UserContext(), cardID, adminUserID, detailUpdates, isEnabled)
	if err != nil { /* ... Hata Yönetimi (NotFound vs) ... */
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrCardNotFound) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/dashboard/cards")
		}
        configslog.Log.Error("Dashboard - UpdateCard Error", zap.Uint("id", cardID), zap.Uint("userID", adminUserID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kartvizit başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound)
}

// DeleteCard bir kartviziti siler (Admin yetkisiyle).
func (h *CardHandler) DeleteCard(c *fiber.Ctx) error {
	adminUserID, ok := c.Locals("userID").(uint)
	if !ok || adminUserID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/dashboard/cards")}
	cardID := uint(id)

	// Servisi adminUserID ile çağır
	err = h.service.DeleteCard(c.UserContext(), cardID, adminUserID)
	if err != nil { /* ... Hata Yönetimi ... */
		errMsg := "Silme hatası: " + err.Error()
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kartvizit başarıyla silindi.")
	}
	return c.Redirect("/dashboard/cards", fiber.StatusSeeOther)
}