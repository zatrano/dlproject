package repositories

import (
	"davet.link/configs/configsdatabase"
	"davet.link/models"

	"gorm.io/gorm"
)

type IAuthRepository interface {
	FindUserByAccount(account string) (*models.User, error)
	FindUserByID(id uint) (*models.User, error)
	UpdateUser(user *models.User) error
}

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository() IAuthRepository {
	return &AuthRepository{db: configsdatabase.GetDB()}
}

func (r *AuthRepository) FindUserByAccount(account string) (*models.User, error) {
	var user models.User
	err := r.db.Where("account = ?", account).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByID(id uint) (*models.User, error) {
	var user models.User
	err := r.db.First(&user, id).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) UpdateUser(user *models.User) error {
	return r.db.Save(user).Error
}

var _ IAuthRepository = (*AuthRepository)(nil)
