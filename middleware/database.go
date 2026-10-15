package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yongwei9527-art/s-ui-go/database"
)

// DatabaseAccess guards the whole API/subscription operation, including token
// checks and transactions. Only the authenticated import endpoint obtains its
// own exclusive lease; never hold a read lease while asking for recovery.
func DatabaseAccess(allowImport bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if allowImport && c.Request.Method == http.MethodPost && c.Param("postAction") == "importdb" {
			if !database.RecoveryBlocked() {
				c.Next()
				return
			}
		} else if release, ok := database.TryAcquireAccess(); ok {
			defer release()
			c.Next()
			return
		}
		c.Header("Retry-After", "3")
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"success": false, "msg": database.ErrRecoveryInProgress.Error(), "obj": nil,
		})
	}
}
