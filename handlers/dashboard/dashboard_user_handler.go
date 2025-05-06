package handlers

import (
	"net/http"
	"strings"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/flashmessages"
	"davet.link/pkg/queryparams"
	"davet.link/pkg/renderer"
	"davet.link/services"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type UserHandler struct {
	userService services.IUserService
}

func NewUserHandler() *UserHandler {
	svc := services.NewUserService()
	return &UserHandler{userService: svc}
}

func (h *UserHandler) ListUsers(c *fiber.Ctx) error {
	var params queryparams.ListParams
	if err := c.QueryParser(&params); err != nil {
		configslog.Log.Warn("Kullanıcı listesi: Query parametreleri parse edilemedi, varsayılanlar kullanılıyor.", zap.Error(err))
		params = queryparams.DefaultListParams()
	}

	if params.Page <= 0 {
		params.Page = queryparams.DefaultPage
	}
	if params.PerPage <= 0 || params.PerPage > queryparams.MaxPerPage {
		params.PerPage = queryparams.DefaultPerPage
	}
	if params.SortBy == "" {
		params.SortBy = queryparams.DefaultSortBy
	}
	if params.OrderBy == "" {
		params.OrderBy = queryparams.DefaultOrderBy
	}

	paginatedResult, dbErr := h.userService.GetAllUsers(params)

	renderData := fiber.Map{
		"Title":  "Kullanıcılar",
		"Result": paginatedResult,
		"Params": params,
	}
	if dbErr != nil {
		configslog.Log.Error("Kullanıcı listesi DB Hatası", zap.Error(dbErr))
		renderData[renderer.FlashErrorKeyView] = "Kullanıcılar getirilirken bir hata oluştu."
		renderData["Result"] = &queryparams.PaginatedResult{
			Data: []models.User{},
			Meta: queryparams.PaginationMeta{
				CurrentPage: params.Page, PerPage: params.PerPage,
			},
		}
	}
	return renderer.Render(c, "dashboard/users/list", "layouts/dashboard", renderData, http.StatusOK)
}

func (h *UserHandler) ShowCreateUser(c *fiber.Ctx) error {
	return renderer.Render(c, "dashboard/users/create", "layouts/dashboard", fiber.Map{
		"Title": "Yeni Kullanıcı Ekle",
	})
}

func (h *UserHandler) CreateUser(c *fiber.Ctx) error {
	var req struct {
		Name     string `form:"name"`
		Account  string `form:"account"`
		Password string `form:"password"`
		Status   string `form:"status"`
		Type     string `form:"type"`
	}
	_ = c.BodyParser(&req)

	if req.Name == "" || req.Account == "" || req.Password == "" || req.Type == "" {
		return renderUserFormError("Yeni Kullanıcı Ekle", req, "Ad, Hesap Adı, Şifre ve Kullanıcı Tipi alanları zorunludur.", c)
	}

	status := req.Status == "true"
	user := &models.User{
		Name:     req.Name,
		Account:  req.Account,
		Password: req.Password,
		Status:   status,
		Type:     models.UserType(req.Type),
	}

	if user.Type != models.Dashboard && user.Type != models.Panel {
		return renderUserFormError("Yeni Kullanıcı Ekle", req, "Geçersiz kullanıcı tipi seçildi.", c)
	}

	if err := h.userService.CreateUser(c.UserContext(), user); err != nil {
		return renderUserFormError("Yeni Kullanıcı Ekle", req, "Kullanıcı oluşturulamadı: "+err.Error(), c)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kullanıcı başarıyla oluşturuldu.")
	return c.Redirect("/dashboard/users", fiber.StatusFound)
}

func (h *UserHandler) ShowUpdateUser(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	user, err := h.userService.GetUserByID(uint(id))
	if err != nil {
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, "Kullanıcı bulunamadı.")
		return c.Redirect("/dashboard/users", fiber.StatusSeeOther)
	}
	return renderer.Render(c, "dashboard/users/update", "layouts/dashboard", fiber.Map{
		"Title": "Kullanıcı Düzenle",
		"User":  user,
	})
}

func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	userID := uint(id)

	var req struct {
		Name     string `form:"name"`
		Account  string `form:"account"`
		Password string `form:"password"`
		Status   string `form:"status"`
		Type     string `form:"type"`
	}
	_ = c.BodyParser(&req)

	if req.Name == "" || req.Account == "" || req.Type == "" {
		user, _ := h.userService.GetUserByID(userID)
		return renderer.Render(c, "dashboard/users/update", "layouts/dashboard", fiber.Map{
			"Title":                    "Kullanıcı Düzenle",
			renderer.FlashErrorKeyView: "Zorunlu alanlar eksik.",
			renderer.FormDataKey:       req,
			"User":                     user,
		}, http.StatusBadRequest)
	}

	userData := &models.User{
		Name:    req.Name,
		Account: req.Account,
		Status:  req.Status == "true",
		Type:    models.UserType(req.Type),
	}
	if req.Password != "" {
		userData.Password = req.Password
	}

	if err := h.userService.UpdateUser(c.UserContext(), userID, userData); err != nil {
		user, _ := h.userService.GetUserByID(userID)
		return renderer.Render(c, "dashboard/users/update", "layouts/dashboard", fiber.Map{
			"Title":                    "Kullanıcı Düzenle",
			renderer.FlashErrorKeyView: "Güncelleme hatası: " + err.Error(),
			renderer.FormDataKey:       req,
			"User":                     user,
		}, http.StatusInternalServerError)
	}

	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kullanıcı başarıyla güncellendi.")
	return c.Redirect("/dashboard/users", fiber.StatusFound)
}

func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	id, _ := c.ParamsInt("id")
	userID := uint(id)

	if err := h.userService.DeleteUser(c.UserContext(), userID); err != nil {
		errMsg := "Kullanıcı silinemedi: " + err.Error()
		if strings.Contains(c.Get("Accept"), "application/json") {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": errMsg})
		}
		_ = flashmessages.SetFlashMessage(c, flashmessages.FlashErrorKey, errMsg)
		return c.Redirect("/dashboard/users", fiber.StatusSeeOther)
	}

	if strings.Contains(c.Get("Accept"), "application/json") {
		return c.JSON(fiber.Map{"message": "Kullanıcı başarıyla silindi."})
	}
	_ = flashmessages.SetFlashMessage(c, flashmessages.FlashSuccessKey, "Kullanıcı başarıyla silindi.")
	return c.Redirect("/dashboard/users", fiber.StatusFound)
}

func renderUserFormError(title string, req any, message string, c *fiber.Ctx) error {
	return renderer.Render(c, "dashboard/users/create", "layouts/dashboard", fiber.Map{
		"Title":                    title,
		renderer.FlashErrorKeyView: message,
		renderer.FormDataKey:       req,
	}, http.StatusBadRequest)
}
