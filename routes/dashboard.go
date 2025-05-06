package routes

import (
	handlers "davet.link/handlers/dashboard" // Dashboard handler'ları
	"davet.link/middlewares"

	// "davet.link/models" // Artık middleware için gerekli değil

	"github.com/gofiber/fiber/v2"
)

// registerDashboardRoutes /dashboard altındaki rotaları ve middleware'leri tanımlar.
// Sadece IsSystem=true olan kullanıcılar erişebilir.
func registerDashboardRoutes(app *fiber.App) {
	// Handler instance'larını başta oluştur
	homeHandler := handlers.NewHomeHandler()
	userHandler := handlers.NewUserHandler()
	invitationHandler := handlers.NewInvitationHandler()
	appointmentHandler := handlers.NewAppointmentHandler()
	formHandler := handlers.NewFormHandler()
	cardHandler := handlers.NewCardHandler() // Yeni Card handler

	// Grup oluştur ve middleware'leri uygula
	dashboardGroup := app.Group("/dashboard")
	dashboardGroup.Use(
		middlewares.AuthMiddleware,   // 1. Giriş yapmış mı?
		middlewares.StatusMiddleware, // 2. Hesap aktif mi?
		middlewares.RequireSystem(),  // 3. Sistem yöneticisi mi?
	)

	// --- Ana Sayfa ---
	dashboardGroup.Get("/home", homeHandler.HomePage) // GET /dashboard/home

	// --- Kullanıcı Yönetimi ---
	dashboardGroup.Get("/users", userHandler.ListUsers)                 // GET /dashboard/users
	dashboardGroup.Get("/users/create", userHandler.ShowCreateUser)     // GET /dashboard/users/create
	dashboardGroup.Post("/users/create", userHandler.CreateUser)        // POST /dashboard/users/create
	dashboardGroup.Get("/users/update/:id", userHandler.ShowUpdateUser) // GET /dashboard/users/update/{id}
	dashboardGroup.Post("/users/update/:id", userHandler.UpdateUser)    // POST /dashboard/users/update/{id}
	dashboardGroup.Post("/users/delete/:id", userHandler.DeleteUser)    // POST /dashboard/users/delete/{id} (Form için)
	dashboardGroup.Delete("/users/delete/:id", userHandler.DeleteUser)  // DELETE /dashboard/users/delete/{id} (API/JS için)

	// --- Davetiye Yönetimi (Admin Görünümü) ---
	// Adminin tüm davetiyeleri yönettiği varsayılıyor.
	dashboardGroup.Get("/invitations", invitationHandler.ListInvitations)                 // GET /dashboard/invitations
	dashboardGroup.Get("/invitations/update/:id", invitationHandler.ShowUpdateInvitation) // GET /dashboard/invitations/update/{id}
	dashboardGroup.Post("/invitations/update/:id", invitationHandler.UpdateInvitation)    // POST /dashboard/invitations/update/{id}
	dashboardGroup.Post("/invitations/delete/:id", invitationHandler.DeleteInvitation)    // POST /dashboard/invitations/delete/{id}
	dashboardGroup.Delete("/invitations/delete/:id", invitationHandler.DeleteInvitation)  // DELETE /dashboard/invitations/delete/{id}

	// --- Randevu Hizmeti Yönetimi (Admin Görünümü) ---
	dashboardGroup.Get("/appointments", appointmentHandler.ListAppointments)                 // GET /dashboard/appointments
	dashboardGroup.Get("/appointments/update/:id", appointmentHandler.ShowUpdateAppointment) // GET /dashboard/appointments/update/{id}
	dashboardGroup.Post("/appointments/update/:id", appointmentHandler.UpdateAppointment)    // POST /dashboard/appointments/update/{id}
	dashboardGroup.Post("/appointments/delete/:id", appointmentHandler.DeleteAppointment)    // POST /dashboard/appointments/delete/{id}
	dashboardGroup.Delete("/appointments/delete/:id", appointmentHandler.DeleteAppointment)  // DELETE /dashboard/appointments/delete/{id}

	// --- Form Yönetimi (Admin Görünümü) ---
	dashboardGroup.Get("/forms", formHandler.ListForms)                 // GET /dashboard/forms
	dashboardGroup.Get("/forms/update/:id", formHandler.ShowUpdateForm) // GET /dashboard/forms/update/{id}
	dashboardGroup.Post("/forms/update/:id", formHandler.UpdateForm)    // POST /dashboard/forms/update/{id}
	dashboardGroup.Post("/forms/delete/:id", formHandler.DeleteForm)    // POST /dashboard/forms/delete/{id}
	dashboardGroup.Delete("/forms/delete/:id", formHandler.DeleteForm)  // DELETE /dashboard/forms/delete/{id}

	// --- Kartvizit Yönetimi (Admin Görünümü) ---
	dashboardGroup.Get("/cards", cardHandler.ListCards)                 // GET /dashboard/cards
	dashboardGroup.Get("/cards/update/:id", cardHandler.ShowUpdateCard) // GET /dashboard/cards/update/{id}
	dashboardGroup.Post("/cards/update/:id", cardHandler.UpdateCard)    // POST /dashboard/cards/update/{id}
	dashboardGroup.Post("/cards/delete/:id", cardHandler.DeleteCard)    // POST /dashboard/cards/delete/{id}
	dashboardGroup.Delete("/cards/delete/:id", cardHandler.DeleteCard)  // DELETE /dashboard/cards/delete/{id}

}
