package configssession

import (
	"encoding/gob"
	"time"

	"davet.link/configs/configsenv"
	"davet.link/configs/configslog"
	"davet.link/models"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
)

var Session *session.Store

func InitSession() {
	Session = createSessionStore()
	registerGobTypes()
	configslog.SLog.Info("Oturum (session) sistemi başlatıldı ve utils içinde kayıt edildi.")
}

func SetupSession() *session.Store {
	if Session == nil {
		configslog.SLog.Warn("Session store isteniyor ancak henüz başlatılmamış, şimdi başlatılıyor.")
		InitSession()
	}
	return Session
}

func createSessionStore() *session.Store {
	sessionExpirationHours := configsenv.GetEnvAsInt("SESSION_EXPIRATION_HOURS", 24)
	cookieSecure := configsenv.IsProduction()

	store := session.New(session.Config{
		CookieHTTPOnly: false,
		CookieSecure:   cookieSecure,
		Expiration:     time.Duration(sessionExpirationHours) * time.Hour,
		KeyLookup:      "cookie:session_id",
		CookieSameSite: "Lax",
	})

	configslog.SLog.Info("Cookie tabanlı session sistemi %d saatlik süreyle yapılandırıldı.", sessionExpirationHours)
	return store
}

func registerGobTypes() {
	gob.Register(models.UserType(""))
	gob.Register(&models.User{})
	configslog.SLog.Debug("Session için gob türleri kaydedildi: models.UserType, *models.User")
}

func SessionStart(c *fiber.Ctx) (*session.Session, error) {
	if Session == nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "session store not initialized")
	}
	return Session.Get(c)
}

func GetUserTypeFromSession(sess *session.Session) (models.UserType, error) {
	userType, ok := sess.Get("user_type").(models.UserType)
	if !ok {
		return "", fiber.NewError(fiber.StatusUnauthorized, "Geçersiz oturum veya kullanıcı tipi")
	}
	return userType, nil
}

func GetUserIDFromSession(sess *session.Session) (uint, error) {
	userID, ok := sess.Get("user_id").(uint)
	if !ok {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Geçersiz oturum veya kullanıcı ID'si")
	}
	return userID, nil
}

func GetUserStatusFromSession(sess *session.Session) (bool, error) {
	userStatus, ok := sess.Get("user_status").(bool)
	if !ok {
		return false, fiber.NewError(fiber.StatusUnauthorized, "Geçersiz oturum veya kullanıcı durumu")
	}
	return userStatus, nil
}
