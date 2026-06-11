package model

import (
	"time"

	"gorm.io/gorm"
)

// RoleEnum defines user roles
type RoleEnum string

const (
	RoleAdmin       RoleEnum = "Admin"
	RoleStudent     RoleEnum = "Student"
	RoleWorker      RoleEnum = "Worker"
	RoleHousemaster RoleEnum = "Housemaster"
)

// WorkOrderStatusEnum defines work order status
type WorkOrderStatusEnum string

const (
	StatusPendingProcessing WorkOrderStatusEnum = "待指派"
	StatusAssigned          WorkOrderStatusEnum = "已指派"
	StatusProcessing        WorkOrderStatusEnum = "维修中"
	StatusCompleted         WorkOrderStatusEnum = "已完工"
	StatusEvaluated         WorkOrderStatusEnum = "已评价"
)

// User model
type User struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Username  string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Password  string         `gorm:"type:varchar(100);not null" json:"-"`
	Role      RoleEnum       `gorm:"type:varchar(20);not null;default:'Student'" json:"role"`
	Phone     string         `gorm:"type:varchar(20)" json:"phone"`
	RealName  string         `gorm:"type:varchar(50)" json:"real_name"`
}

// WorkOrder model
type WorkOrder struct {
	ID        uint                `gorm:"primarykey" json:"id"`
	CreatedAt time.Time           `gorm:"index" json:"created_at"`
	UpdatedAt time.Time           `json:"updated_at"`
	DeletedAt gorm.DeletedAt      `gorm:"index" json:"-"`
	UserID    uint                `gorm:"not null;index" json:"user_id"`
	User      User                `gorm:"foreignKey:UserID" json:"user"`
	WorkerID  *uint               `gorm:"index" json:"worker_id"`
	Worker    *User               `gorm:"foreignKey:WorkerID" json:"worker"`
	Content      string              `gorm:"type:text;not null" json:"content"`
	ContactPhone string              `gorm:"type:varchar(20);not null" json:"contact_phone"`
	ImageURL     string              `gorm:"type:varchar(255)" json:"image_url"`
	Status       WorkOrderStatusEnum `gorm:"type:varchar(20);not null;default:'待指派'" json:"status"`
	Rating       int                 `gorm:"type:int;default:0" json:"rating"` // 0 means not rated, 1-5 for rating
	Location     string              `gorm:"type:varchar(100);not null;default:'宿舍楼A栋'" json:"location"`
}

// Notice model
type Notice struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	UserID    uint           `gorm:"not null;index" json:"user_id"` // Receiver
	Message   string         `gorm:"type:text;not null" json:"message"`
	IsRead    bool           `gorm:"type:boolean;default:false" json:"is_read"`
}

// InspectionOrder model - kept for completeness
type InspectionOrder struct {
	ID          uint           `gorm:"primarykey" json:"id"`
	CreatedAt   time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	InspectorID uint           `gorm:"not null;index" json:"inspector_id"`
	Inspector   User           `gorm:"foreignKey:InspectorID" json:"inspector"`
	Location    string         `gorm:"type:varchar(100);not null" json:"location"`
	Item        string         `gorm:"type:varchar(100);not null" json:"item"`
	Result      string         `gorm:"type:varchar(20);not null" json:"result"`
	Comments    string         `gorm:"type:text" json:"comments"`
}

// Consumable represents a warehouse maintenance material/consumable
type Consumable struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Name      string         `gorm:"type:varchar(100);not null;uniqueIndex" json:"name"`
	Stock     int            `gorm:"type:int;not null;default:0" json:"stock"`
	Unit      string         `gorm:"type:varchar(20);not null;default:'个'" json:"unit"`
}

// WorkOrderConsumable records the consumption of consumables per workorder
type WorkOrderConsumable struct {
	ID           uint       `gorm:"primarykey" json:"id"`
	CreatedAt    time.Time  `json:"created_at"`
	WorkOrderID  uint       `gorm:"not null;index" json:"work_order_id"`
	WorkOrder    WorkOrder  `gorm:"foreignKey:WorkOrderID" json:"-"`
	ConsumableID uint       `gorm:"not null;index" json:"consumable_id"`
	Consumable   Consumable `gorm:"foreignKey:ConsumableID" json:"consumable"`
	Quantity     int        `gorm:"type:int;not null;default:1" json:"quantity"`
}
