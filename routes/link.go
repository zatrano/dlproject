package routes

import (
	handlers "davet.link/handlers" // Public Link Handler'ı içeren paket (veya handlers/public)

	"github.com/gofiber/fiber/v2"
)

// registerPublicLinkRoutes public linkleri (örn. /abcdef12345) yönetecek rotayı tanımlar.
func registerPublicLinkRoutes(app *fiber.App) {
	// Public link handler'ından bir örnek oluştur
	publicHandler := handlers.NewPublicLinkHandler() // Bu handler oluşturulmalı

	// Ana rota: :key parametresi ile link anahtarını yakala
	// Bu rota diğer özel rotalardan (örn. /auth, /dashboard) SONRA tanımlanmalı.
	app.Get("/:key", publicHandler.HandleLink)

	// Belki linklere özel başka alt rotalar da olabilir?
	// Örnek: app.Post("/:key/rsvp", publicHandler.HandleRsvpPost)
}
