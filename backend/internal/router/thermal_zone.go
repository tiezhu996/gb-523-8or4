package router

import (
	"datacenter-thermal-capacity-planner/backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterThermalZoneRoutes(api *gin.RouterGroup, h *handler.ThermalZoneHandler, write gin.HandlerFunc) {
	zones := api.Group("/zones")
	zones.GET("", h.List)
	zones.GET("/:id", h.Get)
	zones.POST("", write, h.Create)
	zones.PUT("/:id", write, h.Update)
}
