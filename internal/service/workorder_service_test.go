package service

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"dorm-repair-system/internal/global"
	"dorm-repair-system/internal/model"
	"dorm-repair-system/internal/repository"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// setupServiceTest 初始化内存 SQLite 数据库和内存 miniredis 实例。
// 返回清理函数以在测试结束后恢复全局变量。
func setupServiceTest(t *testing.T) (*miniredis.Miniredis, func()) {
	// 备份原全局变量
	oldDB := global.DB
	oldRedis := global.Redis

	// 1. 初始化 miniredis 实例
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}

	global.Redis = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	// 2. 初始化内存中运行的 SQLite 数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		mr.Close()
		t.Fatalf("failed to initialize SQLite memory db: %v", err)
	}

	// 自动迁移所有模型表结构
	err = db.AutoMigrate(
		&model.User{},
		&model.WorkOrder{},
		&model.Notice{},
		&model.Consumable{},
		&model.WorkOrderConsumable{},
	)
	if err != nil {
		mr.Close()
		t.Fatalf("failed to migrate models: %v", err)
	}

	global.DB = db

	// 返回测试清理函数
	return mr, func() {
		mr.Close()
		global.DB = oldDB
		global.Redis = oldRedis
	}
}

// TestGrabWorkOrderSuccess 测试单个维修工正常抢单的流程。
func TestGrabWorkOrderSuccess(t *testing.T) {
	_, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewWorkOrderRepository()
	svc := NewWorkOrderService(repo)

	// 创建测试学生和维修工账号
	student := &model.User{Username: "student_test", Role: model.RoleStudent}
	global.DB.Create(student)

	worker := &model.User{Username: "worker_test", Role: model.RoleWorker}
	global.DB.Create(worker)

	// 创建测试工单
	order := &model.WorkOrder{
		UserID:       student.ID,
		Content:      "漏水报修",
		ContactPhone: "13800000000",
		Status:       model.StatusPendingProcessing,
	}
	global.DB.Create(order)

	// 执行抢单操作
	err := svc.GrabWorkOrder(ctx, order.ID, worker.ID)
	if err != nil {
		t.Fatalf("expected grab to succeed, got error: %v", err)
	}

	// 验证数据库状态是否正确变更
	var updatedOrder model.WorkOrder
	err = global.DB.First(&updatedOrder, order.ID).Error
	if err != nil {
		t.Fatalf("failed to fetch updated order: %v", err)
	}

	if updatedOrder.Status != model.StatusAssigned {
		t.Errorf("expected status to be %s, got %s", model.StatusAssigned, updatedOrder.Status)
	}
	if updatedOrder.WorkerID == nil || *updatedOrder.WorkerID != worker.ID {
		t.Errorf("expected worker id to be %d, got %v", worker.ID, updatedOrder.WorkerID)
	}

	// 验证 Redis 中的抢单分布式锁是否已正常释放（Lua 脚本执行完后需释放）
	lockKey := fmt.Sprintf("lock:workorder:%d", order.ID)
	exists, err := global.Redis.Exists(ctx, lockKey).Result()
	if err != nil {
		t.Fatalf("redis error checking lock existence: %v", err)
	}
	if exists > 0 {
		t.Errorf("expected lock key %q to be deleted, but it still exists", lockKey)
	}
}

// TestGrabWorkOrderConcurrency 测试多个维修工并发抢同一张工单的场景。
func TestGrabWorkOrderConcurrency(t *testing.T) {
	_, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewWorkOrderRepository()
	svc := NewWorkOrderService(repo)

	// 创建测试学生账号
	student := &model.User{Username: "student_test", Role: model.RoleStudent}
	global.DB.Create(student)

	// 创建单个待处理（待指派）工单
	order := &model.WorkOrder{
		UserID:       student.ID,
		Content:      "电路故障",
		ContactPhone: "13900000000",
		Status:       model.StatusPendingProcessing,
	}
	global.DB.Create(order)

	// 创建 10 个测试维修工
	numWorkers := 10
	workers := make([]*model.User, numWorkers)
	for i := 0; i < numWorkers; i++ {
		workers[i] = &model.User{
			Username: fmt.Sprintf("worker_%d", i),
			Role:     model.RoleWorker,
		}
		global.DB.Create(workers[i])
	}

	// 并发尝试抢单
	var wg sync.WaitGroup
	successChan := make(chan uint, numWorkers)
	errChan := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID uint) {
			defer wg.Done()
			err := svc.GrabWorkOrder(ctx, order.ID, workerID)
			if err == nil {
				successChan <- workerID
			} else {
				errChan <- err
			}
		}(workers[i].ID)
	}

	// 等待所有抢单协程结束
	wg.Wait()
	close(successChan)
	close(errChan)

	// 从通道收集结果数据
	var successfulWorkerID uint
	successCount := 0
	for id := range successChan {
		successfulWorkerID = id
		successCount++
	}

	failureCount := 0
	for range errChan {
		failureCount++
	}

	// 断言有且仅有一个人抢单成功，其他人全部失败
	if successCount != 1 {
		t.Errorf("expected exactly 1 success, got %d", successCount)
	}
	if failureCount != numWorkers-1 {
		t.Errorf("expected %d failures, got %d", numWorkers-1, failureCount)
	}

	// 从数据库再次验证工单是否正确分配给了胜出的维修工
	var dbOrder model.WorkOrder
	err := global.DB.First(&dbOrder, order.ID).Error
	if err != nil {
		t.Fatalf("failed to query database for order: %v", err)
	}

	if dbOrder.Status != model.StatusAssigned {
		t.Errorf("expected order status to be %s, got %s", model.StatusAssigned, dbOrder.Status)
	}
	if dbOrder.WorkerID == nil || *dbOrder.WorkerID != successfulWorkerID {
		t.Errorf("expected order assigned to worker %d, got %v", successfulWorkerID, dbOrder.WorkerID)
	}

	// 验证 Redis 分布式锁是否被正常删除释放
	lockKey := fmt.Sprintf("lock:workorder:%d", order.ID)
	exists, err := global.Redis.Exists(ctx, lockKey).Result()
	if err != nil {
		t.Fatalf("redis error: %v", err)
	}
	if exists > 0 {
		t.Errorf("expected lock key to be deleted, but it exists")
	}
}

// TestCompleteOrderWithConsumablesSuccess 测试师傅完工并登记消耗耗材的成功流程。
func TestCompleteOrderWithConsumablesSuccess(t *testing.T) {
	_, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewWorkOrderRepository()
	svc := NewWorkOrderService(repo)

	// 1. 创建测试学生与维修工账户
	student := &model.User{Username: "student_c", Role: model.RoleStudent}
	global.DB.Create(student)

	worker := &model.User{Username: "worker_c", Role: model.RoleWorker}
	global.DB.Create(worker)

	// 2. 创建物料库存数据
	c1 := &model.Consumable{Name: "LED日光灯管", Stock: 10, Unit: "根"}
	global.DB.Create(c1)

	c2 := &model.Consumable{Name: "水龙头阀芯", Stock: 5, Unit: "个"}
	global.DB.Create(c2)

	// 3. 创建一张处于“维修中”状态的工单
	order := &model.WorkOrder{
		UserID:       student.ID,
		WorkerID:     &worker.ID,
		Content:      "更换灯管与水龙头",
		ContactPhone: "13512345678",
		Status:       model.StatusProcessing,
	}
	global.DB.Create(order)

	// 4. 调用完工并登记耗材的 Service 接口
	items := []ConsumableUseInput{
		{ConsumableID: c1.ID, Quantity: 2},
		{ConsumableID: c2.ID, Quantity: 3},
	}

	err := svc.CompleteOrderWithConsumables(ctx, order.ID, worker.ID, items)
	if err != nil {
		t.Fatalf("expected complete order to succeed, got error: %v", err)
	}

	// 5. 验证数据库状态是否符合预期变更
	// 验证工单状态更新为已完工
	var updatedOrder model.WorkOrder
	global.DB.First(&updatedOrder, order.ID)
	if updatedOrder.Status != model.StatusCompleted {
		t.Errorf("expected order status to be %s, got %s", model.StatusCompleted, updatedOrder.Status)
	}

	// 验证耗材库存被正确扣减
	var updatedC1, updatedC2 model.Consumable
	global.DB.First(&updatedC1, c1.ID)
	global.DB.First(&updatedC2, c2.ID)

	if updatedC1.Stock != 8 {
		t.Errorf("expected LED stock to be 8, got %d", updatedC1.Stock)
	}
	if updatedC2.Stock != 2 {
		t.Errorf("expected valve stock to be 2, got %d", updatedC2.Stock)
	}

	// 验证工单耗材消耗记录是否正确生成
	var usageRecords []model.WorkOrderConsumable
	global.DB.Where("work_order_id = ?", order.ID).Find(&usageRecords)
	if len(usageRecords) != 2 {
		t.Errorf("expected 2 usage records, got %d", len(usageRecords))
	}
}

// TestCompleteOrderWithConsumablesFailures 测试登记耗材并完工失败时的事务回滚场景。
func TestCompleteOrderWithConsumablesFailures(t *testing.T) {
	_, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()
	repo := repository.NewWorkOrderRepository()
	svc := NewWorkOrderService(repo)

	// 初始化测试账户
	student := &model.User{Username: "student_f", Role: model.RoleStudent}
	global.DB.Create(student)

	workerA := &model.User{Username: "worker_a", Role: model.RoleWorker}
	global.DB.Create(workerA)

	workerB := &model.User{Username: "worker_b", Role: model.RoleWorker}
	global.DB.Create(workerB)

	// 初始化测试耗材
	c1 := &model.Consumable{Name: "LED日光灯管", Stock: 5, Unit: "根"}
	global.DB.Create(c1)

	// 创建一张指派给 workerA 且处于“维修中”的工单
	order := &model.WorkOrder{
		UserID:       student.ID,
		WorkerID:     &workerA.ID,
		Content:      "电路报修",
		ContactPhone: "13512345678",
		Status:       model.StatusProcessing,
	}
	global.DB.Create(order)

	// 场景 1：未指派该工单的维修工 workerB 尝试完成此工单，预期报错拒绝
	items := []ConsumableUseInput{{ConsumableID: c1.ID, Quantity: 1}}
	err := svc.CompleteOrderWithConsumables(ctx, order.ID, workerB.ID, items)
	if err == nil || err.Error() != "you are not assigned to this order" {
		t.Errorf("expected unauthorized error, got %v", err)
	}

	// 验证工单状态与耗材库存未发生变化
	var checkOrder1 model.WorkOrder
	global.DB.First(&checkOrder1, order.ID)
	if checkOrder1.Status != model.StatusProcessing {
		t.Errorf("expected order status to remain %s, got %s", model.StatusProcessing, checkOrder1.Status)
	}
	var checkConsumable1 model.Consumable
	global.DB.First(&checkConsumable1, c1.ID)
	if checkConsumable1.Stock != 5 {
		t.Errorf("expected stock to remain 5, got %d", checkConsumable1.Stock)
	}

	// 场景 2：工单不处于“维修中”状态（如仅处于“已指派”状态），预期报错拒绝
	orderAssigned := &model.WorkOrder{
		UserID:       student.ID,
		WorkerID:     &workerA.ID,
		Content:      "水管报修",
		ContactPhone: "13512345678",
		Status:       model.StatusAssigned,
	}
	global.DB.Create(orderAssigned)

	err = svc.CompleteOrderWithConsumables(ctx, orderAssigned.ID, workerA.ID, items)
	if err == nil || err.Error() != "invalid status transition: must be processing" {
		t.Errorf("expected status transition error, got %v", err)
	}

	// 验证状态未变
	var checkOrder2 model.WorkOrder
	global.DB.First(&checkOrder2, orderAssigned.ID)
	if checkOrder2.Status != model.StatusAssigned {
		t.Errorf("expected order status to remain %s, got %s", model.StatusAssigned, checkOrder2.Status)
	}

	// 场景 3：需要消耗的物料库存不足（请求 6 个，但当前库存仅 5 个），预期报错且事务整体回滚
	itemsInsufficient := []ConsumableUseInput{{ConsumableID: c1.ID, Quantity: 6}}
	err = svc.CompleteOrderWithConsumables(ctx, order.ID, workerA.ID, itemsInsufficient)
	if err == nil || err.Error() != fmt.Sprintf("物料库存不足: %s (当前库存: 5)", c1.Name) {
		t.Errorf("expected stock insufficient error, got %v", err)
	}

	// 验证工单状态和耗材库存均保持不变（事务成功回滚）
	var checkOrder3 model.WorkOrder
	global.DB.First(&checkOrder3, order.ID)
	if checkOrder3.Status != model.StatusProcessing {
		t.Errorf("expected order status to remain %s, got %s", model.StatusProcessing, checkOrder3.Status)
	}
	var checkConsumable3 model.Consumable
	global.DB.First(&checkConsumable3, c1.ID)
	if checkConsumable3.Stock != 5 {
		t.Errorf("expected stock to remain 5, got %d", checkConsumable3.Stock)
	}

	// 场景 4：使用的物料 ID 不存在，预期报错且事务整体回滚
	itemsNonExistent := []ConsumableUseInput{{ConsumableID: 99999, Quantity: 1}}
	err = svc.CompleteOrderWithConsumables(ctx, order.ID, workerA.ID, itemsNonExistent)
	if err == nil || err.Error() != "物料ID不存在" {
		t.Errorf("expected non-existent consumable error, got %v", err)
	}

	// 验证状态未变（回滚成功）
	var checkOrder4 model.WorkOrder
	global.DB.First(&checkOrder4, order.ID)
	if checkOrder4.Status != model.StatusProcessing {
		t.Errorf("expected order status to remain %s, got %s", model.StatusProcessing, checkOrder4.Status)
	}
}
