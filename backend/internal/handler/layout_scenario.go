package handler

import (
	"strconv"

	"datacenter-thermal-capacity-planner/backend/internal/auth"
	"datacenter-thermal-capacity-planner/backend/internal/constants"
	"datacenter-thermal-capacity-planner/backend/internal/dto"
	"datacenter-thermal-capacity-planner/backend/internal/service"
	"datacenter-thermal-capacity-planner/backend/internal/web"
	"github.com/gin-gonic/gin"
)

type LayoutScenarioHandler struct {
	service *service.LayoutScenarioService
}

func NewLayoutScenarioHandler(service *service.LayoutScenarioService) *LayoutScenarioHandler {
	return &LayoutScenarioHandler{service: service}
}

func (h *LayoutScenarioHandler) List(c *gin.Context) {
	page, size := web.QueryPage(c)
	items, total, err := h.service.List(c.Request.Context(), web.SearchTerm(c), c.Query("status"), page, size)
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, web.Page{Items: items, Total: total, Page: page, Size: size})
}

func (h *LayoutScenarioHandler) Get(c *gin.Context) {
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

func (h *LayoutScenarioHandler) Create(c *gin.Context) {
	var req dto.CreateLayoutScenarioRequest
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

func (h *LayoutScenarioHandler) Evaluate(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	var req dto.EvaluateScenarioRequest
	if !web.BindJSON(c, &req) {
		return
	}
	item, err := h.service.Evaluate(c.Request.Context(), id, req.Version, auditFrom(c))
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, item)
}

func (h *LayoutScenarioHandler) Transition(c *gin.Context) {
	id, ok := web.ParamID(c)
	if !ok {
		return
	}
	var req dto.TransitionScenarioRequest
	if !web.BindJSON(c, &req) {
		return
	}
	role := web.Role(c)
	if req.TargetStatus == constants.ScenarioApproved && role != auth.RoleReviewer && role != auth.RoleAdmin {
		web.Fail(c, web.Forbidden("reviewer or admin role is required to approve a scenario"))
		return
	}
	if req.TargetStatus == constants.ScenarioArchived && role != auth.RoleAdmin {
		web.Fail(c, web.Forbidden("admin role is required to archive a scenario"))
		return
	}
	item, err := h.service.Transition(c.Request.Context(), id, req, auditFrom(c))
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, item)
}

func (h *LayoutScenarioHandler) Compare(c *gin.Context) {
	leftID, ok := web.ParamID(c)
	if !ok {
		return
	}
	rightValue, err := strconv.ParseUint(c.Query("right_id"), 10, 64)
	if err != nil || rightValue == 0 {
		web.Fail(c, web.BadRequest("INVALID_RIGHT_ID", "right_id must be a positive scenario id", err))
		return
	}
	comparison, err := h.service.Compare(c.Request.Context(), leftID, uint(rightValue))
	if err != nil {
		web.Fail(c, err)
		return
	}
	web.OK(c, comparison)
}
