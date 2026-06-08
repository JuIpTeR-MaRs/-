package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
	"dorm-repair-system/internal/repository"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type WorkOrderService struct {
	workOrderRepo *repository.WorkOrderRepository
}

func NewWorkOrderService() *WorkOrderService {
	return &WorkOrderService{
		workOrderRepo: repository.NewWorkOrderRepository(),
	}
}

// -----------------------------------------------------
// Student Operations
// -----------------------------------------------------

type SubmitWorkOrderInput struct {
	Content      string `json:"content"`
	ContactPhone string `json:"contact_phone"`
	ImageURL     string `json:"image_url"`
}

func (s *WorkOrderService) CreateOrder(userID uint, input *SubmitWorkOrderInput) error {
	order := &model.WorkOrder{
		UserID:       userID,
		Content:      input.Content,
		ContactPhone: input.ContactPhone,
		ImageURL:     input.ImageURL,
		Status:  model.StatusPendingProcessing,
	}

	err := s.workOrderRepo.CreateOrder(order)
	if err == nil {
		s.invalidateListCache()
	}
	return err
}

type EvaluateOrderInput struct {
	Rating int `json:"rating" binding:"required,min=1,max=5"`
}

func (s *WorkOrderService) EvaluateOrder(orderID uint, userID uint, input *EvaluateOrderInput) error {
	order, err := s.workOrderRepo.GetOrderByID(orderID)
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

	err = s.workOrderRepo.UpdateOrder(order)
	if err == nil {
		// Redis Leaderboard Highlight: Update worker score in ZSet based on rating
		// Use current month as key to generate monthly leaderboards
		currentMonth := time.Now().Format("200601")
		cacheKey := fmt.Sprintf("worker_leaderboard:%s", currentMonth)
		ctx := context.Background()
		
		// Add rating score to worker
		global.Redis.ZIncrBy(ctx, cacheKey, float64(input.Rating), fmt.Sprintf("%d", *order.WorkerID))
		s.invalidateListCache()
	}
	return err
}

// -----------------------------------------------------
// Admin Operations
// -----------------------------------------------------

type AssignWorkerInput struct {
	WorkerID uint `json:"worker_id" binding:"required"`
}

func (s *WorkOrderService) AssignWorker(orderID uint, input *AssignWorkerInput) error {
	// High-Score Highlight: GORM Transaction to update order and insert notice
	return global.DB.Transaction(func(tx *gorm.DB) error {
		var order model.WorkOrder
		if err := tx.First(&order, orderID).Error; err != nil {
			return err
		}

		if order.Status != model.StatusPendingProcessing {
			return errors.New("order is not pending assignment")
		}

		// 1. Update WorkOrder
		order.WorkerID = &input.WorkerID
		order.Status = model.StatusAssigned
		if err := tx.Save(&order).Error; err != nil {
			return err
		}

		// 2. Generate Notice
		notice := model.Notice{
			UserID:  input.WorkerID,
			Message: fmt.Sprintf("系统分配了新的报修工单给您，工单号: %d", order.ID),
		}
		if err := tx.Create(&notice).Error; err != nil {
			return err // Transaction will rollback
		}

		return nil // Transaction will commit
	})
}

// -----------------------------------------------------
// Worker Operations
// -----------------------------------------------------

type UpdateStatusInput struct {
	Status string `json:"status" binding:"required,oneof='维修中' '已完工'"`
}

func (s *WorkOrderService) UpdateStatusByWorker(orderID uint, workerID uint, input *UpdateStatusInput) error {
	order, err := s.workOrderRepo.GetOrderByID(orderID)
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

	err = s.workOrderRepo.UpdateOrder(order)
	if err == nil {
		s.invalidateListCache()
	}
	return err
}

// -----------------------------------------------------
// Query Operations
// -----------------------------------------------------

type ListOrdersOutput struct {
	Total int64               `json:"total"`
	Items []model.WorkOrder   `json:"items"`
}

func (s *WorkOrderService) ListOrders(page, pageSize int, userID, workerID *uint, status string) (*ListOrdersOutput, error) {
	offset := (page - 1) * pageSize
	cacheKey := fmt.Sprintf("workorder_list:%d:%d:%v:%v:%s", page, pageSize, userID, workerID, status)
	ctx := context.Background()

	val, err := global.Redis.Get(ctx, cacheKey).Result()
	if err == nil {
		var output ListOrdersOutput
		if json.Unmarshal([]byte(val), &output) == nil {
			return &output, nil
		}
	}

	orders, total, err := s.workOrderRepo.ListOrders(offset, pageSize, userID, workerID, status)
	if err != nil {
		return nil, err
	}

	output := &ListOrdersOutput{
		Total: total,
		Items: orders,
	}

	if cacheBytes, err := json.Marshal(output); err == nil {
		global.Redis.Set(ctx, cacheKey, cacheBytes, 5*time.Minute)
	}

	return output, nil
}

func (s *WorkOrderService) invalidateListCache() {
	ctx := context.Background()
	iter := global.Redis.Scan(ctx, 0, "workorder_list:*", 0).Iterator()
	for iter.Next(ctx) {
		global.Redis.Del(ctx, iter.Val())
	}
}

func (s *WorkOrderService) GetMonthlyWorkerLeaderboard(limit int64) ([]redis.Z, error) {
	ctx := context.Background()
	currentMonth := time.Now().Format("200601")
	cacheKey := fmt.Sprintf("worker_leaderboard:%s", currentMonth)
	return global.Redis.ZRevRangeWithScores(ctx, cacheKey, 0, limit-1).Result()
}
