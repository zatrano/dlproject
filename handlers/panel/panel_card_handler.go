package handlers // handlers/panel paketi

import (
	"davet.link/configs/configslog" // Loglama
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/services"
	"errors" // Hata kontrolü
	"fmt"
	"net/http" // Status kodları
	"strconv"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// PanelCardHandler kullanıcının kendi kartvizitleri için handler.
type PanelCardHandler struct {
	service services.ICardService
}

// NewPanelCardHandler yeni bir PanelCardHandler örneği oluşturur.
func NewPanelCardHandler() *PanelCardHandler {
	return &PanelCardHandler{
		service: services.NewCardService(),
	}
}

// ListCards kullanıcının kendi kartvizitlerini listeler.
func (h *PanelCardHandler) ListCards(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Oturum bilgileri geçersiz.")
		return c.Redirect("/auth/login")
	}

	var params queryparams.ListParams
	if err := c.QueryParser(¶ms); err != nil {
		configslog.Log.Warn("Panel ListCards: Query parse error", zap.Error(err))
		params = queryparams.DefaultListParams("created_at") // Default sıralama
	}
	params.Validate() // Sayfa, PerPage vb. limitleri uygula

	paginatedResult, err := h.service.GetCardsForUser(c.UserContext(), userID, params)

	renderData := fiber.Map{
		"Title":     "Kartvizitlerim",
		"Result":    paginatedResult,
		"Params":    params,
	}
	if err != nil {
		renderData[renderer.FlashErrorKeyView] = "Kartvizitler listelenirken bir hata oluştu."
		renderData["Result"] = &queryparams.PaginatedResult{Data: []models.Card{}, Meta: queryparams.PaginationMeta{}}
		configslog.Log.Error("Panel - ListCards Error", zap.Uint("userID", userID), zap.Error(err))
	}
	return renderer.Render(c, "panel/cards/list", "layouts/panel_layout", renderData, http.StatusOK)
}


// ShowCreateCard yeni kartvizit oluşturma formunu gösterir.
func (h *PanelCardHandler) ShowCreateCard(c *fiber.Ctx) error {
	// Hata durumunda formu doldurmak için flash'tan FormData alınabilir
	formData := flashmessages.GetFlashFormData(c)

	return renderer.Render(c, "panel/cards/create", "layouts/panel_layout", fiber.Map{
		"Title":     "Yeni Kartvizit Oluştur",
		"FormData": formData, // Hata sonrası formu doldurmak için
	})
}

// CreateCard yeni kartvizit oluşturur.
func (h *PanelCardHandler) CreateCard(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") } // Güvenlik

	var detail models.CardDetail
	if err := c.BodyParser(&detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
		_ = flashmessages.SetFlashFormData(c, detail) // Hatalı veriyi flash'a kaydet
		return c.Redirect("/panel/cards/create", fiber.StatusSeeOther)
	}

	// Servis validasyonu kullan
	if err := services.ValidateCardDetail(detail); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/cards/create", fiber.StatusSeeOther)
	}

	// Servisi çağır
	_, err := h.service.CreateCard(c.UserContext(), userID, detail) // Context'i ilet
	if err != nil {
		configslog.Log.Error("Panel - CreateCard Error", zap.Uint("creatorUserID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Kartvizit oluşturulamadı: "+err.Error())
		_ = flashmessages.SetFlashFormData(c, detail)
		return c.Redirect("/panel/cards/create", fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kartvizit başarıyla oluşturuldu.")
	return c.Redirect("/panel/cards", fiber.StatusFound)
}

// ShowUpdateCard kartvizit düzenleme formunu gösterir.
func (h *PanelCardHandler) ShowUpdateCard(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/cards")}
	cardID := uint(id)

	// Servis ID ve userID ile çağrılır (yetki kontrolü yapar)
	card, err := h.service.GetCardByID(c.UserContext(), cardID, userID)
	if err != nil {
		errMsg := "Kartvizit bulunamadı veya bu kartviziti düzenleme yetkiniz yok."
		if !errors.Is(err, services.ErrCardNotFound) && !errors.Is(err, services.ErrCardForbidden) {
			 errMsg = "Kartvizit bilgileri alınırken bir hata oluştu."
			 configslog.Log.Error("Panel - ShowUpdateCard Error", zap.Uint("id", cardID), zap.Uint("userID", userID), zap.Error(err))
		}
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/panel/cards")
	}
     formData := flashmessages.GetFlashFormData(c)

	return renderer.Render(c, "panel/cards/update", "layouts/panel_layout", fiber.Map{
		"Title":     "Kartviziti Düzenle",
		"Card":      card,
		"Detail":    card.Detail,
        "FormData": formData, // Hata sonrası formu doldurmak için
	})
}

// UpdateCard kartvizit bilgilerini günceller.
func (h *PanelCardHandler) UpdateCard(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/cards")}
	cardID := uint(id)
	redirectPathOnError := fmt.Sprintf("/panel/cards/update/%d", cardID)

	var detailUpdates models.CardDetail
	if err := c.BodyParser(&detailUpdates); err != nil {
        _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz form verisi.")
        // Hatalı veriyi flash'a kaydetmek zor olabilir, çünkü detailUpdates tam dolmadı.
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
    }
	isEnabledStr := c.FormValue("is_enabled", "false") // Checkbox/Switch değeri
	isEnabled := isEnabledStr == "true" || isEnabledStr == "on"


	if err := services.ValidateCardDetail(detailUpdates); err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, err.Error())
		_ = flashmessages.SetFlashFormData(c, detailUpdates) // Başarılı parse edilen veriyi flash'a kaydet
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	err = h.service.UpdateCard(c.UserContext(), cardID, userID, detailUpdates, isEnabled)
	if err != nil {
		errMsg := "Güncelleme hatası: " + err.Error()
		if errors.Is(err, services.ErrCardNotFound) || errors.Is(err, services.ErrCardForbidden) {
			 _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
			 return c.Redirect("/panel/cards")
		}
		configslog.Log.Error("Panel - UpdateCard Error", zap.Uint("id", cardID), zap.Uint("userID", userID), zap.Error(err))
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		_ = flashmessages.SetFlashFormData(c, detailUpdates)
		return c.Redirect(redirectPathOnError, fiber.StatusSeeOther)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kartvizit başarıyla güncellendi.")
	return c.Redirect(redirectPathOnError, fiber.StatusFound) // Başarı sonrası formu tekrar göster
}

// DeleteCard kartviziti siler.
func (h *PanelCardHandler) DeleteCard(c *fiber.Ctx) error {
	userID, ok := c.Locals("userID").(uint)
	if !ok || userID == 0 { return c.Redirect("/auth/login") }

	id, err := c.ParamsInt("id")
	if err != nil || id <= 0 { _=flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Geçersiz ID."); return c.Redirect("/panel/cards")}
	cardID := uint(id)

	err = h.service.DeleteCard(c.UserContext(), cardID, userID) // Yetki kontrolü serviste
	if err != nil {
		errMsg := "Silme hatası: " + err.Error()
		if !errors.Is(err, services.ErrCardNotFound) && !errors.Is(err, services.ErrCardForbidden) {
             configslog.Log.Error("Panel - DeleteCard Error", zap.Uint("id", cardID), zap.Uint("userID", userID), zap.Error(err))
        }
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
	} else {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kartvizit başarıyla silindi.")
	}
	return c.Redirect("/panel/cards", fiber.StatusSeeOther) // Her durumda listeye dön
}