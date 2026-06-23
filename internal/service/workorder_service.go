package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
	"dorm-repair-system/internal/repository"
	pkgtx "dorm-repair-system/pkg/tx"

	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type IWorkOrderService interface {
	CreateOrder(ctx context.Context, userID uint, input *SubmitWorkOrderInput) error
	EvaluateOrder(ctx context.Context, orderID uint, userID uint, input *EvaluateOrderInput) error
	AssignWorker(ctx context.Context, orderID uint, input *AssignWorkerInput) error
	UpdateStatusByWorker(ctx context.Context, orderID uint, workerID uint, input *UpdateStatusInput) error
	ListOrders(ctx context.Context, page, pageSize int, userID, workerID *uint, status string) (*ListOrdersOutput, error)
	GetMonthlyWorkerLeaderboard(ctx context.Context, limit int64) ([]redis.Z, error)

	// New Advanced Features
	GrabWorkOrder(ctx context.Context, orderID uint, workerID uint) error
	CompleteOrderWithConsumables(ctx context.Context, orderID uint, workerID uint, items []ConsumableUseInput) error
	GetLocationStats(ctx context.Context) ([]repository.LocationStat, error)
	GetWorkerEfficiency(ctx context.Context) ([]repository.WorkerEfficiencyStat, error)
}

type WorkOrderService struct {
	workOrderRepo repository.IWorkOrderRepository
	sfGroup       singleflight.Group
}

func NewWorkOrderService(repo repository.IWorkOrderRepository) IWorkOrderService {
	return &WorkOrderService{
		workOrderRepo: repo,
	}
}

// Transaction helper for clean GORM transaction propagation
// 事务帮助函数：用于实现优雅的 GORM 事务传播
func Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return global.DB.WithContext(ctx).Transaction(func(txConn *gorm.DB) error {
		txCtx := pkgtx.WithValue(ctx, txConn)
		return fn(txCtx)
	})
}

// -----------------------------------------------------
// 学生操作方法 (Student Operations)
// -----------------------------------------------------

type SubmitWorkOrderInput struct {
	Content      string `json:"content"`
	ContactPhone string `json:"contact_phone"`
	ImageURL     string `json:"image_url"`
	Location     string `json:"location"`
}

// CreateOrder：学生发起报修工单提交
func (s *WorkOrderService) CreateOrder(ctx context.Context, userID uint, input *SubmitWorkOrderInput) error {
	order := &model.WorkOrder{
		UserID:       userID,
		Content:      input.Content,
		ContactPhone: input.ContactPhone,
		ImageURL:     input.ImageURL,
		Status:       model.StatusPendingProcessing, // 初始状态设为：待指派
		Location:     input.Location,
	}
	if order.Location == "" {
		order.Location = "宿舍楼A栋"
	}

	err := s.workOrderRepo.CreateOrder(ctx, order)
	if err == nil {
		// 清除 Redis 工单列表缓存，确保报修大厅能实时看到新提交的工单
		s.invalidateListCache(ctx)
	}
	return err
}

type EvaluateOrderInput struct {
	Rating int `json:"rating" binding:"required,min=1,max=5"`
}

// EvaluateOrder：学生对已完成的维修工单进行评分
func (s *WorkOrderService) EvaluateOrder(ctx context.Context, orderID uint, userID uint, input *EvaluateOrderInput) error {
	order, err := s.workOrderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	// 权限与状态检查：只能评价属于自己的且状态为“已完工”的工单
	if order.UserID != userID {
		return errors.New("you can only evaluate your own orders")
	}
	if order.Status != model.StatusCompleted {
		return errors.New("can only evaluate completed orders")
	}
	if order.WorkerID == nil {
		return errors.New("invalid worker assignment")
	}

	order.Status = model.StatusEvaluated
	order.Rating = input.Rating

	err = s.workOrderRepo.UpdateOrder(ctx, order)
	if err == nil {
		// 评分成功后，异步累加写入 Redis Sorted Set (ZSET)，更新当月金牌师傅排行榜
		currentMonth := time.Now().Format("200601")
		cacheKey := fmt.Sprintf("worker_leaderboard:%s", currentMonth)
		global.Redis.ZIncrBy(ctx, cacheKey, float64(input.Rating), fmt.Sprintf("%d", *order.WorkerID))
		
		// 清除列表缓存，刷新工单状态
		s.invalidateListCache(ctx)
	}
	return err
}

// -----------------------------------------------------
// Admin Operations
// -----------------------------------------------------

type AssignWorkerInput struct {
	WorkerID uint `json:"worker_id" binding:"required"`
}

// AssignWorker：管理员/宿管分配特定的工单给选定的维修师傅
func (s *WorkOrderService) AssignWorker(ctx context.Context, orderID uint, input *AssignWorkerInput) error {
	// 使用本地声明的 Transaction 辅助函数进行声明式数据库事务封装，确保工单指派与通知生成为原子操作
	err := Transaction(ctx, func(txCtx context.Context) error {
		order, err := s.workOrderRepo.GetOrderByID(txCtx, orderID)
		if err != nil {
			return err
		}

		// 检查工单状态：只能指派处于“待指派”状态的工单
		if order.Status != model.StatusPendingProcessing {
			return errors.New("order is not pending assignment")
		}

		// 1. 更新工单指派状态及负责人 ID
		order.WorkerID = &input.WorkerID
		order.Status = model.StatusAssigned
		if err := s.workOrderRepo.UpdateOrder(txCtx, order); err != nil {
			return err
		}

		// 2. 生成系统站内消息通知对应的维修师傅
		notice := &model.Notice{
			UserID:  input.WorkerID,
			Message: fmt.Sprintf("系统分配了新的报修工单给您，工单号: %d", order.ID),
		}
		if err := s.workOrderRepo.CreateNotice(txCtx, notice); err != nil {
			return err
		}

		return nil
	})

	if err == nil {
		// 事务提交成功后，及时清除 Redis 缓存，防止前端读到旧列表缓存导致状态未刷新
		s.invalidateListCache(ctx)
	}
	return err
}

// -----------------------------------------------------
// Worker Operations
// -----------------------------------------------------

type UpdateStatusInput struct {
	Status string `json:"status" binding:"required,oneof='维修中' '已完工'"`
}

// UpdateStatusByWorker：维修师傅推进工单状态（如：已指派 -> 维修中）
func (s *WorkOrderService) UpdateStatusByWorker(ctx context.Context, orderID uint, workerID uint, input *UpdateStatusInput) error {
	order, err := s.workOrderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	// 鉴权：只有当前工单指派的责任维修工才能修改其状态
	if order.WorkerID == nil || *order.WorkerID != workerID {
		return errors.New("you are not assigned to this order")
	}

	// 状态机状态转换校验
	if input.Status == string(model.StatusProcessing) && order.Status == model.StatusAssigned {
		order.Status = model.StatusProcessing
	} else if input.Status == string(model.StatusCompleted) && order.Status == model.StatusProcessing {
		order.Status = model.StatusCompleted
	} else {
		return errors.New("invalid status transition")
	}

	err = s.workOrderRepo.UpdateOrder(ctx, order)
	if err == nil {
		// 更新成功清除工单列表缓存
		s.invalidateListCache(ctx)
	}
	return err
}

// GrabWorkOrder维修师傅自主并发抢单（Redis分布式锁控制，防重复处理）
// GrabWorkOrder：维修师傅并发抢单
// 使用 Redis SetNX 分布式锁控制并发，防止在高并发高负载下出现同一个工单被多名师傅重复抢占的情况
func (s *WorkOrderService) GrabWorkOrder(ctx context.Context, orderID uint, workerID uint) error {
	lockKey := fmt.Sprintf("lock:workorder:%d", orderID)
	lockVal := fmt.Sprintf("%d_%d", workerID, time.Now().UnixNano())

	// 尝试获取锁，设置 5 秒超时时间防死锁
	acquired, err := global.Redis.SetNX(ctx, lockKey, lockVal, 5*time.Second).Result()
	if err != nil || !acquired {
		return errors.New("当前工单正在被其他师傅抢占，请稍后重试")
	}

	defer func() {
		// 使用 Lua 脚本安全地释放 Redis 锁（实现 CAS 操作），避免因网络延迟或业务超时误删其他师傅持有的锁
		luaScript := `
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end`
		_, _ = global.Redis.Eval(ctx, luaScript, []string{lockKey}, lockVal).Result()
	}()

	err = Transaction(ctx, func(txCtx context.Context) error {
		order, err := s.workOrderRepo.GetOrderByID(txCtx, orderID)
		if err != nil {
			return err
		}

		// 检查状态：只能抢占未分配（待指派）且尚未指派给任何人的工单
		if order.Status != model.StatusPendingProcessing || order.WorkerID != nil {
			return errors.New("该工单已被分配或抢占")
		}

		order.WorkerID = &workerID
		order.Status = model.StatusAssigned
		return s.workOrderRepo.UpdateOrder(txCtx, order)
	})

	if err == nil {
		// 抢单成功后，清除 Redis 缓存以刷新大厅页面状态
		s.invalidateListCache(ctx)
	}
	return err
}

type ConsumableUseInput struct {
	ConsumableID uint `json:"consumable_id"`
	Quantity     int  `json:"quantity"`
}

// CompleteOrderWithConsumables：维保完工并扣减耗材库存
// 支持对工单行与耗材物料行使用数据库悲观锁 (FOR UPDATE) 配合多级事务，防止出现负库存超卖，并在失败时自动回滚所有数据库状态
func (s *WorkOrderService) CompleteOrderWithConsumables(ctx context.Context, orderID uint, workerID uint, items []ConsumableUseInput) error {
	return Transaction(ctx, func(txCtx context.Context) error {
		db, _ := pkgtx.FromContext(txCtx)

		// 1. 获取并锁定工单数据行 (FOR UPDATE 悲观锁) 防止并发修改
		var order model.WorkOrder
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&order, orderID).Error; err != nil {
			return err
		}

		if order.WorkerID == nil || *order.WorkerID != workerID {
			return errors.New("you are not assigned to this order")
		}

		if order.Status != model.StatusProcessing {
			return errors.New("invalid status transition: must be processing")
		}

		// 2. 遍历使用的所有耗材，依次对物料行进行上锁并进行扣减
		for _, item := range items {
			var consumable model.Consumable
			// 悲观锁锁定特定耗材行防止并发扣减造成库存穿透
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&consumable, item.ConsumableID).Error; err != nil {
				return errors.New("物料ID不存在")
			}

			// 库存校验防超卖
			if consumable.Stock < item.Quantity {
				return fmt.Errorf("物料库存不足: %s (当前库存: %d)", consumable.Name, consumable.Stock)
			}

			// 扣减库存并保存
			consumable.Stock -= item.Quantity
			if err := db.Save(&consumable).Error; err != nil {
				return err
			}

			// 创建工单与耗材物料使用的明细关联记录
			woc := &model.WorkOrderConsumable{
				WorkOrderID:  orderID,
				ConsumableID: item.ConsumableID,
				Quantity:     item.Quantity,
			}
			if err := db.Create(woc).Error; err != nil {
				return err
			}
		}

		// 3. 将工单更新为“已完工”状态
		order.Status = model.StatusCompleted
		err := db.Save(&order).Error
		if err == nil {
			// 清除列表缓存，刷新前端展示
			s.invalidateListCache(ctx)
		}
		return err
	})
}

// -----------------------------------------------------
// Query Operations
// -----------------------------------------------------

type ListOrdersOutput struct {
	Total int64             `json:"total"`
	Items []model.WorkOrder `json:"items"`
}

// ListOrders：查询及过滤实时报修工单列表
// 实施了高并发查询优化：
// 1) Redis Cache-Aside：优先读取 Redis 缓存；
// 2) Cache Avalanche Guard：对缓存写回使用了随机过期时间抖动（Jitter），防止大量缓存并发在同一时刻集体失效；
// 3) Cache Breakdown Guard：使用 golang.org/x/sync/singleflight 机制，当缓存未命中时只允许一个并发请求穿透查库，合并其他相同 key 请求，防瞬间击穿数据库；
// 4) Cache Penetration Guard：当查询结果为空时在 Redis 中缓存特殊占位符 "empty"，防止恶意空数据请求持续穿透打爆 MySQL。
func (s *WorkOrderService) ListOrders(ctx context.Context, page, pageSize int, userID, workerID *uint, status string) (*ListOrdersOutput, error) {
	offset := (page - 1) * pageSize
	cacheKey := fmt.Sprintf("workorder_list:%d:%d:%v:%v:%s", page, pageSize, userID, workerID, status)

	// 1. 第一关：检查 Redis 缓存
	val, err := global.Redis.Get(ctx, cacheKey).Result()
	if err == nil {
		if val == "empty" {
			// 防穿透占位符命中，说明数据库确实无对应数据，直接拦截返回空
			return &ListOrdersOutput{Total: 0, Items: []model.WorkOrder{}}, nil
		}
		var output ListOrdersOutput
		if json.Unmarshal([]byte(val), &output) == nil {
			return &output, nil
		}
	}

	// 2. 第二关：使用 singleflight 防并发查库击穿
	result, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
		// 在临界区内双重校验缓存（Double-Checked Locking 变体），防止排队等待期间缓存已被前驱协程写入
		val, err := global.Redis.Get(ctx, cacheKey).Result()
		if err == nil {
			if val == "empty" {
				return &ListOrdersOutput{Total: 0, Items: []model.WorkOrder{}}, nil
			}
			var output ListOrdersOutput
			if json.Unmarshal([]byte(val), &output) == nil {
				return &output, nil
			}
		}

		// 缓存彻底未命中，安全穿透查询关系型数据库 (MySQL)
		orders, total, err := s.workOrderRepo.ListOrders(ctx, offset, pageSize, userID, workerID, status)
		if err != nil {
			return nil, err
		}

		return &ListOrdersOutput{
			Total: total,
			Items: orders,
		}, nil
	})

	if err != nil {
		return nil, err
	}

	output := result.(*ListOrdersOutput)

	// 3. 第三关：对查出的结果重新写入缓存，加入随机生存时间抖动 (Jitter)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSeconds := r.Intn(60) // 随机波动 0-60 秒
	ttl := 5*time.Minute + time.Duration(randomSeconds)*time.Second

	if len(output.Items) == 0 {
		// 对空结果在 Redis 中缓存 1 分钟的占位符 "empty"，避免反复穿透
		global.Redis.Set(ctx, cacheKey, "empty", 1*time.Minute)
		global.Redis.SAdd(ctx, "workorder_list_keys", cacheKey)
	} else {
		if cacheBytes, err := json.Marshal(output); err == nil {
			global.Redis.Set(ctx, cacheKey, cacheBytes, ttl)
			// 将当前缓存的 Key 维护在一个 Redis Set 中，方便在数据发生修改时执行级联批量缓存清理（主动失效）
			global.Redis.SAdd(ctx, "workorder_list_keys", cacheKey)
		}
	}

	return output, nil
}

func (s *WorkOrderService) invalidateListCache(ctx context.Context) {
	keys, err := global.Redis.SMembers(ctx, "workorder_list_keys").Result()
	if err != nil || len(keys) == 0 {
		return
	}

	pipe := global.Redis.Pipeline()
	for _, key := range keys {
		pipe.Del(ctx, key)
	}
	pipe.Del(ctx, "workorder_list_keys")
	_, _ = pipe.Exec(ctx)
}

func (s *WorkOrderService) GetMonthlyWorkerLeaderboard(ctx context.Context, limit int64) ([]redis.Z, error) {
	currentMonth := time.Now().Format("200601")
	cacheKey := fmt.Sprintf("worker_leaderboard:%s", currentMonth)
	return global.Redis.ZRevRangeWithScores(ctx, cacheKey, 0, limit-1).Result()
}

func (s *WorkOrderService) GetLocationStats(ctx context.Context) ([]repository.LocationStat, error) {
	return s.workOrderRepo.GetLocationReportStats(ctx)
}

func (s *WorkOrderService) GetWorkerEfficiency(ctx context.Context) ([]repository.WorkerEfficiencyStat, error) {
	return s.workOrderRepo.GetWorkerEfficiencyStats(ctx)
}
