package httpx

import "github.com/gin-gonic/gin"

// Error is the single error shape returned by the API.
func Error(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
