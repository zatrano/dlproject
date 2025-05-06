package middlewares

import (
	"context"
	"davet.link/configs/configssession"
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
)

func AuthMiddleware(c *fiber.Ctx) error {
	sess, err := configssession.SessionStart(c)
	if err != nil {
		return c.Redirect("/auth/login")
	}

	userID, err := configssession.GetUserIDFromSession(sess)
	if err != nil {
		return c.Redirect("/auth/login")
	}

	authService := services.NewAuthService()
	_, err = authService.GetUserProfile(userID)
	if err != nil {
		_ = sess.Destroy()
		return c.Redirect("/auth/login")
	}

	ctx := context.WithValue(c.Context(), "user_id", userID)
	c.SetUserContext(ctx)

	return c.Next()
}
