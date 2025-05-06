package handlers

import (
	"davet.link/models" // TypeName sabitleri için
	"davet.link/services"
	"davet.link/utils" // Loglama için

	"errors" // Hata kontrolü

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// PublicLinkHandler public link isteklerini yönetir.
type PublicLinkHandler struct {
	linkService        services.ILinkService // Linki bulmak için
	invitationService  services.IInvitationService
	appointmentService services.IAppointmentService
	formService        services.IFormService
	cardService        services.ICardService
}

// NewPublicLinkHandler yeni bir PublicLinkHandler örneği oluşturur.
func NewPublicLinkHandler() *PublicLinkHandler {
	return &PublicLinkHandler{
		linkService:        services.NewLinkService(),
		invitationService:  services.NewInvitationService(),
		appointmentService: services.NewAppointmentService(),
		formService:        services.NewFormService(),
		cardService:        services.NewCardService(),
	}
}

// HandleLink gelen :key parametresine göre ilgili hizmet sayfasını gösterir.
func (h *PublicLinkHandler) HandleLink(c *fiber.Ctx) error {
	key := c.Params("key")
	if len(key) != 20 { // Varsayılan key uzunluğumuz 20 idi
		// Geçersiz key formatı, 404 veya ana sayfaya yönlendir?
		// Veya statik dosya ismiyle çakışıyorsa c.Next() denebilir?
		// Şimdilik 404 verelim.
		return c.Status(fiber.StatusNotFound).SendString("Geçersiz link formatı.")
	}

	// 1. Linki anahtara göre bul
	link, err := h.linkService.GetLinkByKey(key) // Bu metod Type bilgisini de Preload etmeli
	if err != nil {
		if errors.Is(err, services.ErrLinkNotFound) {
			// Link bulunamadı, 404
			return c.Status(fiber.StatusNotFound).Render("errors/404", fiber.Map{"Title": "Link Bulunamadı"}, "layouts/error_layout")
		}
		// Diğer DB hataları
		utils.Log.Error("HandleLink: GetLinkByKey error", zap.String("key", key), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).SendString("Link bilgileri alınırken bir hata oluştu.")
	}

	// 2. Link tipine göre ilgili servisi çağır ve view'ı render et
	ctx := c.UserContext() // Servisler context bekleyebilir
	switch link.Type.Name {
	case models.TypeNameInvitation:
		invitation, invErr := h.invitationService.GetCardByKey(key) // Public erişim metodu
		if invErr != nil {
			if errors.Is(invErr, services.ErrInvitationNotFound) {
				return c.Status(fiber.StatusNotFound).Render("errors/404", fiber.Map{"Title": "Davetiye Bulunamadı/Aktif Değil"}, "layouts/error_layout")
			}
			utils.Log.Error("HandleLink: GetInvitationByKey error", zap.String("key", key), zap.Error(invErr))
			return c.Status(fiber.StatusInternalServerError).SendString("Davetiye yüklenirken hata.")
		}
		// TODO: Şifre kontrolü gerekebilir
		// TODO: View "public/invitation_view.html"
		return c.Render("public/invitation_view", fiber.Map{"Invitation": invitation, "Detail": invitation.Detail}) // Layout belirtilmedi, view kendi layout'unu içerebilir

	case models.TypeNameAppointment:
		appointment, appErr := h.appointmentService.GetAppointmentByKey(key) // Public erişim metodu
		if appErr != nil {                                                   /* ... Hata yönetimi (NotFound vb.) ... */
			return c.SendStatus(fiber.StatusNotFound)
		}
		// TODO: Şifre kontrolü
		// TODO: View "public/appointment_booking.html"
		return c.Render("public/appointment_booking", fiber.Map{"Appointment": appointment, "Detail": appointment.Detail})

	case models.TypeNameForm:
		form, formErr := h.formService.GetFormByKey(key) // Public erişim metodu
		if formErr != nil {                              /* ... Hata yönetimi (NotFound, Closed vb.) ... */
			return c.SendStatus(fiber.StatusNotFound)
		}
		// TODO: Şifre kontrolü
		// TODO: Form alanlarını (FieldDefinitions) da getirmek gerekebilir
		// TODO: View "public/form_fill.html"
		return c.Render("public/form_fill", fiber.Map{"Form": form, "Detail": form.Detail})

	case models.TypeNameCard:
		card, cardErr := h.cardService.GetCardByKey(key) // Public erişim metodu
		if cardErr != nil {                              /* ... Hata yönetimi (NotFound vb.) ... */
			return c.SendStatus(fiber.StatusNotFound)
		}
		// TODO: Şifre kontrolü
		// TODO: View "public/card_view.html"
		return c.Render("public/card_view", fiber.Map{"Card": card, "Detail": card.Detail})

	default:
		// Bilinmeyen link tipi
		utils.Log.Error("HandleLink: Bilinmeyen link tipi", zap.String("key", key), zap.String("type", link.Type.Name))
		return c.Status(fiber.StatusNotFound).SendString("Geçersiz link türü.")
	}
}
