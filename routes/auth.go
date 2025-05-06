package routes

import (
	auth_handlers "davet.link/handlers/auth" // İsim çakışmasını önlemek için alias
	"davet.link/middlewares"
	"github.com/gofiber/fiber/v2"
)

func registerAuthRoutes(app *fiber.App) {
	authHandler := auth_handlers.NewAuthHandler()
	authGroup := app.Group("/auth")

	guestRoutes := authGroup.Group("")
	guestRoutes.Use(middlewares.GuestMiddleware)
	guestRoutes.Get("/login", authHandler.ShowLogin)
	guestRoutes.Post("/login", authHandler.Login)
	guestRoutes.Get("/register", authHandler.ShowRegister)
	guestRoutes.Post("/register", authHandler.Register)
	guestRoutes.Get("/password/request", authHandler.ShowForgotPassword)
	guestRoutes.Post("/password/request", authHandler.RequestPasswordReset)
	guestRoutes.Get("/password/reset/:token", authHandler.ShowResetPassword)
	guestRoutes.Post("/password/reset", authHandler.ResetPassword)

	authGroup.Get("/verify-email", authHandler.VerifyEmail)

	userRoutes := authGroup.Group("")
	userRoutes.Use(middlewares.AuthMiddleware)
	userRoutes.Get("/logout", authHandler.Logout)
	userRoutes.Post("/logout", authHandler.Logout)
	userRoutes.Get("/profile", authHandler.Profile)
	userRoutes.Post("/profile/update-password", authHandler.UpdatePassword)
	// userRoutes.Post("/verify-email/resend", authHandler.ResendVerificationEmail)
}
