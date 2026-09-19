package handler

import (
	"strconv"

	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/service"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

type RackHandler struct{ service *service.RackService }

func NewRackHandler(service *service.RackService) *RackHandler { return &RackHandler{service: service} }

func (h *RackHandler) List(c *gin.Context) {
	page, size := web.QueryPage(c)
	zoneValue, _ := strconv.ParseUint(c.Query("zone_id"), 10, 64)
	items, total, err := h.service.List(c.Request.Context(), web.SearchTerm(c), c.Query("status"), uint(zoneValue), page, size)
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, web.Page{Items: items, Total: total, Page: page, Size: size})
}

func (h *RackHandler) Get(c *gin.Context) {
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

func (h *RackHandler) Create(c *gin.Context) {
	var req dto.CreateRackRequest
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

func (h *RackHandler) Update(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	var req dto.UpdateRackRequest
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
