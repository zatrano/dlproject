package renderer

import (
	"net/http"
	"davet.link/pkg/flashmessages"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/log"
	"go.uber.org/zap"
)

const (
	CsrfTokenKey        = "CsrfToken"
	FlashSuccessKeyView = "Success"
	FlashErrorKeyView   = "Error"
	FormDataKey         = "FormData"
)

func prepareRenderData(c *fiber.Ctx, data fiber.Map) fiber.Map {
	renderData := make(fiber.Map)

	renderData[CsrfTokenKey] = c.Locals("csrf")

	flashData, flashErr := flashmessages.GetFlashMessages(c)
	if flashErr != nil {
		log.Warn("Render helper: Flash mesajları alınamadı", zap.Error(flashErr))
	}
	renderData[FlashSuccessKeyView] = flashData.Success

	var handlerError string
	if data == nil {
		data = fiber.Map{}
	}

	if errVal, ok := data[FlashErrorKeyView]; ok {
		if errStr, okStr := errVal.(string); okStr {
			handlerError = errStr
		}
	}

	for key, value := range data {
		renderData[key] = value
	}

	combinedError := flashData.Error
	if handlerError != "" {
		if combinedError != "" {
			combinedError += " | " + handlerError
		} else {
			combinedError = handlerError
		}
	}

	if combinedError != "" {
		renderData[FlashErrorKeyView] = combinedError
	} else {
		if _, exists := renderData[FlashErrorKeyView]; exists {
			delete(renderData, FlashErrorKeyView)
		}
	}

	return renderData
}

func Render(c *fiber.Ctx, template string, layout string, data fiber.Map, statusCode ...int) error {
	status := http.StatusOK
	if len(statusCode) > 0 {
		status = statusCode[0]
	}

	finalData := prepareRenderData(c, data)

	if layout == "" {
		log.Debug("Rendering template without layout",
			zap.String("template", template),
			zap.Int("status", status),
		)
		return c.Status(status).Render(template, finalData)
	} else {
		log.Debug("Rendering template with layout",
			zap.String("template", template),
			zap.String("layout", layout),
			zap.Int("status", status),
		)
		return c.Status(status).Render(template, finalData, layout)
	}
}
