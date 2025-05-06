package repositories

import (
	"context"

	"davet.link/configs/configsdatabase"
	"davet.link/models"
	"davet.link/pkg/queryparams"
)

type IUserRepository interface {
	GetAllUsers(params queryparams.ListParams) ([]models.User, int64, error)
	GetUserByID(id uint) (*models.User, error)
	CreateUser(ctx context.Context, user *models.User) error
	BulkCreateUsers(ctx context.Context, users []models.User) error
	UpdateUser(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error
	BulkUpdateUsers(ctx context.Context, condition map[string]interface{}, data map[string]interface{}, updatedBy uint) error
	DeleteUser(ctx context.Context, id uint) error
	BulkDeleteUsers(ctx context.Context, condition map[string]interface{}) error
	GetUserCount() (int64, error)
}

type UserRepository struct {
	base IBaseRepository[models.User]
}

func NewUserRepository() IUserRepository {
	base := NewBaseRepository[models.User](configsdatabase.GetDB())
	base.SetAllowedSortColumns([]string{"id", "name", "account", "created_at", "status", "type"})

	return &UserRepository{base: base}
}

func (r *UserRepository) GetAllUsers(params queryparams.ListParams) ([]models.User, int64, error) {
	return r.base.GetAll(params)
}

func (r *UserRepository) GetUserByID(id uint) (*models.User, error) {
	return r.base.GetByID(id)
}

func (r *UserRepository) CreateUser(ctx context.Context, user *models.User) error {
	return r.base.Create(ctx, user)
}

func (r *UserRepository) BulkCreateUsers(ctx context.Context, users []models.User) error {
	return r.base.BulkCreate(ctx, users)
}

func (r *UserRepository) UpdateUser(ctx context.Context, id uint, data map[string]interface{}, updatedBy uint) error {
	return r.base.Update(ctx, id, data, updatedBy)
}

func (r *UserRepository) BulkUpdateUsers(ctx context.Context, condition map[string]interface{}, data map[string]interface{}, updatedBy uint) error {
	return r.base.BulkUpdate(ctx, condition, data, updatedBy)
}

func (r *UserRepository) DeleteUser(ctx context.Context, id uint) error {
	return r.base.Delete(ctx, id)
}

func (r *UserRepository) BulkDeleteUsers(ctx context.Context, condition map[string]interface{}) error {
	return r.base.BulkDelete(ctx, condition)
}

func (r *UserRepository) GetUserCount() (int64, error) {
	return r.base.GetCount()
}

var _ IUserRepository = (*UserRepository)(nil)
