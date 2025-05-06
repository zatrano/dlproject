package routes

import (
	handlers "davet.link/handlers/dashboard"
	"davet.link/middlewares"
	"davet.link/models"

	"github.com/gofiber/fiber/v2"
)

func registerDashboardRoutes(app *fiber.App) {
	dashboardGroup := app.Group("/dashboard")
	dashboardGroup.Use(
		middlewares.AuthMiddleware,
		middlewares.StatusMiddleware,
		middlewares.TypeMiddleware(models.Dashboard),
	)

	dashboardHomeHandler := handlers.NewDashboardHomeHandler()
	dashboardGroup.Get("/home", dashboardHomeHandler.HomePage)

	userHandler := handlers.NewUserHandler()
	dashboardGroup.Get("/users", userHandler.ListUsers)
	dashboardGroup.Get("/users/create", userHandler.ShowCreateUser)
	dashboardGroup.Post("/users/create", userHandler.CreateUser)
	dashboardGroup.Get("/users/update/:id", userHandler.ShowUpdateUser)
	dashboardGroup.Post("/users/update/:id", userHandler.UpdateUser)
	dashboardGroup.Delete("/users/delete/:id", userHandler.DeleteUser)
}
