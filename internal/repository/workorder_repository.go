package repository

import (
	"context"
	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
	pkgtx "dorm-repair-system/pkg/tx"

	"gorm.io/gorm"
)

type LocationStat struct {
	Location   string `json:"location"`
	TotalCount int64  `json:"total_count"`
}

type WorkerEfficiencyStat struct {
	WorkerID   uint    `json:"worker_id"`
	RealName   string  `json:"real_name"`
	AvgMinutes float64 `json:"avg_minutes"`
}

type IWorkOrderRepository interface {
	CreateOrder(ctx context.Context, order *model.WorkOrder) error
	UpdateOrder(ctx context.Context, order *model.WorkOrder) error
	GetOrderByID(ctx context.Context, id uint) (*model.WorkOrder, error)
	ListOrders(ctx context.Context, offset, limit int, userID, workerID *uint, status string) ([]model.WorkOrder, int64, error)
	CreateNotice(ctx context.Context, notice *model.Notice) error

	// New Analytics & Consumable Access Methods
	GetLocationReportStats(ctx context.Context) ([]LocationStat, error)
	GetWorkerEfficiencyStats(ctx context.Context) ([]WorkerEfficiencyStat, error)
	GetConsumableByID(ctx context.Context, id uint) (*model.Consumable, error)
	UpdateConsumable(ctx context.Context, item *model.Consumable) error
	CreateWorkOrderConsumable(ctx context.Context, woc *model.WorkOrderConsumable) error
}

type WorkOrderRepository struct{}

func NewWorkOrderRepository() IWorkOrderRepository {
	return &WorkOrderRepository{}
}

func (r *WorkOrderRepository) getDB(ctx context.Context) *gorm.DB {
	if db, ok := pkgtx.FromContext(ctx); ok {
		return db
	}
	return global.DB.WithContext(ctx)
}

func (r *WorkOrderRepository) CreateOrder(ctx context.Context, order *model.WorkOrder) error {
	return r.getDB(ctx).Create(order).Error
}

func (r *WorkOrderRepository) UpdateOrder(ctx context.Context, order *model.WorkOrder) error {
	return r.getDB(ctx).Save(order).Error
}

func (r *WorkOrderRepository) GetOrderByID(ctx context.Context, id uint) (*model.WorkOrder, error) {
	var order model.WorkOrder
	err := r.getDB(ctx).Preload("User").Preload("Worker").First(&order, id).Error
	return &order, err
}

func (r *WorkOrderRepository) ListOrders(ctx context.Context, offset, limit int, userID, workerID *uint, status string) ([]model.WorkOrder, int64, error) {
	var orders []model.WorkOrder
	var total int64
	
	db := r.getDB(ctx)
	query := db.Model(&model.WorkOrder{})
	
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

func (r *WorkOrderRepository) CreateNotice(ctx context.Context, notice *model.Notice) error {
	return r.getDB(ctx).Create(notice).Error
}

func (r *WorkOrderRepository) GetLocationReportStats(ctx context.Context) ([]LocationStat, error) {
	var stats []LocationStat
	err := r.getDB(ctx).Model(&model.WorkOrder{}).
		Select("location, count(id) as total_count").
		Group("location").
		Order("total_count desc").
		Scan(&stats).Error
	return stats, err
}

func (r *WorkOrderRepository) GetWorkerEfficiencyStats(ctx context.Context) ([]WorkerEfficiencyStat, error) {
	var results []WorkerEfficiencyStat
	// Select average time between creation (Assigned status created time or order created time) and update (completed time)
	// Using GORM table builder joining with users to aggregate avg repair time in minutes
	err := r.getDB(ctx).Table("work_orders").
		Select("work_orders.worker_id, users.real_name, AVG(TIMESTAMPDIFF(MINUTE, work_orders.created_at, work_orders.updated_at)) as avg_minutes").
		Joins("JOIN users ON users.id = work_orders.worker_id").
		Where("work_orders.status IN (?)", []string{string(model.StatusCompleted), string(model.StatusEvaluated)}).
		Group("work_orders.worker_id, users.real_name").
		Order("avg_minutes asc").
		Scan(&results).Error
	return results, err
}

func (r *WorkOrderRepository) GetConsumableByID(ctx context.Context, id uint) (*model.Consumable, error) {
	var item model.Consumable
	err := r.getDB(ctx).First(&item, id).Error
	return &item, err
}

func (r *WorkOrderRepository) UpdateConsumable(ctx context.Context, item *model.Consumable) error {
	return r.getDB(ctx).Save(item).Error
}

func (r *WorkOrderRepository) CreateWorkOrderConsumable(ctx context.Context, woc *model.WorkOrderConsumable) error {
	return r.getDB(ctx).Create(woc).Error
}
