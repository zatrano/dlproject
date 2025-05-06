package routes

import (
	"davet.link/configs" // Session ve CSRF konfigürasyonu için (SetupCSRF referansı)
	// Public Link Handler için
	// Genel middleware'ler ve rootRedirector için (varsa)
	// rootRedirector için (eğer tip kontrolü yapıyorsa - ki artık yapmamalı)
	"davet.link/utils" // rootRedirector içindeki session yardımcıları için

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	recoverMiddleware "github.com/gofiber/fiber/v2/middleware/recover"
	// "gorm.io/gorm" // Artık DB parametresi yok
	// rootRedirector içindeki loglama için (eğer varsa)
)

// SetupRoutes tüm uygulama rotalarını ve genel middleware'leri ayarlar.
func SetupRoutes(app *fiber.App) {
	// --- Genel Middleware'ler ---
	app.Use(recoverMiddleware.New()) // Panic yakalama
	app.Use(logger.New())            // İstek loglama
	// CSRF middleware'i main.go'da veya burada eklenebilir (Session'dan önce gelmeli)
	// app.Use(configs.SetupCSRF())
	app.Use(initializeSessionAndLocals()) // Session ve temel locals ayarları

	// --- Rota Grupları ---
	registerAuthRoutes(app)      // /auth rotaları
	registerDashboardRoutes(app) // /dashboard rotaları
	registerPanelRoutes(app)     // /panel rotaları

	// --- Public Link Rotası ---
	// Diğer özel gruplardan sonra gelmeli
	registerPublicLinkRoutes(app)

	// --- Kök URL ("/") Yönlendirmesi ---
	// Bu, public link rotasından sonra gelirse, public link key "/" olamaz.
	// Eğer public link "/" olabilecekse, bu yönlendiriciyi public link handler'ının
	// içine taşımak veya public link handler'ını önce çalıştırmak gerekebilir.
	// Şimdilik burada bırakalım.
	app.Get("/", rootRedirector)

	// --- 404 Handler ---
	// En sonda, eşleşmeyen tüm rotaları yakalar.
	app.Use(notFoundHandler)
}

// initializeSessionAndLocals (Önceki cevapta verilen haliyle - Değişiklik Yok)
func initializeSessionAndLocals() fiber.Handler {
	sessionStore := configs.SetupSession()
	return func(c *fiber.Ctx) error {
		c.Locals("session_store", sessionStore)
		sess, err := utils.SessionStart(c)
		if err != nil {
			return c.Next()
		}
		userID, idErr := utils.GetUserIDFromSession(sess)
		isSystem, sysErr := utils.GetIsSystemFromSession(sess) // Bu fonksiyon utils'de olmalı
		userName, nameOk := sess.Get("user_name").(string)
		if idErr == nil {
			c.Locals("userID", userID)
		}
		if sysErr == nil {
			c.Locals("isSystem", isSystem)
		}
		if nameOk {
			c.Locals("userName", userName)
		}
		return c.Next()
	}
}

// rootRedirector (Önceki cevapta verilen haliyle - Değişiklik Yok)
func rootRedirector(c *fiber.Ctx) error {
	userIDRaw := c.Locals("userID")
	isSystemRaw := c.Locals("isSystem")
	if userIDRaw == nil || isSystemRaw == nil {
		return c.Redirect("/auth/login", fiber.StatusTemporaryRedirect)
	}
	isSystem, ok := isSystemRaw.(bool)
	if !ok { /* ... Hata, Session Temizle, Login'e Yönlendir ... */
		return c.Redirect("/auth/login")
	}
	if isSystem {
		return c.Redirect("/dashboard/home", fiber.StatusFound)
	}
	return c.Redirect("/panel/home", fiber.StatusFound)
}

// notFoundHandler (Önceki cevapta verilen haliyle - Değişiklik Yok)
func notFoundHandler(c *fiber.Ctx) error {
	accepts := c.Accepts("application/json", "text/html")
	switch accepts {
	case "application/json":
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Kaynak bulunamadı"})
	default:
		return c.Status(fiber.StatusNotFound).Render("errors/404", fiber.Map{"Title": "Sayfa Bulunamadı"}, "layouts/error_layout")
	}
}

// Not: registerAuthRoutes, registerDashboardRoutes, registerPanelRoutes
// fonksiyonları kendi ayrı dosyalarında (auth_route.go, dashboard_route.go, panel_route.go)
// tanımlanmış olmalıdır.
