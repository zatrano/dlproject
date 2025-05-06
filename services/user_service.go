package services

import (
	"context"
	"errors"
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/pkg/queryparams"
	"davet.link/repositories"

	"go.uber.org/zap"
)

const contextUserIDKey = "user_id"

type IUserService interface {
	GetAllUsers(params queryparams.ListParams) (*queryparams.PaginatedResult, error)
	GetUserByID(id uint) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	UpdateUser(ctx context.Context, id uint, userData *models.User) error
	DeleteUser(ctx context.Context, id uint) error
	GetUserCount() (int64, error)
}

type UserService struct {
	repo repositories.IUserRepository
}

func NewUserService() IUserService {
	return &UserService{repo: repositories.NewUserRepository()}
}

func (s *UserService) GetAllUsers(params queryparams.ListParams) (*queryparams.PaginatedResult, error) {
	users, totalCount, err := s.repo.GetAllUsers(params)
	if err != nil {
		configslog.Log.Error("Kullanıcılar alınamadı", zap.Error(err))
		return nil, errors.New("kullanıcılar getirilirken bir hata oluştu")
	}

	result := &queryparams.PaginatedResult{
		Data: users,
		Meta: queryparams.PaginationMeta{
			CurrentPage: params.Page,
			PerPage:     params.PerPage,
			TotalItems:  totalCount,
			TotalPages:  queryparams.CalculateTotalPages(totalCount, params.PerPage),
		},
	}
	return result, nil
}

func (s *UserService) GetUserByID(id uint) (*models.User, error) {
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		configslog.Log.Warn("Kullanıcı bulunamadı", zap.Uint("user_id", id), zap.Error(err))
		return nil, errors.New("kullanıcı bulunamadı")
	}
	return user, nil
}

func (s *UserService) CreateUser(ctx context.Context, user *models.User) error {
	if user.Password == "" {
		return errors.New("şifre alanı boş olamaz")
	}
	if err := user.SetPassword(user.Password); err != nil {
		configslog.Log.Error("Şifre oluşturulamadı", zap.Error(err))
		return errors.New("şifre oluşturulurken hata oluştu")
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *UserService) UpdateUser(ctx context.Context, id uint, userData *models.User) error {
	currentUserID, ok := ctx.Value(contextUserIDKey).(uint)
	if !ok || currentUserID == 0 {
		return errors.New("güncelleyen kullanıcı kimliği geçersiz")
	}

	_, err := s.repo.GetUserByID(id)
	if err != nil {
		return errors.New("kullanıcı bulunamadı")
	}

	updateData := map[string]interface{}{
		"name":    userData.Name,
		"account": userData.Account,
		"status":  userData.Status,
		"type":    userData.Type,
	}

	if userData.Password != "" {
		hashed := models.User{}
		if err := hashed.SetPassword(userData.Password); err != nil {
			return errors.New("şifre oluşturulurken hata oluştu")
		}
		updateData["password"] = hashed.Password
	}

	return s.repo.UpdateUser(ctx, id, updateData, currentUserID)
}

func (s *UserService) DeleteUser(ctx context.Context, id uint) error {
	return s.repo.DeleteUser(ctx, id)
}

func (s *UserService) GetUserCount() (int64, error) {
	return s.repo.GetUserCount()
}

var _ IUserService = (*UserService)(nil)
