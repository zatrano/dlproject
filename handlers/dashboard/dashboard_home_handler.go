package handlers

import (
	"net/http"
	"davet.link/configs/configslog"
	"davet.link/pkg/renderer"
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type DashboardHomeHandler struct {
	userService services.IUserService
}

func NewDashboardHomeHandler() *DashboardHomeHandler {
	svc := services.NewUserService()
	return &DashboardHomeHandler{userService: svc}
}

func (h *DashboardHomeHandler) HomePage(c *fiber.Ctx) error {
	userCount, userErr := h.userService.GetUserCount()
	if userErr != nil {
		configslog.Log.Error("Anasayfa: Kullanıcı sayısı alınamadı", zap.Error(userErr))
		userCount = 0
	}

	mapData := fiber.Map{
		"Title":     "Dashboard",
		"UserCount": userCount,
	}
	return renderer.Render(c, "dashboard/home/home", "layouts/dashboard", mapData, http.StatusOK)
}
