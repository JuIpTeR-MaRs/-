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

// SubmitWorkOrderRequest 定义提交工单的请求参数及校验规则
type SubmitWorkOrderRequest struct {
	Content      string `json:"content" binding:"required,min=10,max=200"`
	ContactPhone string `json:"contact_phone" binding:"required,len=11,numeric"`
	ImageURL     string `json:"image_url"`
	Location     string `json:"location"`
}

// 学生提交报修工单
func (ctrl *WorkOrderController) CreateOrder(c *gin.Context) {
	var req SubmitWorkOrderRequest
	// 参数绑定和校验
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 获取当前登录用户 ID
	userID := c.MustGet("userID").(uint)
	input := &service.SubmitWorkOrderInput{
		Content:      req.Content,
		ContactPhone: req.ContactPhone,
		ImageURL:     req.ImageURL,
		Location:     req.Location,
	}

	// 调用 Service 层处理
	if err := ctrl.workOrderService.CreateOrder(c.Request.Context(), userID, input); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// 学生评价工单
func (ctrl *WorkOrderController) EvaluateOrder(c *gin.Context) {
	// 解析工单 ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.EvaluateOrderInput
	// 绑定并验证评价参数
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	userID := c.MustGet("userID").(uint)
	// 调用 Service 评价工单
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

// 宿管指派工单
func (ctrl *WorkOrderController) AssignWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.AssignWorkerInput
	// 绑定指派参数
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 调用指派逻辑
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

// 维修工更新工单状态
func (ctrl *WorkOrderController) UpdateStatusByWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.UpdateStatusInput
	// 绑定状态参数
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

// 获取工单列表
func (ctrl *WorkOrderController) ListOrders(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	
	// 过滤条件
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

	// 数据隔离隔离限制
	role := c.MustGet("role").(string)
	currentUserID := c.MustGet("userID").(uint)
	
	if role == "Student" {
		// 学生只能看自己提交的
		userIDPtr = &currentUserID
	} else if role == "Worker" {
		// 师傅只能看自己的，或者是所有待指派的
		if status != "待指派" {
			workerIDPtr = &currentUserID
		}
	}

	// 查询数据
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

// GrabOrder 维修工自主抢单（使用 Redis 锁控制并发）
// 维修工自助抢单
func (ctrl *WorkOrderController) GrabOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	workerID := c.MustGet("userID").(uint)
	// 抢单逻辑
	if err := ctrl.workOrderService.GrabWorkOrder(c.Request.Context(), uint(orderID), workerID); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// CompleteOrderWithConsumables 维修工提交完成工单并扣减耗材库存
// 完工并登记耗材
func (ctrl *WorkOrderController) CompleteOrderWithConsumables(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var req []service.ConsumableUseInput
	// 绑定消耗耗材列表
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	workerID := c.MustGet("userID").(uint)
	// 调用 Service 逻辑
	if err := ctrl.workOrderService.CompleteOrderWithConsumables(c.Request.Context(), uint(orderID), workerID, req); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// GetLocationStats 统计报修地点频率分布数据
func (ctrl *WorkOrderController) GetLocationStats(c *gin.Context) {
	stats, err := ctrl.workOrderService.GetLocationStats(c.Request.Context())
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, stats)
}

// GetWorkerEfficiency 统计维修师傅的工单处理时效/效率数据
func (ctrl *WorkOrderController) GetWorkerEfficiency(c *gin.Context) {
	stats, err := ctrl.workOrderService.GetWorkerEfficiency(c.Request.Context())
	if err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, stats)
}

// TestPanic 测试用 Panic 触发接口，用于验证全局异常恢复中间件功能
func (ctrl *WorkOrderController) TestPanic(c *gin.Context) {
	panic("This is an intentional panic to test recovery middleware!")
}
