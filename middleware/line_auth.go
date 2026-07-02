package middleware

import (
	"finance-chat/agent"
	"finance-chat/config"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// UserIDContextKey is the gin context key the verified LINE userId is stored
// under after RequireLineAuth runs.
const UserIDContextKey = "userID"

// truncateForLog keeps a header value loggable without ever printing a full
// bearer token — short values pass through, long ones show just enough of
// each end to recognize them across log lines.
func truncateForLog(s string) string {
	if s == "" {
		return "<empty>"
	}
	if len(s) <= 24 {
		return s
	}
	return s[:14] + "..." + s[len(s)-6:]
}

// RequireLineAuth verifies the LIFF ID token sent as "Authorization: Bearer <token>"
// and stores the resulting LINE userId in the request context. Requests without
// a valid token are rejected with 401 before reaching the handler.
//
// This is the single place auth-related request logging lives — controllers
// don't need their own logging, since every /api request passes through here.
func RequireLineAuth(lineSvc agent.LineService, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.AuthDevBypass {
			userID := strings.TrimSpace(c.Query("user_id"))
			if userID == "" {
				userID = cfg.AuthDevUserID
			}
			log.Printf("[auth] dev bypass %s %s — userID=%s", c.Request.Method, c.Request.URL.String(), userID)
			c.Set(UserIDContextKey, userID)
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		reqLine := c.Request.Method + " " + c.Request.URL.String()

		token, ok := strings.CutPrefix(authHeader, "Bearer ")
		if !ok || strings.TrimSpace(token) == "" {
			log.Printf("[auth] 401 missing/malformed Authorization header on %s — Authorization=%s Origin=%q UA=%q",
				reqLine, truncateForLog(authHeader), c.GetHeader("Origin"), c.Request.UserAgent())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing or malformed Authorization header"})
			return
		}

		userID, err := lineSvc.VerifyIDToken(token)
		if err != nil {
			log.Printf("[auth] 401 invalid ID token on %s — token=%s err=%v Origin=%q",
				reqLine, truncateForLog(token), err, c.GetHeader("Origin"))
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid LINE ID token: " + err.Error()})
			return
		}

		log.Printf("[auth] ok %s — userID=%s", reqLine, userID)
		c.Set(UserIDContextKey, userID)
		c.Next()
	}
}
