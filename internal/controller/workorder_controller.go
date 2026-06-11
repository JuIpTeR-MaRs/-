package controller

import (
	"strconv"

	"dorm-repair-system/internal/service"
	"dorm-repair-system/pkg/e"
	"dorm-repair-system/pkg/response"

	"github.com/gin-gonic/gin"
)

type WorkOrderController struct {
	workOrderService service.IWorkOrderService
}

func NewWorkOrderController(s service.IWorkOrderService) *WorkOrderController {
	return &WorkOrderController{
		workOrderService: s,
	}
}

// SubmitWorkOrderRequest defines the validation rules for creating an order
type SubmitWorkOrderRequest struct {
	Content      string `json:"content" binding:"required,min=10,max=200"`
	ContactPhone string `json:"contact_phone" binding:"required,len=11,numeric"`
	ImageURL     string `json:"image_url"`
	Location     string `json:"location"`
}

func (ctrl *WorkOrderController) CreateOrder(c *gin.Context) {
	var req SubmitWorkOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	userID := c.MustGet("userID").(uint)
	input := &service.SubmitWorkOrderInput{
		Content:      req.Content,
		ContactPhone: req.ContactPhone,
		ImageURL:     req.ImageURL,
		Location:     req.Location,
	}

	if err := ctrl.workOrderService.CreateOrder(c.Request.Context(), userID, input); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) EvaluateOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.EvaluateOrderInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	userID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.EvaluateOrder(c.Request.Context(), uint(orderID), userID, &input); err != nil {
		errMsg := err.Error()
		if errMsg == "you can only evaluate your own orders" {
			response.Fail(c, e.OrderAuthError)
		} else if errMsg == "can only evaluate completed orders" || errMsg == "invalid worker assignment" {
			response.Fail(c, e.OrderStateError, errMsg)
		} else {
			response.Fail(c, e.ServerPanic, errMsg)
		}
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) AssignWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.AssignWorkerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	if err := ctrl.workOrderService.AssignWorker(c.Request.Context(), uint(orderID), &input); err != nil {
		errMsg := err.Error()
		if errMsg == "order is not pending assignment" {
			response.Fail(c, e.OrderStateError)
		} else {
			response.Fail(c, e.ServerPanic, errMsg)
		}
		return
	}

	response.Success(c, nil)
}

func (ctrl *WorkOrderController) UpdateStatusByWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.UpdateStatusInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	workerID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.UpdateStatusByWorker(c.Request.Context(), uint(orderID), workerID, &input); err != nil {
		errMsg := err.Error()
		if errMsg == "you are not assigned to this order" {
			response.Fail(c, e.OrderAuthError)
		} else if errMsg == "invalid status transition" {
			response.Fail(c, e.OrderStateError)
		} else {
			response.Fail(c, e.ServerPanic, errMsg)
		}
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

	role := c.MustGet("role").(string)
	currentUserID := c.MustGet("userID").(uint)
	
	if role == "Student" {
		userIDPtr = &currentUserID
	} else if role == "Worker" {
		if status != "待指派" {
			workerIDPtr = &currentUserID
		}
	}

	output, err := ctrl.workOrderService.ListOrders(c.Request.Context(), page, pageSize, userIDPtr, workerIDPtr, status)
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, output)
}

func (ctrl *WorkOrderController) GetWorkerLeaderboard(c *gin.Context) {
	limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
	
	leaderboard, err := ctrl.workOrderService.GetMonthlyWorkerLeaderboard(c.Request.Context(), limit)
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, leaderboard)
}

// GrabOrder handles autonomous worker grabbing of an order using Redis lock
func (ctrl *WorkOrderController) GrabOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	workerID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.GrabWorkOrder(c.Request.Context(), uint(orderID), workerID); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// CompleteOrderWithConsumables handles completing an order with inventory depletion
func (ctrl *WorkOrderController) CompleteOrderWithConsumables(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var req []service.ConsumableUseInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	workerID := c.MustGet("userID").(uint)
	if err := ctrl.workOrderService.CompleteOrderWithConsumables(c.Request.Context(), uint(orderID), workerID, req); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetLocationStats handles building report statistics for location frequency
func (ctrl *WorkOrderController) GetLocationStats(c *gin.Context) {
	stats, err := ctrl.workOrderService.GetLocationStats(c.Request.Context())
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, stats)
}

// GetWorkerEfficiency handles building statistics for worker completion efficiency
func (ctrl *WorkOrderController) GetWorkerEfficiency(c *gin.Context) {
	stats, err := ctrl.workOrderService.GetWorkerEfficiency(c.Request.Context())
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, stats)
}

// TestPanic endpoint to demonstrate global recovery middleware
func (ctrl *WorkOrderController) TestPanic(c *gin.Context) {
	panic("This is an intentional panic to test recovery middleware!")
}
