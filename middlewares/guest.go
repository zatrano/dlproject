package middlewares

import (
	"davet.link/configs/configssession"
	"davet.link/models"
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
)

func GuestMiddleware(c *fiber.Ctx) error {
	sess, err := configssession.SessionStart(c)
	if err != nil {
		return c.Next()
	}

	userID, err := configssession.GetUserIDFromSession(sess)
	if err != nil {
		return c.Next()
	}

	authService := services.NewAuthService()
	user, err := authService.GetUserProfile(userID)
	if err != nil {
		_ = sess.Destroy()
		return c.Next()
	}

	var redirectURL string
	switch user.Type {
	case models.Panel:
		redirectURL = "/panel/home"
	case models.Dashboard:
		redirectURL = "/dashboard/home"
	default:
		_ = sess.Destroy()
		return c.Next()
	}

	return c.Redirect(redirectURL)
}
