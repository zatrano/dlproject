package handlers

import (
	"net/http"
	"davet.link/pkg/renderer"

	"github.com/gofiber/fiber/v2"
)

func PanelHomeHandler(c *fiber.Ctx) error {
	mapData := fiber.Map{
		"Title": "Aracı Ana Sayfa",
	}

	return renderer.Render(c, "panel/home/home", "layouts/panel", mapData, http.StatusOK)
}
