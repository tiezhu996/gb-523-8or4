package router

import (
	"datacenter-thermal-capacity-planner/backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterRackRoutes(api *gin.RouterGroup, h *handler.RackHandler, write gin.HandlerFunc) {
	racks := api.Group("/racks")
	racks.GET("", h.List)
	racks.GET("/:id", h.Get)
	racks.POST("", write, h.Create)
	racks.PUT("/:id", write, h.Update)
}
