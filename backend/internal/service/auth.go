package service

import (
	"errors"
	"strings"
	"time"

	"oscraper/config"
	"oscraper/internal/model"
	"oscraper/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	bootstrapUsername = "admin"
	bootstrapPassword = "admin"
)

type AuthService struct {
	repo       *repository.UserRepository
	jwtSecret  []byte
	tokenHours int
}

type LoginCommand struct {
	Username string
	Password string
}

type SetupAdminCommand struct {
	Username string
	Password string
}

type ChangePasswordCommand struct {
	OldPassword string
	NewPassword string
}

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

type UserResponse struct {
	ID                 uint      `json:"id"`
	Username           string    `json:"username"`
	IsAdmin            bool      `json:"is_admin"`
	RequiresAdminSetup bool      `json:"requires_admin_setup"`
	CreatedAt          time.Time `json:"created_at"`
}

func NewAuthService(cfg *config.Config, db *gorm.DB) *AuthService {
	return &AuthService{repo: repository.NewUserRepository(db), jwtSecret: []byte(cfg.JWTSecret), tokenHours: cfg.AccessTokenHours}
}

func (s *AuthService) InitBootstrapAdmin() error {
	count, err := s.repo.CountAdmins()
	if err != nil || count > 0 {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(bootstrapPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.repo.Create(&model.User{
		Username: bootstrapUsername, PasswordHash: string(hash), IsAdmin: true, RequiresAdminSetup: true,
	})
}

func (s *AuthService) Login(request LoginCommand) (*TokenResponse, error) {
	user, err := s.repo.FindByUsername(strings.TrimSpace(request.Username))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.Password)) != nil {
		return nil, Unauthorized("auth.invalid_credentials", "Invalid username or password")
	}
	return s.issueToken(user)
}

func (s *AuthService) SetupAdmin(userID uint, request SetupAdminCommand) (*TokenResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, NotFound("auth.user_not_found", "User not found")
	}
	if !user.IsAdmin || !user.RequiresAdminSetup {
		return nil, Conflict("auth.setup_complete", "Administrator setup is already complete")
	}
	username := strings.TrimSpace(request.Username)
	if username != user.Username {
		if _, err := s.repo.FindByUsername(username); err == nil {
			return nil, Conflict("auth.username_exists", "Username already exists")
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, Internal("auth.lookup_failed", "Failed to validate username", err)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, Internal("auth.password_hash_failed", "Failed to secure password", err)
	}
	user.Username = username
	user.PasswordHash = string(hash)
	user.RequiresAdminSetup = false
	user.TokenVersion++
	if err := s.repo.Update(user); err != nil {
		return nil, Internal("auth.setup_failed", "Failed to complete administrator setup", err)
	}
	return s.issueToken(user)
}

func (s *AuthService) Profile(userID uint) (*UserResponse, error) {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, NotFound("auth.user_not_found", "User not found")
	}
	return toUserResponse(user), nil
}

func (s *AuthService) ChangePassword(userID uint, request ChangePasswordCommand) error {
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return NotFound("auth.user_not_found", "User not found")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(request.OldPassword)) != nil {
		return BadRequest("auth.old_password_invalid", "Current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(request.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return Internal("auth.password_hash_failed", "Failed to secure password", err)
	}
	user.PasswordHash = string(hash)
	user.TokenVersion++
	if err := s.repo.Update(user); err != nil {
		return Internal("auth.password_change_failed", "Failed to change password", err)
	}
	return nil
}

func (s *AuthService) Logout(userID uint) error {
	if err := s.repo.IncrementTokenVersion(userID); err != nil {
		return Internal("auth.logout_failed", "Failed to log out", err)
	}
	return nil
}

func (s *AuthService) issueToken(user *model.User) (*TokenResponse, error) {
	expiresAt := time.Now().Add(time.Duration(s.tokenHours) * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID, "username": user.Username, "is_admin": user.IsAdmin,
		"token_version": user.TokenVersion, "exp": expiresAt.Unix(),
	})
	encoded, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return nil, Internal("auth.token_failed", "Failed to create access token", err)
	}
	return &TokenResponse{Token: encoded, ExpiresAt: expiresAt.Unix()}, nil
}

func toUserResponse(user *model.User) *UserResponse {
	return &UserResponse{ID: user.ID, Username: user.Username, IsAdmin: user.IsAdmin, RequiresAdminSetup: user.RequiresAdminSetup, CreatedAt: user.CreatedAt}
}
