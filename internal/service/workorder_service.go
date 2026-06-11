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
func Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	return global.DB.WithContext(ctx).Transaction(func(txConn *gorm.DB) error {
		txCtx := pkgtx.WithValue(ctx, txConn)
		return fn(txCtx)
	})
}

// -----------------------------------------------------
// Student Operations
// -----------------------------------------------------

type SubmitWorkOrderInput struct {
	Content      string `json:"content"`
	ContactPhone string `json:"contact_phone"`
	ImageURL     string `json:"image_url"`
	Location     string `json:"location"`
}

func (s *WorkOrderService) CreateOrder(ctx context.Context, userID uint, input *SubmitWorkOrderInput) error {
	order := &model.WorkOrder{
		UserID:       userID,
		Content:      input.Content,
		ContactPhone: input.ContactPhone,
		ImageURL:     input.ImageURL,
		Status:       model.StatusPendingProcessing,
		Location:     input.Location,
	}
	if order.Location == "" {
		order.Location = "宿舍楼A栋"
	}

	err := s.workOrderRepo.CreateOrder(ctx, order)
	if err == nil {
		s.invalidateListCache(ctx)
	}
	return err
}

type EvaluateOrderInput struct {
	Rating int `json:"rating" binding:"required,min=1,max=5"`
}

func (s *WorkOrderService) EvaluateOrder(ctx context.Context, orderID uint, userID uint, input *EvaluateOrderInput) error {
	order, err := s.workOrderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

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
		// Redis Leaderboard Update
		currentMonth := time.Now().Format("200601")
		cacheKey := fmt.Sprintf("worker_leaderboard:%s", currentMonth)
		global.Redis.ZIncrBy(ctx, cacheKey, float64(input.Rating), fmt.Sprintf("%d", *order.WorkerID))
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

func (s *WorkOrderService) AssignWorker(ctx context.Context, orderID uint, input *AssignWorkerInput) error {
	return Transaction(ctx, func(txCtx context.Context) error {
		order, err := s.workOrderRepo.GetOrderByID(txCtx, orderID)
		if err != nil {
			return err
		}

		if order.Status != model.StatusPendingProcessing {
			return errors.New("order is not pending assignment")
		}

		// 1. Update WorkOrder
		order.WorkerID = &input.WorkerID
		order.Status = model.StatusAssigned
		if err := s.workOrderRepo.UpdateOrder(txCtx, order); err != nil {
			return err
		}

		// 2. Generate Notice
		notice := &model.Notice{
			UserID:  input.WorkerID,
			Message: fmt.Sprintf("系统分配了新的报修工单给您，工单号: %d", order.ID),
		}
		if err := s.workOrderRepo.CreateNotice(txCtx, notice); err != nil {
			return err
		}

		return nil
	})
}

// -----------------------------------------------------
// Worker Operations
// -----------------------------------------------------

type UpdateStatusInput struct {
	Status string `json:"status" binding:"required,oneof='维修中' '已完工'"`
}

func (s *WorkOrderService) UpdateStatusByWorker(ctx context.Context, orderID uint, workerID uint, input *UpdateStatusInput) error {
	order, err := s.workOrderRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return err
	}

	if order.WorkerID == nil || *order.WorkerID != workerID {
		return errors.New("you are not assigned to this order")
	}

	if input.Status == string(model.StatusProcessing) && order.Status == model.StatusAssigned {
		order.Status = model.StatusProcessing
	} else if input.Status == string(model.StatusCompleted) && order.Status == model.StatusProcessing {
		order.Status = model.StatusCompleted
	} else {
		return errors.New("invalid status transition")
	}

	err = s.workOrderRepo.UpdateOrder(ctx, order)
	if err == nil {
		s.invalidateListCache(ctx)
	}
	return err
}

// GrabWorkOrder维修师傅自主并发抢单（Redis分布式锁控制，防重复处理）
func (s *WorkOrderService) GrabWorkOrder(ctx context.Context, orderID uint, workerID uint) error {
	lockKey := fmt.Sprintf("lock:workorder:%d", orderID)
	lockVal := fmt.Sprintf("%d_%d", workerID, time.Now().UnixNano())

	// 尝试获取锁，设置 5 秒超时时间防死锁
	acquired, err := global.Redis.SetNX(ctx, lockKey, lockVal, 5*time.Second).Result()
	if err != nil || !acquired {
		return errors.New("当前工单正在被其他师傅抢占，请稍后重试")
	}

	defer func() {
		// 使用 Lua 脚本释放锁，防止释放他人的锁
		luaScript := `
			if redis.call("get", KEYS[1]) == ARGV[1] then
				return redis.call("del", KEYS[1])
			else
				return 0
			end`
		_, _ = global.Redis.Eval(ctx, luaScript, []string{lockKey}, lockVal).Result()
	}()

	return Transaction(ctx, func(txCtx context.Context) error {
		order, err := s.workOrderRepo.GetOrderByID(txCtx, orderID)
		if err != nil {
			return err
		}

		if order.Status != model.StatusPendingProcessing || order.WorkerID != nil {
			return errors.New("该工单已被分配或抢占")
		}

		order.WorkerID = &workerID
		order.Status = model.StatusAssigned
		return s.workOrderRepo.UpdateOrder(txCtx, order)
	})
}

type ConsumableUseInput struct {
	ConsumableID uint `json:"consumable_id"`
	Quantity     int  `json:"quantity"`
}

// CompleteOrderWithConsumables 维保完工并扣减耗材库存（支持悲观锁与级联事务回滚）
func (s *WorkOrderService) CompleteOrderWithConsumables(ctx context.Context, orderID uint, workerID uint, items []ConsumableUseInput) error {
	return Transaction(ctx, func(txCtx context.Context) error {
		db, _ := pkgtx.FromContext(txCtx)

		// 1. 获取并锁定工单行 (FOR UPDATE)
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

		// 2. 依次扣减耗材库存并生成明细
		for _, item := range items {
			var consumable model.Consumable
			if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).First(&consumable, item.ConsumableID).Error; err != nil {
				return errors.New("物料ID不存在")
			}

			if consumable.Stock < item.Quantity {
				return fmt.Errorf("物料库存不足: %s (当前库存: %d)", consumable.Name, consumable.Stock)
			}

			// 扣减库存
			consumable.Stock -= item.Quantity
			if err := db.Save(&consumable).Error; err != nil {
				return err
			}

			// 创建使用记录
			woc := &model.WorkOrderConsumable{
				WorkOrderID:  orderID,
				ConsumableID: item.ConsumableID,
				Quantity:     item.Quantity,
			}
			if err := db.Create(woc).Error; err != nil {
				return err
			}
		}

		// 3. 更新工单状态为已完工
		order.Status = model.StatusCompleted
		err := db.Save(&order).Error
		if err == nil {
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

func (s *WorkOrderService) ListOrders(ctx context.Context, page, pageSize int, userID, workerID *uint, status string) (*ListOrdersOutput, error) {
	offset := (page - 1) * pageSize
	cacheKey := fmt.Sprintf("workorder_list:%d:%d:%v:%v:%s", page, pageSize, userID, workerID, status)

	// 1. Check Redis Cache
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

	// 2. singleflight
	result, err, _ := s.sfGroup.Do(cacheKey, func() (interface{}, error) {
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

	// 3. Write cache back with random TTL jitter
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomSeconds := r.Intn(60)
	ttl := 5*time.Minute + time.Duration(randomSeconds)*time.Second

	if len(output.Items) == 0 {
		global.Redis.Set(ctx, cacheKey, "empty", 1*time.Minute)
		global.Redis.SAdd(ctx, "workorder_list_keys", cacheKey)
	} else {
		if cacheBytes, err := json.Marshal(output); err == nil {
			global.Redis.Set(ctx, cacheKey, cacheBytes, ttl)
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
