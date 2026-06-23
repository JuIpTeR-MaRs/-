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

// CreateOrder 接口：学生提交报修工单
func (ctrl *WorkOrderController) CreateOrder(c *gin.Context) {
	var req SubmitWorkOrderRequest
	// 绑定 JSON 请求参数并执行模型绑定验证（内容长度限制在 10-200，手机号必须是11位数字等）
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 从 JWT 中间件解析存入上下文中的当前登录学生 ID
	userID := c.MustGet("userID").(uint)
	input := &service.SubmitWorkOrderInput{
		Content:      req.Content,
		ContactPhone: req.ContactPhone,
		ImageURL:     req.ImageURL,
		Location:     req.Location,
	}

	// 调用服务层逻辑执行数据库写入
	if err := ctrl.workOrderService.CreateOrder(c.Request.Context(), userID, input); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	// 统一格式成功返回
	response.Success(c, nil)
}

// EvaluateOrder 接口：学生对已完成的维修工单进行星级评价
func (ctrl *WorkOrderController) EvaluateOrder(c *gin.Context) {
	// 从路径参数中解析工单 ID
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.EvaluateOrderInput
	// 绑定并验证评价参数（评星限制在 1-5 星之间）
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 获取当前登录学生 ID
	userID := c.MustGet("userID").(uint)
	// 调用服务层进行评价流转与排行榜积分累计
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

// AssignWorker 接口：宿管/管理员指派维修师傅
func (ctrl *WorkOrderController) AssignWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.AssignWorkerInput
	// 绑定分配请求，要求必须有被指派的师傅 ID (worker_id)
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 推进业务层工单指派事务
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

// UpdateStatusByWorker 接口：维修师傅推进状态（如：接受任务后标记“维修中”）
func (ctrl *WorkOrderController) UpdateStatusByWorker(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var input service.UpdateStatusInput
	// 校验要修改的状态参数只能是 “维修中” 或 “已完工”
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	// 获取当前操作的师傅 ID 并流转工单状态
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

// ListOrders 接口：查询并获取工单列表，支持多条件过滤与分页
func (ctrl *WorkOrderController) ListOrders(c *gin.Context) {
	// 获取分页与每页显示行数，默认第一页，每页10条
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	status := c.Query("status")
	
	// 按学生 ID 过滤
	var userIDPtr *uint
	if userIDStr := c.Query("user_id"); userIDStr != "" {
		if id, err := strconv.ParseUint(userIDStr, 10, 32); err == nil {
			uid := uint(id)
			userIDPtr = &uid
		}
	}

	// 按负责师傅 ID 过滤
	var workerIDPtr *uint
	if workerIDStr := c.Query("worker_id"); workerIDStr != "" {
		if id, err := strconv.ParseUint(workerIDStr, 10, 32); err == nil {
			wid := uint(id)
			workerIDPtr = &wid
		}
	}

	// 获取当前登录用户角色与用户 ID，实现“按角色数据隔离”
	role := c.MustGet("role").(string)
	currentUserID := c.MustGet("userID").(uint)
	
	if role == "Student" {
		// 学生只能查询并看到自己发起的工单
		userIDPtr = &currentUserID
	} else if role == "Worker" {
		// 维修师傅对于已经分配的工单只能查看自己负责的，如果是“待指派”工单，则可以跨区域查看以自主抢单
		if status != "待指派" {
			workerIDPtr = &currentUserID
		}
	}

	// 调用服务层获取缓存/数据库混合数据
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
// GrabOrder 接口：维修师傅自主从待指派大厅中抢占工单
func (ctrl *WorkOrderController) GrabOrder(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	// 获取操作的师傅 ID
	workerID := c.MustGet("userID").(uint)
	// 调用含有分布式锁的抢单逻辑
	if err := ctrl.workOrderService.GrabWorkOrder(c.Request.Context(), uint(orderID), workerID); err != nil {
		response.Fail(c, e.ServerPanic, err.Error())
		return
	}

	response.Success(c, nil)
}

// CompleteOrderWithConsumables handles completing an order with inventory depletion
// CompleteOrderWithConsumables 接口：维修师傅登记完工并输入实际消耗的耗材清单
func (ctrl *WorkOrderController) CompleteOrderWithConsumables(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		response.Fail(c, e.InvalidParams, "invalid order id")
		return
	}

	var req []service.ConsumableUseInput
	// 绑定使用的耗材物料数组（含物料ID及消耗数量）
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Fail(c, e.InvalidParams, response.TranslateError(err))
		return
	}

	workerID := c.MustGet("userID").(uint)
	// 调用扣库存悲观锁事务操作
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
