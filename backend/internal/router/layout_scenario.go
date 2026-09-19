package router

import (
	"datacenter-thermal-capacity-planner/backend/internal/handler"
	"github.com/gin-gonic/gin"
)

func RegisterLayoutScenarioRoutes(api *gin.RouterGroup, h *handler.LayoutScenarioHandler, plan, review gin.HandlerFunc) {
	scenarios := api.Group("/scenarios")
	scenarios.GET("", h.List)
	scenarios.GET("/:id", h.Get)
	scenarios.GET("/:id/compare", h.Compare)
	scenarios.POST("", plan, h.Create)
	scenarios.POST("/:id/evaluate", plan, h.Evaluate)
	scenarios.POST("/:id/transition", review, h.Transition)
}
