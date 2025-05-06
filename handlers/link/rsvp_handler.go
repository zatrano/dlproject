// handlers/public/public_rsvp_handler.go (YENİ DOSYA)
package handlers // veya handlers/public

import (
	"errors"

	"davet.link/configs/configslog"
	"davet.link/models" // Belki gerekmez, JSON dönülebilir
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// PublicRSVPHandler public RSVP işlemleri için handler.
type PublicRSVPHandler struct {
	invitationService services.IInvitationService
	// Belki Captcha servisi?
}

// NewPublicRSVPHandler yeni bir PublicRSVPHandler örneği oluşturur.
func NewPublicRSVPHandler() *PublicRSVPHandler {
	return &PublicRSVPHandler{
		invitationService: services.NewInvitationService(),
	}
}

// ShowRSVPForm (GET /{key}/rsvp veya GET /rsvp/{key})
// Davetiyeyi bulur ve RSVP formunu gösterir. Misafir ID/Kod da parametre olarak gelebilir.
func (h *PublicRSVPHandler) ShowRSVPForm(c *fiber.Ctx) error {
	key := c.Params("key")
	guestIdentifier := c.Query("guest") // Query param ?guest=CODE veya ?guest=ID

	invitation, err := h.invitationService.GetInvitationByKey(c.UserContext(), key)
	if err != nil {
		// Handle NotFound etc. -> 404 sayfası göster
		return c.Status(fiber.StatusNotFound).Render("errors/404", fiber.Map{"Title": "Davetiye Bulunamadı"})
	}

	// TODO: Misafir bilgilerini (guestIdentifier ile) alıp forma gönderebiliriz.
	// guest, _ := invitationService.GetGuestByIdentifier(invitation.ID, guestIdentifier)

	// TODO: View "public/rsvp_form.html"
	return c.Render("public/rsvp_form", fiber.Map{
		"Title":           "LCV: " + invitation.Detail.Title,
		"Invitation":      invitation,
		"Detail":          invitation.Detail,
		"GuestIdentifier": guestIdentifier,  // Formun hangi misafir için olduğunu bilmesi için
		"CsrfToken":       c.Locals("csrf"), // CSRF token (public formda gerekli mi? Captcha daha iyi olabilir)
	}) // Layout?
}

// SubmitRSVP (POST /{key}/rsvp veya POST /rsvp/{key})
// Formdan gelen RSVP verisini işler.
func (h *PublicRSVPHandler) SubmitRSVP(c *fiber.Ctx) error {
	key := c.Params("key")
	guestIdentifier := c.FormValue("guest_identifier") // Formdan hidden input ile alınır

	var rsvpData models.InvitationRSVP
	// Formdan sadece Status, PlusOnes, Notes alınır.
	if err := c.BodyParser(&rsvpData); err != nil {
		configslog.Log.Warn("SubmitRSVP: Form verisi parse edilemedi", zap.Error(err))
		// Genellikle JSON yanıt dönmek daha iyi olabilir
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Geçersiz veri."})
		// Veya flash mesajla forma geri yönlendir?
		// _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Lütfen formu kontrol edin.")
		// return c.Redirect(fmt.Sprintf("/%s/rsvp?guest=%s", key, guestIdentifier), fiber.StatusSeeOther)
	}

	// Servisi çağır
	err := h.invitationService.SubmitRSVP(c.UserContext(), key, guestIdentifier, rsvpData)
	if err != nil {
		errMsg := "LCV gönderilirken bir hata oluştu: " + err.Error()
		statusCode := fiber.StatusInternalServerError
		if errors.Is(err, services.ErrInvitationNotFound) || errors.Is(err, services.ErrGuestNotFoundForRSVP) {
			statusCode = fiber.StatusNotFound
		} else if errors.Is(err, services.ErrRSVPDeadlinePassed) || errors.Is(err, services.ErrInvalidRSVPStatus) ||
			errors.Is(err, services.ErrPlusOnesNotAllowed) || errors.Is(err, services.ErrMaxPlusOnesExceeded) {
			statusCode = fiber.StatusBadRequest
		}
		configslog.Log.Error("SubmitRSVP Error", zap.String("key", key), zap.String("guest", guestIdentifier), zap.Error(err))
		return c.Status(statusCode).JSON(fiber.Map{"error": errMsg})
		// Veya flash mesajla forma geri yönlendir?
		// _ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		// return c.Redirect(fmt.Sprintf("/%s/rsvp?guest=%s", key, guestIdentifier), fiber.StatusSeeOther)
	}

	// Başarılı yanıt
	// TODO: Teşekkürler sayfası veya mesajı göster
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"message": "LCV yanıtınız başarıyla alındı."})
	// Veya
	// _ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "LCV yanıtınız başarıyla alındı.")
	// return c.Redirect(fmt.Sprintf("/%s/rsvp/thankyou", key)) // Teşekkürler sayfasına yönlendir
}
