package middleware

import (
	"fmt"
	"log/slog"
	"runtime/debug"

	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

func Recovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("request panic recovered", "request_id", web.RequestID(c), "error", fmt.Sprint(recovered), "stack", string(debug.Stack()))
				c.Abort()
				web.Fail(c, web.Internal(fmt.Errorf("panic recovered: %v", recovered)))
			}
		}()
		c.Next()
	}
}
