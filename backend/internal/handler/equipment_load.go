package handler

import (
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/service"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

type EquipmentLoadHandler struct{ service *service.EquipmentLoadService }

func NewEquipmentLoadHandler(service *service.EquipmentLoadService) *EquipmentLoadHandler {
	return &EquipmentLoadHandler{service: service}
}

func (h *EquipmentLoadHandler) List(c *gin.Context) {
	page, size := web.QueryPage(c)
	items, total, err := h.service.List(c.Request.Context(), web.SearchTerm(c), c.Query("status"), page, size)
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, web.Page{Items: items, Total: total, Page: page, Size: size})
}

func (h *EquipmentLoadHandler) Get(c *gin.Context) {
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

func (h *EquipmentLoadHandler) Create(c *gin.Context) {
	var req dto.CreateEquipmentLoadRequest
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

func (h *EquipmentLoadHandler) Update(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	var req dto.UpdateEquipmentLoadRequest
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

func (h *EquipmentLoadHandler) Validate(c *gin.Context) {
	result, err := h.service.Validate(c.Request.Context())
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, result)
}
