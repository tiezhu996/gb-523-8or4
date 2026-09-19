package router

import (
	"datacenter-thermal-capacity-planner/backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterEquipmentLoadRoutes(api *gin.RouterGroup, h *handler.EquipmentLoadHandler, write gin.HandlerFunc) {
	loads := api.Group("/loads")
	loads.GET("", h.List)
	loads.GET("/:id", h.Get)
	loads.POST("", write, h.Create)
	loads.PUT("/:id", write, h.Update)
	loads.POST("/validate", write, h.Validate)
}
