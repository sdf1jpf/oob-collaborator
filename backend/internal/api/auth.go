package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/oob-collaborator/backend/internal/config"
	"github.com/oob-collaborator/backend/internal/ratelimit"
	"github.com/oob-collaborator/backend/internal/security"
	"golang.org/x/crypto/bcrypt"
)

const jwtLifetime = 8 * time.Hour

type AuthHandler struct {
	cfg      *config.Config
	lockout  *ratelimit.LockoutLimiter
}

func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{
		cfg: cfg,
		lockout: ratelimit.NewLockoutLimiter(5, 15*time.Minute, 15*time.Minute),
	}
}

type loginRequest struct {
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	ip := clientIP(c)
	if h.lockout.IsLocked(ip) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many failed attempts; try again later"})
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if !security.ConstantTimeEqual(req.Password, h.cfg.AdminPassword) {
		h.lockout.RecordFailure(ip)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}
	h.lockout.RecordSuccess(ip)

	signed, err := signToken(h.cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "token generation failed"})
		return
	}

	setSessionCookie(c, signed, h.cfg)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	clearSessionCookie(c, h.cfg)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AuthHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"authenticated": true})
}

func signToken(cfg *config.Config) (string, error) {
	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "admin",
		"iss": security.JWTIssuer,
		"aud": security.JWTAudience,
		"exp": now.Add(jwtLifetime).Unix(),
		"iat": now.Unix(),
	})
	return token.SignedString([]byte(cfg.JWTSecret))
}

func ValidateToken(tokenStr string, cfg *config.Config) error {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(cfg.JWTSecret), nil
	}, jwt.WithIssuer(security.JWTIssuer), jwt.WithAudience(security.JWTAudience))
	if err != nil || !token.Valid {
		return jwt.ErrTokenInvalidClaims
	}
	return nil
}

func JWTAuth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, ok := sessionTokenFromRequest(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}
		if err := ValidateToken(tokenStr, cfg); err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		c.Next()
	}
}

func sessionTokenFromRequest(c *gin.Context) (string, bool) {
	if cookie, err := c.Cookie(security.SessionCookieName); err == nil && cookie != "" {
		return cookie, true
	}
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[7:]), true
	}
	return "", false
}

func setSessionCookie(c *gin.Context, token string, cfg *config.Config) {
	secure := cfg.Domain != "localhost" && strings.ToLower(cfg.Domain) != "127.0.0.1"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(security.SessionCookieName, token, int(jwtLifetime.Seconds()), "/", "", secure, true)
}

func clearSessionCookie(c *gin.Context, cfg *config.Config) {
	secure := cfg.Domain != "localhost" && strings.ToLower(cfg.Domain) != "127.0.0.1"
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(security.SessionCookieName, "", -1, "/", "", secure, true)
}

func clientIP(c *gin.Context) string {
	if ip := c.GetHeader("X-Real-IP"); ip != "" {
		return ip
	}
	if ip := c.GetHeader("X-Forwarded-For"); ip != "" {
		if i := strings.Index(ip, ","); i >= 0 {
			return strings.TrimSpace(ip[:i])
		}
		return ip
	}
	return c.ClientIP()
}

// HashPassword is a utility for generating bcrypt hashes (not used at runtime with plain env password).
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}
