package handlers

import (
	"errors" // Hata kontrolü

	"davet.link/configs/configslog" // Loglama
	"davet.link/models"
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	// "gorm.io/gorm" // Gerekli değil
)

// LinkHandler public link isteklerini yönetir.
type LinkHandler struct {
	linkService        services.ILinkService // Linki bulmak ve doğrulamak için
	invitationService  services.IInvitationService
	appointmentService services.IAppointmentService
	formService        services.IFormService
	cardService        services.ICardService
	// TODO: Gerekirse IAuthService (örn. şifreli linkler için)
}

// NewLinkHandler yeni bir LinkHandler örneği oluşturur.
func NewLinkHandler() *LinkHandler {
	return &LinkHandler{
		linkService:        services.NewLinkService(),
		invitationService:  services.NewInvitationService(),
		appointmentService: services.NewAppointmentService(),
		formService:        services.NewFormService(),
		cardService:        services.NewCardService(),
	}
}

// HandleLink gelen :key parametresine göre ilgili hizmet sayfasını gösterir.
func (h *LinkHandler) HandleLink(c *fiber.Ctx) error {
	key := c.Params("key")
	// Key uzunluğunu ve formatını başta kontrol etmek iyi bir pratik
	if len(key) != 20 { // Modeldeki Key uzunluğu ile aynı olmalı
		configslog.SLog.Warnf("Geçersiz formatta link anahtarı denendi: %s", key)
		return h.renderNotFound(c, "Geçersiz Link") // 404 sayfası
	}

	// 1. Linki anahtara göre bul (servis üzerinden)
	// Bu metod sadece linkin varlığını kontrol eder, aktiflik/süre kontrolü yapmaz.
	ctx := c.UserContext() // İstek context'ini al
	link, err := h.linkService.GetLinkByKey(ctx, key)
	if err != nil {
		if errors.Is(err, services.ErrLinkNotFound) {
			return h.renderNotFound(c, "Link Bulunamadı") // 404
		}
		// Diğer link servisi veya DB hataları
		configslog.Log.Error("HandleLink: GetLinkByKey error", zap.String("key", key), zap.Error(err))
		return h.renderError(c, "Link bilgileri alınırken bir sorun oluştu.") // 500 sayfası
	}

	// 2. Link tipine göre ilgili hizmet servisini çağır ve view'ı render et
	// Hizmet servisleri kendi içlerinde IsEnabled, ExpiresAt gibi kontrolleri yapmalı.
	switch link.Type.Name {
	case models.TypeNameInvitation:
		// GetInvitationByKey metodu aktiflik vb. kontrolleri yapmalı
		invitation, invErr := h.invitationService.GetCardByKey(key) // Bu metod public erişim için olmalı
		if invErr != nil {
			if errors.Is(invErr, services.ErrInvitationNotFound) {
				return h.renderNotFound(c, "Davetiye Bulunamadı")
			}
			configslog.Log.Error("HandleLink: GetInvitationByKey error", zap.String("key", key), zap.Error(invErr))
			return h.renderError(c, "Davetiye yüklenirken bir sorun oluştu.")
		}
		// TODO: Şifre kontrolü: Eğer invitation.Detail.PasswordHash varsa, şifre formu göster/kontrol et
		// TODO: View "public/invitation_view.html"
		return c.Render("public/invitation_view", fiber.Map{"Invitation": invitation, "Detail": invitation.Detail})

	case models.TypeNameAppointment:
		appointment, appErr := h.appointmentService.GetAppointmentByKey(key)
		if appErr != nil {
			if errors.Is(appErr, services.ErrAppointmentNotFound) {
				return h.renderNotFound(c, "Randevu Hizmeti Bulunamadı")
			}
			configslog.Log.Error("HandleLink: GetAppointmentByKey error", zap.String("key", key), zap.Error(appErr))
			return h.renderError(c, "Randevu hizmeti yüklenirken bir sorun oluştu.")
		}
		// TODO: Şifre kontrolü
		// TODO: View "public/appointment_booking.html"
		return c.Render("public/appointment_booking", fiber.Map{"Appointment": appointment, "Detail": appointment.Detail})

	case models.TypeNameForm:
		form, formErr := h.formService.GetFormByKey(key)
		if formErr != nil {
			if errors.Is(formErr, services.ErrFormNotFound) {
				return h.renderNotFound(c, "Form Bulunamadı")
			}
			configslog.Log.Error("HandleLink: GetFormByKey error", zap.String("key", key), zap.Error(formErr))
			return h.renderError(c, "Form yüklenirken bir sorun oluştu.")
		}
		// TODO: Şifre kontrolü
		// TODO: Form alanları (FieldDefinitions) servisten gelmeli
		// TODO: View "public/form_fill.html"
		return c.Render("public/form_fill", fiber.Map{"Form": form, "Detail": form.Detail, "CsrfToken": c.Locals("csrf")}) // Form gönderimi için CSRF

	case models.TypeNameCard:
		card, cardErr := h.cardService.GetCardByKey(key)
		if cardErr != nil {
			if errors.Is(cardErr, services.ErrCardNotFound) {
				return h.renderNotFound(c, "Kartvizit Bulunamadı")
			}
			configslog.Log.Error("HandleLink: GetCardByKey error", zap.String("key", key), zap.Error(cardErr))
			return h.renderError(c, "Kartvizit yüklenirken bir sorun oluştu.")
		}
		// TODO: Şifre kontrolü
		// TODO: View "public/card_view.html"
		return c.Render("public/card_view", fiber.Map{"Card": card, "Detail": card.Detail})

	default:
		// Bilinmeyen veya desteklenmeyen link tipi
		configslog.Log.Error("HandleLink: Bilinmeyen link tipi", zap.String("key", key), zap.String("type", link.Type.Name))
		return h.renderNotFound(c, "Geçersiz Link Türü")
	}
}

// renderNotFound standart 404 sayfasını render eder.
func (h *LinkHandler) renderNotFound(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).Render("errors/404", fiber.Map{
		"Title":   "Bulunamadı",
		"Message": message,
	}, "layouts/error_layout") // Veya public layout
}

// renderError standart 500 hata sayfasını render eder.
func (h *LinkHandler) renderError(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusInternalServerError).Render("errors/500", fiber.Map{
		"Title":   "Sunucu Hatası",
		"Message": message,
	}, "layouts/error_layout") // Veya public layout
}
