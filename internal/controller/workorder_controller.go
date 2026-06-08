package controller

import (
	"strconv"

	"dorm-repair-system/internal/service"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
)

type WorkOrderController struct {
	workOrderService *service.WorkOrderService
}

func NewWorkOrderController() *WorkOrderController {
	return &WorkOrderController{
		workOrderService: service.NewWorkOrderService(),
	}
}

// SubmitWorkOrderRequest defines the strict validation rules for creating an order
type SubmitWorkOrderRequest struct {
	Content      string `json:"content" binding:"required,min=10,max=200"`
	ContactPhone string `json:"contact_phone" binding:"required,len=11,numeric"`
	ImageURL     string `json:"image_url"`
}

func (ctrl *WorkOrderController) CreateOrder(c *gin.Context) {
	var req SubmitWorkOrderRequest
	// ShouldBindJSON triggers Validator. If fails, it returns 400 error automatically handled here.
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "参数错误: "+err.Error())
		return
	}

	userID := c.MustGet("userID").(uint)
	input := &service.SubmitWorkOrderInput{
		Content:      req.Content,
		ContactPhone: req.ContactPhone,
		ImageURL:     req.ImageURL,
	}

	if err := ctrl.workOrderService.CreateOrder(userID, input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) EvaluateOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "invalid order id")
		return
	}

	var input service.EvaluateOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "参数错误: "+err.Error())
		return
	}

	userID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.EvaluateOrder(uint(orderID), userID, &input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) AssignWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "invalid order id")
		return
	}

	var input service.AssignWorkerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "参数错误: "+err.Error())
		return
	}

	if err := ctrl.workOrderService.AssignWorker(uint(orderID), &input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) UpdateStatusByWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "invalid order id")
		return
	}

	var input service.UpdateStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.ErrorWithStatus(c, 400, response.CodeError, "参数错误: "+err.Error())
		return
	}

	workerID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.UpdateStatusByWorker(uint(orderID), workerID, &input); err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	
	var userIDPtr *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uid := uint(id)
			userIDPtr = &uid
		}
	}

	var workerIDPtr *uint
	if workerIDStr := c.Query("worker_id"); workerIDStr != "" {
		if id, err := strconv.ParseUint(workerIDStr, 10, 32); err == nil {
			wid := uint(id)
			workerIDPtr = &wid
		}
	}

	// Casbin should enforce endpoint level, but here we can add some business constraints
	role := c.MustGet("role").(string)
	currentUserID := c.MustGet("userID").(uint)
	
	if role == "Student" {
		userIDPtr = &currentUserID // Force filter by self
	} else if role == "Worker" {
		workerIDPtr = &currentUserID // Force filter by self
	}

	output, err := ctrl.workOrderService.ListOrders(page, pageSize, userIDPtr, workerIDPtr, status)
	if err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, output)
}

func (ctrl *WorkOrderController) GetWorkerLeaderboard(c *gin.Context) {
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
	
	leaderboard, err := ctrl.workOrderService.GetMonthlyWorkerLeaderboard(limit)
	if err != nil {
		response.Error(c, response.CodeError, err.Error())
		return
	}

	response.Success(c, leaderboard)
}

// TestPanic endpoint to demonstrate global recovery middleware
func (ctrl *WorkOrderController) TestPanic(c *gin.Context) {
	panic("This is an intentional panic to test recovery middleware!")
}
