package configscsrf

import (
	"strings"
	"time"
	"davet.link/configs/configslog"
	"davet.link/pkg/flashmessages"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/utils"
	"go.uber.org/zap"
)

var csrfExemptPaths = []string{
	// "rotalar",
}

func SetupCSRF() fiber.Handler {
	config := csrf.Config{
		KeyLookup:      "header:X-CSRF-Token",
		CookieName:     "csrf_",
		CookieHTTPOnly: true,
		CookieSecure:   false,
		CookieSameSite: "Lax",
		Expiration:     1 * time.Hour,
		KeyGenerator:   utils.UUID,
		ContextKey:     "csrf",
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			configslog.Log.Warn("CSRF validation failed",
				zap.Error(err),
				zap.String("ip", c.IP()),
				zap.String("path", c.Path()),
				zap.String("method", c.Method()),
			)
			_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Güvenlik doğrulaması başarısız oldu. Lütfen sayfayı yenileyip tekrar deneyin.")
			return c.Redirect("/auth/login", fiber.StatusSeeOther)
		},
		Next: func(c *fiber.Ctx) bool {
			token := c.Get("X-CSRF-Token")
			if token == "" {
				token = c.FormValue("csrf_token")
				if token != "" {
					c.Request().Header.Set("X-CSRF-Token", token)
				}
			}

			path := c.Path()
			for _, exemptPath := range csrfExemptPaths {
				if strings.HasPrefix(path, exemptPath) {
					configslog.Log.Debug("CSRF koruması atlanıyor (Next)", zap.String("path", path))
					return true
				}
			}
			return false
		},
	}

	configslog.SLog.Info("CSRF middleware yapılandırıldı", zap.Strings("exempt_paths", csrfExemptPaths))
	return csrf.New(config)
}
