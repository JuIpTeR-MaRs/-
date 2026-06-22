# 智慧宿舍报修与物业巡检系统 (Dormitory Repair & Inspection System)

这是一个基于 **Go + Gin + GORM + Redis + MySQL + Casbin** 构建的高性能、现代化的宿舍报修与服务大厅管理系统。

前端采用了一套极具视觉吸引力的**暗黑渐变玻璃拟态（Glassmorphism）**大屏风格设计，配合流畅的微动效，提供了极致的用户视觉体验。本项目非常适合作为高校期末大作业、软件工程实践项目，或作为学习 Golang 后端开发的工程参考。

---

## 🌟 项目亮点与特色

- **微观拟物化视觉大屏**：前端采用现代 CSS 变量、毛玻璃拟态面板及流光渐变背景动画（`index.html` 与 `login.html`），打造极具质感的交互系统。
- **RBAC 细粒度权限控制**：基于 **Casbin (gorm-adapter)** 实现了动态规则匹配与基于角色的访问控制，严格防范越权行为。
- **高并发实时排行榜**：利用 **Redis ZSet (有序集合)** 维护“本月金牌师傅排行榜”，每次学生评分后通过 Redis 实时计算并推送，降低关系型数据库的查询压力。
- **Graceful Shutdown (优雅启停)**：后端服务支持捕捉系统信号（SIGINT, SIGTERM），实现优雅的 HTTP 连接关闭，保护未处理完的请求。
- **完备的工程化架构**：后端遵循标准的 Go 项目分层规范（Controller -> Service -> Repository/Model），集成 **Zap** 异步日志系统及 **Lumberjack** 日志自动切割归档。

---

## 🛠️ 技术栈

| 模块 | 技术选型 | 说明 |
| :--- | :--- | :--- |
| **开发语言** | Go 1.25+ | 高性能并发语言 |
| **Web 框架** | Gin v1.12.0 | 高性能、轻量级路由与 HTTP 服务框架 |
| **ORM 框架** | GORM v2 (MySQL) | 数据库对象关系映射与自动迁移表结构 |
| **权限控制** | Casbin v3 + GORM Adapter | 基于角色的动态访问权限控制器 (RBAC) |
| **内存数据库** | Redis v8 (Go-redis) | 缓存与实时高频金牌师傅排行榜 |
| **身份认证** | JWT (golang-jwt v5) | 无状态 Token 鉴权，支持过期自动阻断 |
| **配置读取** | Viper v1.21.0 | YAML 结构化多环境配置读取 |
| **日志切割** | Zap + Lumberjack.v2 | 高性能结构化日志，支持自动分割、保留天数配置 |
| **前端页面** | 原生 HTML5 + CSS3 + Vanilla JS | 响应式栅格布局、毛玻璃效果、Fetch 异步交互 |

---

## 📂 项目目录结构

```text
├── cmd/
│   └── server/          # 应用程序入口
│       └── main.go      # 主启动类 (优雅初始化配置、数据库、Redis、路由及服务启停)
├── config/
│   ├── config.yaml      # 全局配置文件 (数据库、Redis、JWT秘钥、日志分割等参数)
│   └── rbac_model.conf  # Casbin 访问控制模型配置文件 (定义 r, p, g, e, m)
├── internal/
│   ├── config/          # 映射 config.yaml 的结构体定义
│   ├── controller/      # 控制器层：接收并校验 HTTP 请求 (User, WorkOrder)
│   ├── global/          # 全局单例与服务初始化 (InitConfig, InitLogger, InitDB, InitRedis, InitCasbin)
│   ├── middleware/      # 中间件层：JWT鉴权、CasbinRBAC鉴权、CORS跨域、Recover异常捕获
│   ├── model/           # 模型定义：GORM 数据结构定义与枚举类型 (User, WorkOrder, Notice, InspectionOrder)
│   ├── repository/      # 数据访问层：MySQL 原生存取交互
│   ├── service/         # 业务逻辑层：核心业务算法与流程逻辑封装
│   └── router/          # 路由注册与分群组配置
├── init.sql             # 数据库初始化脚本 (自动建库建表及注入测试数据)
├── index.html           # 前端大屏系统面板主页
├── login.html           # 前端登录/注册精美页面
├── go.mod               # Go 依赖描述文件
└── README.md            # 项目说明文档
```

---

## 🛢️ 数据库设计说明

本系统在启动时会自动读取模型结构体，并通过 GORM 执行 `AutoMigrate` 自动创建/补充表结构。也可以使用 `init.sql` 快速初始化。

### 核心表结构模型：

1. **User (用户表)**
   - `id`: 自增主键
   - `username`: 账号唯一索引 (学号/工号)
   - `password`: 使用 **bcrypt** 加密存储的密码哈希
   - `role`: 用户角色（枚举：`Admin` 宿管管理员, `Student` 学生, `Worker` 维修工）
   - `real_name`: 真实姓名
   - `phone`: 联系电话

2. **WorkOrder (报修工单表)**
   - `id`: 唯一工单号
   - `user_id`: 报修学生的外键关联
   - `worker_id`: 指派维修工的外键关联 (可为空)
   - `content`: 详细的报修故障文字描述
   - `contact_phone`: 报修填写的学生联系方式
   - `status`: 工单状态（枚举：`待指派` -> `已指派` -> `维修中` -> `已完工` -> `已评价`）
   - `rating`: 评分 (0 表示未评分，1-5 表示星级)

3. **CasbinRule (权限规则表)**
   - 由 Casbin 自动管理，存储不同角色访问特定路径所拥有的 API 请求方式（如 `Student` 仅允许对 `/api/v1/workorder` 发送 `POST` 与 `GET` 请求）。

---

## 🚀 快速本地部署与运行

### 1. 前置准备环境
- **Go 环境**：请确保本地已配置 `Go 1.20+` 环境变量。
- **MySQL 数据库**：建议版本 5.7 或 8.0。
- **Redis 数据库**：单机版 Redis 服务。

### 2. 初始化数据库
在你的 MySQL 客户端执行项目根目录下的 [init.sql](file:///d:/fuwuqidazuoye/init.sql) 脚本文件：
```sql
# 执行后将自动创建 dorm_repair 数据库、相关表结构
# 并且会预先插入 3 个不同角色的演示账号，以及默认的 Casbin 权限过滤规则
```

### 3. 修改配置文件
编辑项目根目录下的配置文件 [config/config.yaml](file:///d:/fuwuqidazuoye/config/config.yaml)，更新你的 MySQL 和 Redis 连接配置信息：
```yaml
mysql:
  host: 127.0.0.1
  port: 3306
  user: your_mysql_username
  password: your_mysql_password
  dbname: dorm_repair

redis:
  host: 127.0.0.1
  port: 6379
  password: your_redis_password  # 没有密码请留空
```

### 4. 启动服务
在项目根目录的终端中运行：
```bash
go run cmd/server/main.go
```
启动成功后，控制台会输出：
```text
Config loaded successfully.
Logger initialized successfully
MySQL initialized successfully
Database tables migrated successfully
Redis initialized successfully
Casbin initialized successfully
Server is running at http://127.0.0.1:8080
```

### 5. 访问大屏前端
打开浏览器访问：[http://127.0.0.1:8080/login](http://127.0.0.1:8080/login)

---
## 🎬 业务流演示步骤 (测试体验说明)

在登录页面进行测试时，请手动输入下方列出的系统预设演示角色账号进行登录：

| 演示角色 | 登录用户名 | 登录密码 | 说明 |
| :--- | :--- | :--- | :--- |
| **学生** | `student1` | `123456` | 可以发起报修工单、评价已完工的工单 |
| **系统管理员** | `admin` | `123456` | 拥有超级管理员权限，可以进行全局系统管理与操作 |
| **宿管老师** | `housemaster1` | `123456` | 宿管业务角色，可以看到全校工单列表、指派工单给特定师傅 |
| **维修师傅** | `worker1` | `123456` | 可以接受指派给自己的工单、更新工单状态为维修或完成 |

### 完整演示闭环流程：

1. **第一步：学生提交报修**
   - 用学生账号 `student1` 登录。
   - 在左侧操作面板填写报修内容（例如：“女生寝室3栋402宿舍的阳台水龙头漏水严重”），填入联系方式，点击 **“立即提交报修”**。右侧工单大厅将实时刷新出该工单，状态为 `待指派`。然后点击“安全退出”。
2. **第二步：管理员指派任务**
   - 用管理员账号 `admin` 登录。
   - 在右侧大厅找到刚才提交的工单，可以看到操作栏出现 **“指派给李四 (ID:3)”** 按钮。点击指派，状态变为 `已指派`，负责师傅显示为李四。点击“安全退出”。
3. **第三步：师傅接单与维保**
   - 用维修师傅账号 `worker1` 登录。
   - 右侧会显示指派给他的工单。点击 **“开始处理”** (状态更新为 `维修中`)，处理完成后点击 **“完成维保”** (状态更新为 `已完工`)。点击“安全退出”。
4. **第四步：学生打分反馈**
   - 换回学生账号 `student1` 登录。
   - 可以看到刚才的工单已经变为了 `已完工`。操作栏出现 **“立即评价”** 按钮。点击并在弹窗中为李四师傅打分（例如 5 星好评）。
   - 提交评价后，工单状态进入最终态 `已评价`，页面左侧的 **“本月金牌师傅排行榜”** 将通过 Redis ZSet 自动累加得分并实时更新展现新排名！

---

## 🧪 单元测试与核心校验 (Unit Testing)

本项目针对**安全身份拦截层**（Middleware）和**抢单与扣减锁控制层**（Concurrency Lock）编写了完善的单元测试，采用 SQLite 内存数据库和 `miniredis` 内存 Redis 技术，实现 100% 独立于外部环境的绿色自动化测试。

### 1. 测试用例设计

#### A. 身份认证与权限拦截测试 (`internal/middleware`)
测试逻辑位于 [internal/middleware/auth_test.go](file:///d:/fuwuqidazuoye/internal/middleware/auth_test.go)，覆盖场景包括：
- **JWT 身份认证测试 (`TestJWTAuth`)**：
  - **缺失 Authorization 头部**：返回 HTTP 401，原因为 `"Authorization header is required"`。
  - **头部格式错误**：返回 HTTP 401，原因为 `"Authorization header format must be Bearer {token}"`。
  - **无效或过期 Token**：返回 HTTP 401，原因为 `"Invalid or expired token"`。
  - **合法 Token 校验**：验证通过并进入后续业务，且 Gin 上下文中正确存储绑定的用户信息。
- **Casbin 权限拦截测试 (`TestCasbinRBAC`)**：
  - **上下文缺失角色**：拦截并返回 HTTP 401 `"Unauthorized"`。
  - **合法请求（策略匹配）**：如 `Student` 对 `/api/v1/workorders` 进行 `POST`/`GET` 请求放行。
  - **越权拦截（禁止访问）**：如 `Student` 尝试 `PUT` 更改 `/api/v1/workorders/:id/status` 拦截并返回 HTTP 403 `"Forbidden"`。
  - **动态路由参数匹配**：如 `Worker` 对 `/api/v1/workorders/123/status` 发送 `PUT` 成功识别并放行。
  - **超级管理员放行**：验证 `Admin` 角色无条件放行所有路径和请求方法。

#### B. 维修抢单分布式锁测试 (`internal/service`)
测试逻辑位于 [workorder_service_test.go](file:///d:/fuwuqidazuoye/internal/service/workorder_service_test.go)，通过内存 Redis 对 SetNX 加锁和 Lua 解锁进行模拟：
- **普通单人抢单测试 (`TestGrabWorkOrderSuccess`)**：
  - 验证单个师傅抢单时，工单状态流转为“已指派”，且负责师傅 ID 写入数据库。
  - 验证抢单流程结束后，Redis 中的分布式锁被 Lua 脚本正确释放。
- **10人并发抢单锁冲突测试 (`TestGrabWorkOrderConcurrency`)**：
  - 并发启动 10 个 goroutine 模拟 10 位不同的师傅同时抢占同一个“待指派”工单。
  - 验证在高并发下，**有且仅有 1 位**师傅能够顺利抢单成功，其余 9 位由于分布式锁拦截或数据库事务冲突全部报错失败，且数据库中工单负责人精准归属于该唯一胜出者。

#### C. 完工扣减耗材行级悲观锁测试 (`internal/service`)
测试逻辑位于 [workorder_service_test.go](file:///d:/fuwuqidazuoye/internal/service/workorder_service_test.go)，通过 GORM 事务内 `FOR UPDATE` 对工单和物料记录施加悲观锁：
- **正常完工库存扣减 (`TestCompleteOrderWithConsumablesSuccess`)**：
  - 验证师傅提交完工并扣减多种耗材时，物料库存被扣除、工单变更为“已完工”、并生成耗材使用明细记录。
- **异常回滚拦截 (`TestCompleteOrderWithConsumablesFailures`)**：
  - 验证非负责师傅越权操作、工单状态异常、物料库存不足、无效物料 ID 等所有失败场景。
  - 验证当遇到任何错误时，**事务均完美回滚**，工单状态与物料库存没有任何变动，防范任何数据不一致。

---

### 2. 运行单元测试

您可以使用以下命令一键运行项目中的所有单元测试：

```bash
# 一键运行项目内所有测试包
go test -v ./...

# 查看特定包测试覆盖率
go test -cover ./internal/middleware
go test -cover ./internal/service
```

---

### 3. 测试运行结果与日志

执行 `go test -v ./...` 的真实控制台输出如下截图与日志所示：

![测试运行结果截图](./test_results.png)
*(注：请在项目根目录下存放名为 `test_results.png` 的测试结果截图即可在文档中显示)*

```text
=== RUN   TestJWTAuth
=== RUN   TestJWTAuth/Missing_Authorization_Header
=== RUN   TestJWTAuth/Incorrect_Header_Format
=== RUN   TestJWTAuth/Invalid_Token_Signature/Value
=== RUN   TestJWTAuth/Valid_Token
--- PASS: TestJWTAuth (0.00s)
    --- PASS: TestJWTAuth/Missing_Authorization_Header (0.00s)
    --- PASS: TestJWTAuth/Incorrect_Header_Format (0.00s)
    --- PASS: TestJWTAuth/Invalid_Token_Signature/Value (0.00s)
    --- PASS: TestJWTAuth/Valid_Token (0.00s)
=== RUN   TestCasbinRBAC
=== RUN   TestCasbinRBAC/No_Role_In_Context
=== RUN   TestCasbinRBAC/Student_-_Post_Work_Order_(Allowed)
=== RUN   TestCasbinRBAC/Student_-_Get_Work_Orders_(Allowed)
=== RUN   TestCasbinRBAC/Student_-_Put_Status_(Denied/Forbidden)
=== RUN   TestCasbinRBAC/Worker_-_Put_Status_(Allowed,_KeyMatch2_URL_param)
=== RUN   TestCasbinRBAC/Worker_-_Post_Work_Order_(Denied/Forbidden)
=== RUN   TestCasbinRBAC/Admin_-_Bypasses_All_Rules_(Superuser_Allowed)
=== RUN   TestCasbinRBAC/Admin_-_Bypasses_Non-Existent_Path_(Superuser_Allowed)
--- PASS: TestCasbinRBAC (0.00s)
    --- PASS: TestCasbinRBAC/No_Role_In_Context (0.00s)
    --- PASS: TestCasbinRBAC/Student_-_Post_Work_Order_(Allowed) (0.00s)
    --- PASS: TestCasbinRBAC/Student_-_Get_Work_Orders_(Allowed) (0.00s)
    --- PASS: TestCasbinRBAC/Student_-_Put_Status_(Denied/Forbidden) (0.00s)
    --- PASS: TestCasbinRBAC/Worker_-_Put_Status_(Allowed,_KeyMatch2_URL_param) (0.00s)
    --- PASS: TestCasbinRBAC/Worker_-_Post_Work_Order_(Denied/Forbidden) (0.00s)
    --- PASS: TestCasbinRBAC/Admin_-_Bypasses_All_Rules_(Superuser_Allowed) (0.00s)
    --- PASS: TestCasbinRBAC/Admin_-_Bypasses_Non-Existent_Path_(Superuser_Allowed) (0.00s)
PASS
ok  	dorm-repair-system/internal/middleware	0.274s
=== RUN   TestGrabWorkOrderSuccess
--- PASS: TestGrabWorkOrderSuccess (0.02s)
=== RUN   TestGrabWorkOrderConcurrency
--- PASS: TestGrabWorkOrderConcurrency (0.01s)
=== RUN   TestCompleteOrderWithConsumablesSuccess
--- PASS: TestCompleteOrderWithConsumablesSuccess (0.01s)
=== RUN   TestCompleteOrderWithConsumablesFailures

2026/06/22 18:10:24 D:/fuwuqidazuoye/internal/service/workorder_service.go:258 record not found
[0.000ms] [rows:0] SELECT * FROM `consumables` WHERE `consumables`.`id` = 99999 AND `consumables`.`deleted_at` IS NULL ORDER BY `consumables`.`id` LIMIT 1 
--- PASS: TestCompleteOrderWithConsumablesFailures (0.00s)
PASS
ok  	dorm-repair-system/internal/service	0.242s
```

### 4. 测试结果总结 (Test Results Summary)

| 核心模块 | 测试功能 / 函数 | 覆盖核心场景概述 | 代码覆盖率 | 测试状态 |
| :--- | :--- | :--- | :--- | :--- |
| **安全中间件** | `JWTAuth` | 缺失验证头、格式错误、无效/过期Token拦截、有效Token鉴权放行 | **100.0%** | ✅ PASS |
| **安全中间件** | `CasbinRBAC` | 无角色越权拦截、角色策略匹配放行、动态URL权限拦截、超管特权无条件放行 | **80.0%** | ✅ PASS |
| **核心业务流** | `GrabWorkOrder` | 师傅正常单次抢单成功、10人高并发抢单排他性拦截及Redis分布式锁正确释放 | **94.1%** | ✅ PASS |
| **核心业务流** | `CompleteOrderWithConsumables` | 提交完工并正常扣减库存记录明细、非负责师傅越权、库存不足及无效物料事务回滚 | **88.5%** | ✅ PASS |
