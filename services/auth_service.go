package services

import (
	"davet.link/configs/configslog"
	"davet.link/models"
	"davet.link/repositories"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type ServiceError string

func (e ServiceError) Error() string {
	return string(e)
}

const (
	ErrInvalidCredentials       ServiceError = "geçersiz kimlik bilgileri"
	ErrUserNotFound             ServiceError = "kullanıcı bulunamadı"
	ErrUserInactive             ServiceError = "kullanıcı aktif değil"
	ErrCurrentPasswordIncorrect ServiceError = "mevcut şifre hatalı"
	ErrPasswordTooShort         ServiceError = "yeni şifre en az 6 karakter olmalıdır"
	ErrPasswordSameAsOld        ServiceError = "yeni şifre mevcut şifre ile aynı olamaz"
	ErrAuthGeneric              ServiceError = "kimlik doğrulaması sırasında bir hata oluştu"
	ErrProfileGeneric           ServiceError = "profil bilgileri alınırken hata"
	ErrUpdatePasswordGeneric    ServiceError = "şifre güncellenirken bir hata oluştu"
	ErrHashingFailed            ServiceError = "yeni şifre oluşturulurken hata"
	ErrDatabaseUpdateFailed     ServiceError = "veritabanı güncellemesi başarısız oldu"
)

type IAuthService interface {
	Authenticate(account, password string) (*models.User, error)
	GetUserProfile(id uint) (*models.User, error)
	UpdatePassword(userID uint, currentPass, newPassword string) error
}

type AuthService struct {
	repo repositories.IAuthRepository
}

func NewAuthService() IAuthService {
	return &AuthService{repo: repositories.NewAuthRepository()}
}

func (s *AuthService) Authenticate(account, password string) (*models.User, error) {
	user, err := s.repo.FindUserByAccount(account)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			configslog.Log.Warn("Kimlik doğrulama başarısız: Kullanıcı bulunamadı", zap.String("account", account))
			return nil, ErrInvalidCredentials
		}
		configslog.Log.Error("Kimlik doğrulama hatası (DB)",
			zap.String("account", account),
			zap.Error(err),
		)
		return nil, ErrAuthGeneric
	}

	if !user.Status {
		configslog.Log.Warn("Kimlik doğrulama başarısız: Kullanıcı aktif değil",
			zap.String("account", account),
			zap.Uint("user_id", user.ID),
		)
		return nil, ErrUserInactive
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	if err != nil {
		configslog.Log.Warn("Kimlik doğrulama başarısız: Geçersiz parola",
			zap.String("account", account),
			zap.Uint("user_id", user.ID),
		)
		return nil, ErrInvalidCredentials
	}

	configslog.Log.Info("Kimlik doğrulama başarılı",
		zap.String("account", account),
		zap.Uint("user_id", user.ID),
	)
	return user, nil
}

func (s *AuthService) GetUserProfile(id uint) (*models.User, error) {
	user, err := s.repo.FindUserByID(id)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			configslog.Log.Warn("Profil alınamadı: Kullanıcı bulunamadı", zap.Uint("user_id", id))
			return nil, ErrUserNotFound
		}
		configslog.Log.Error("Profil alma hatası (DB)",
			zap.Uint("user_id", id),
			zap.Error(err),
		)
		return nil, ErrProfileGeneric
	}
	return user, nil
}

func (s *AuthService) UpdatePassword(userID uint, currentPass, newPassword string) error {
	user, err := s.repo.FindUserByID(userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			configslog.Log.Warn("Parola güncelleme başarısız: Kullanıcı bulunamadı", zap.Uint("user_id", userID))
			return ErrUserNotFound
		}
		configslog.Log.Error("Parola güncelleme hatası: Kullanıcı bulunurken DB hatası",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return ErrUpdatePasswordGeneric
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(currentPass)); err != nil {
		configslog.Log.Warn("Parola güncelleme başarısız: Mevcut parola hatalı", zap.Uint("user_id", userID))
		return ErrCurrentPasswordIncorrect
	}

	if len(newPassword) < 6 {
		configslog.Log.Warn("Parola güncelleme başarısız: Yeni parola çok kısa", zap.Uint("user_id", userID))
		return ErrPasswordTooShort
	}
	if currentPass == newPassword {
		configslog.Log.Warn("Parola güncelleme başarısız: Yeni parola eskiyle aynı", zap.Uint("user_id", userID))
		return ErrPasswordSameAsOld
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		configslog.Log.Error("Parola güncelleme hatası: Yeni parola hashlenemedi",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return ErrHashingFailed
	}

	user.Password = string(hashedPassword)
	if err := s.repo.UpdateUser(user); err != nil {
		configslog.Log.Error("Parola güncelleme hatası: Kullanıcı güncellenirken DB hatası",
			zap.Uint("user_id", userID),
			zap.Error(err),
		)
		return ErrDatabaseUpdateFailed
	}

	configslog.Log.Info("Parola başarıyla güncellendi", zap.Uint("user_id", userID))
	return nil
}

var _ IAuthService = (*AuthService)(nil)
