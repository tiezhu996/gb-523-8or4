package handler

import (
	"datacenter-thermal-capacity-planner/backend/internal/audit"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/service"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

type ThermalZoneHandler struct{ service *service.ThermalZoneService }

func NewThermalZoneHandler(service *service.ThermalZoneService) *ThermalZoneHandler {
	return &ThermalZoneHandler{service: service}
}

func (h *ThermalZoneHandler) List(c *gin.Context) {
	page, size := web.QueryPage(c)
	items, total, err := h.service.List(c.Request.Context(), web.SearchTerm(c), c.Query("status"), page, size)
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, web.Page{Items: items, Total: total, Page: page, Size: size})
}

func (h *ThermalZoneHandler) Get(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, item)
}

func (h *ThermalZoneHandler) Create(c *gin.Context) {
	var req dto.CreateThermalZoneRequest
	if !web.BindJSON(c, &req) {
		return
	}
	item, err := h.service.Create(c.Request.Context(), req, auditFrom(c))
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.Created(c, item)
}

func (h *ThermalZoneHandler) Update(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	var req dto.UpdateThermalZoneRequest
	if !web.BindJSON(c, &req) {
		return
	}
	item, err := h.service.Update(c.Request.Context(), id, req, auditFrom(c))
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, item)
}

func auditFrom(c *gin.Context) audit.Entry {
	return audit.Entry{
		RequestID: web.RequestID(c), ActorID: web.UserID(c), ActorUsername: web.Username(c),
	}
}
