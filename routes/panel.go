package routes

import (
	// Panel handler'larını import et
	panel_handlers "davet.link/handlers/panel"
	"davet.link/middlewares"

	"github.com/gofiber/fiber/v2"
)

// registerPanelRoutes /panel altındaki rotaları ve middleware'leri tanımlar.
// Sadece normal kullanıcıların (IsSystem == false) erişimine izin verilir.
func registerPanelRoutes(app *fiber.App) {
	// Handler instance'larını başta oluştur
	panelHomeHandler := panel_handlers.NewPanelHomeHandler()
	invitationHandler := panel_handlers.NewPanelInvitationHandler()
	appointmentHandler := panel_handlers.NewPanelAppointmentHandler()
	formHandler := panel_handlers.NewPanelFormHandler()
	cardHandler := panel_handlers.NewPanelCardHandler() // Yeni Card handler

	// /panel grubu oluştur ve middleware'leri uygula
	panelGroup := app.Group("/panel")
	panelGroup.Use(
		middlewares.AuthMiddleware,   // 1. Giriş yapmış mı?
		middlewares.StatusMiddleware, // 2. Hesap aktif mi?
		middlewares.RequireUser(),    // 3. Normal kullanıcı mı?
	)

	// --- Panel Ana Sayfa ---
	panelGroup.Get("/home", panelHomeHandler.PanelHomeHandler) // GET /panel/home

	// --- Kullanıcının Kendi Davetiyeleri ---
	panelGroup.Get("/invitations", invitationHandler.ListInvitations)                 // GET /panel/invitations
	panelGroup.Get("/invitations/create", invitationHandler.ShowCreateInvitation)     // GET /panel/invitations/create
	panelGroup.Post("/invitations/create", invitationHandler.CreateInvitation)        // POST /panel/invitations/create
	panelGroup.Get("/invitations/update/:id", invitationHandler.ShowUpdateInvitation) // GET /panel/invitations/update/{id}
	panelGroup.Post("/invitations/update/:id", invitationHandler.UpdateInvitation)    // POST /panel/invitations/update/{id}
	panelGroup.Post("/invitations/delete/:id", invitationHandler.DeleteInvitation)    // POST /panel/invitations/delete/{id} (Formdan silme)
	panelGroup.Delete("/invitations/delete/:id", invitationHandler.DeleteInvitation)  // DELETE /panel/invitations/delete/{id} (JS/API için)

	// --- Kullanıcının Kendi Randevu Hizmetleri ---
	panelGroup.Get("/appointments", appointmentHandler.ListAppointments)                 // GET /panel/appointments
	panelGroup.Get("/appointments/create", appointmentHandler.ShowCreateAppointment)     // GET /panel/appointments/create
	panelGroup.Post("/appointments/create", appointmentHandler.CreateAppointment)        // POST /panel/appointments/create
	panelGroup.Get("/appointments/update/:id", appointmentHandler.ShowUpdateAppointment) // GET /panel/appointments/update/{id}
	panelGroup.Post("/appointments/update/:id", appointmentHandler.UpdateAppointment)    // POST /panel/appointments/update/{id}
	panelGroup.Post("/appointments/delete/:id", appointmentHandler.DeleteAppointment)    // POST /panel/appointments/delete/{id}
	panelGroup.Delete("/appointments/delete/:id", appointmentHandler.DeleteAppointment)  // DELETE /panel/appointments/delete/{id}
	// TODO: Randevu rezervasyonları (bookings) için rotalar

	// --- Kullanıcının Kendi Formları ---
	panelGroup.Get("/forms", formHandler.ListForms)                 // GET /panel/forms
	panelGroup.Get("/forms/create", formHandler.ShowCreateForm)     // GET /panel/forms/create
	panelGroup.Post("/forms/create", formHandler.CreateForm)        // POST /panel/forms/create
	panelGroup.Get("/forms/update/:id", formHandler.ShowUpdateForm) // GET /panel/forms/update/{id}
	panelGroup.Post("/forms/update/:id", formHandler.UpdateForm)    // POST /panel/forms/update/{id}
	panelGroup.Post("/forms/delete/:id", formHandler.DeleteForm)    // POST /panel/forms/delete/{id}
	panelGroup.Delete("/forms/delete/:id", formHandler.DeleteForm)  // DELETE /panel/forms/delete/{id}
	// TODO: Form gönderimleri (submissions) için rotalar

	// --- Kullanıcının Kendi Kartvizitleri ---
	panelGroup.Get("/cards", cardHandler.ListCards)                 // GET /panel/cards
	panelGroup.Get("/cards/create", cardHandler.ShowCreateCard)     // GET /panel/cards/create
	panelGroup.Post("/cards/create", cardHandler.CreateCard)        // POST /panel/cards/create
	panelGroup.Get("/cards/update/:id", cardHandler.ShowUpdateCard) // GET /panel/cards/update/{id}
	panelGroup.Post("/cards/update/:id", cardHandler.UpdateCard)    // POST /panel/cards/update/{id}
	panelGroup.Post("/cards/delete/:id", cardHandler.DeleteCard)    // POST /panel/cards/delete/{id}
	panelGroup.Delete("/cards/delete/:id", cardHandler.DeleteCard)  // DELETE /panel/cards/delete/{id}

	// --- Profil ---
	// /auth/profile rotası kullanılır. Panel menüsünden link verilir.
}
