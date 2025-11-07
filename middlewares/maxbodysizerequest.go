package middlewares

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware để giới hạn kích thước request body
func MaxBodySizeMiddleware(maxRequestSize int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Nếu request vượt quá giới hạn, việc đọc nó (như c.MultipartForm()) sẽ báo lỗi
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, int64(maxRequestSize))

		// Rất quan trọng: Thêm header này để Gin biết giới hạn
		c.Request.Header.Set("Content-Length", c.Request.Header.Get("Content-Length"))

		c.Next()
	}
}
