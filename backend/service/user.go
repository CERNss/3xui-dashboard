package service

import (
	"errors"

	"github.com/cern/3xui-dashboard/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserService provides user management operations.
type UserService struct {
	db *gorm.DB
}

// NewUserService constructs a UserService with the given database.
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db}
}

// Register creates a new user account.
func (s *UserService) Register(username, email, password, role string) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	result := s.db.Create(user)
	if result.Error != nil {
		return nil, result.Error
	}
	return user, nil
}

// Login checks credentials and returns the user if valid.
func (s *UserService) Login(username, password string) (*model.User, error) {
	var user model.User
	if err := s.db.Where("username = ?", username).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return &user, nil
}

// GetByID returns a user by primary key.
func (s *UserService) GetByID(id uint) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers returns all dashboard users.
func (s *UserService) ListUsers() ([]model.User, error) {
	var users []model.User
	if err := s.db.Find(&users).Error; err != nil {
		return nil, err
	}
	return users, nil
}

// UpdateUser updates mutable fields on a user.
func (s *UserService) UpdateUser(id uint, username, email, role, xuiEmail, xuiSubID string) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	user.Username = username
	user.Email = email
	user.Role = role
	user.XUIClientEmail = xuiEmail
	user.XUISubID = xuiSubID
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// DeleteUser removes a user by ID.
func (s *UserService) DeleteUser(id uint) error {
	return s.db.Delete(&model.User{}, id).Error
}

// ChangePassword verifies the old password and sets a new one.
func (s *UserService) ChangePassword(id uint, oldPassword, newPassword string) error {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return errors.New("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.Model(&user).Update("password_hash", string(hash)).Error
}

// UpdateProfile updates non-sensitive profile fields.
func (s *UserService) UpdateProfile(id uint, email string) (*model.User, error) {
	var user model.User
	if err := s.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	user.Email = email
	if err := s.db.Save(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// CountAdmins returns the number of admin users (used on first-run seeding).
func (s *UserService) CountAdmins() (int64, error) {
	var count int64
	err := s.db.Model(&model.User{}).Where("role = ?", model.RoleAdmin).Count(&count).Error
	return count, err
}
