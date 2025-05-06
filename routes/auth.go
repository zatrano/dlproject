package routes

import (
	handlers "davet.link/handlers/auth"
	"davet.link/middlewares"

	"github.com/gofiber/fiber/v2"
)

func registerAuthRoutes(app *fiber.App) {
	authHandler := handlers.NewAuthHandler()

	authGroup := app.Group("/auth")

	authGroup.Get("/login", middlewares.GuestMiddleware, authHandler.ShowLogin)
	authGroup.Post("/login", middlewares.GuestMiddleware, authHandler.Login)

	authGroup.Get("/logout", middlewares.AuthMiddleware, authHandler.Logout)
	authGroup.Get("/profile", middlewares.AuthMiddleware, authHandler.Profile)
	authGroup.Post("/profile/update-password", middlewares.AuthMiddleware, authHandler.UpdatePassword)
}
