package repository

import (
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
)

type WorkOrderRepository struct{}

func NewWorkOrderRepository() *WorkOrderRepository {
	return &WorkOrderRepository{}
}

func (r *WorkOrderRepository) CreateOrder(order *model.WorkOrder) error {
	return global.DB.Create(order).Error
}

func (r *WorkOrderRepository) UpdateOrder(order *model.WorkOrder) error {
	return global.DB.Save(order).Error
}

func (r *WorkOrderRepository) GetOrderByID(id uint) (*model.WorkOrder, error) {
	var order model.WorkOrder
	err := global.DB.Preload("User").Preload("Worker").First(&order, id).Error
	return &order, err
}

func (r *WorkOrderRepository) ListOrders(offset, limit int, userID, workerID *uint, status string) ([]model.WorkOrder, int64, error) {
	var orders []model.WorkOrder
	var total int64
	
	query := global.DB.Model(&model.WorkOrder{})
	
	if userID != nil {
		query = query.Where("user_id = ?", *userID)
	}
	if workerID != nil {
		query = query.Where("worker_id = ?", *workerID)
	}
	if status != "" {
		query = query.Where("status = ?", status)
	}

	err := query.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	err = query.Preload("User").Preload("Worker").Offset(offset).Limit(limit).Order("created_at desc").Find(&orders).Error
	return orders, total, err
}
