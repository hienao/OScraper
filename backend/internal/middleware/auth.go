package middleware

import (
	"errors"
	"net/http"
	"strings"

	"oscraper/config"
	"oscraper/internal/repository"
	"oscraper/pkg/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

func JWTAuth(cfg *config.Config, db *gorm.DB) gin.HandlerFunc {
	repo := repository.NewUserRepository(db)
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
			response.Unauthorized(c, "auth.token_missing", "A valid Bearer token is required")
			c.Abort()
			return
		}
		token, err := jwt.Parse(strings.TrimSpace(parts[1]), func(token *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWTSecret), nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || !token.Valid {
			response.Unauthorized(c, "auth.token_invalid", "Access token is invalid or expired")
			c.Abort()
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			response.Unauthorized(c, "auth.token_invalid", "Access token is invalid")
			c.Abort()
			return
		}
		userIDValue, userOK := claims["user_id"].(float64)
		versionValue, versionOK := claims["token_version"].(float64)
		if !userOK || !versionOK || userIDValue < 1 {
			response.Unauthorized(c, "auth.token_invalid", "Access token has invalid claims")
			c.Abort()
			return
		}
		user, err := repo.FindByID(uint(userIDValue))
		if errors.Is(err, gorm.ErrRecordNotFound) || err != nil || user.TokenVersion != int(versionValue) {
			response.Unauthorized(c, "auth.token_revoked", "Access token has been revoked")
			c.Abort()
			return
		}
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("is_admin", user.IsAdmin)
		c.Set("requires_admin_setup", user.RequiresAdminSetup)
		c.Next()
	}
}

func AdminSetupComplete() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetBool("requires_admin_setup") {
			response.Forbidden(c, "auth.setup_required", "Complete administrator setup first")
			c.Abort()
			return
		}
		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !c.GetBool("is_admin") {
			response.Error(c, http.StatusForbidden, "auth.admin_required", "Administrator permission is required")
			c.Abort()
			return
		}
		c.Next()
	}
}
