package middleware

import (
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

func RBAC(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(c *gin.Context) {
		if !allowed[web.Role(c)] {
			c.Abort()
			web.Fail(c, web.Forbidden("current role cannot perform this operation"))
			return
		}
		c.Next()
	}
}
