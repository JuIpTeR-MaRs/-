-- 创建数据库并设置正确的字符集
CREATE DATABASE IF NOT EXISTS dorm_repair DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE dorm_repair;

-- 创建核心的 User 表
CREATE TABLE IF NOT EXISTS `users` (
  `id` bigint unsigned AUTO_INCREMENT,
  `created_at` datetime(3) NULL,
  `updated_at` datetime(3) NULL,
  `deleted_at` datetime(3) NULL,
  `username` varchar(50) NOT NULL,
  `password` varchar(100) NOT NULL,
  `role` varchar(20) NOT NULL DEFAULT 'Student',
  `phone` varchar(20) DEFAULT NULL,
  `real_name` varchar(50) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_users_username` (`username`),
  KEY `idx_users_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 创建 Casbin 的权限控制表
CREATE TABLE IF NOT EXISTS `casbin_rule` (
  `id` bigint unsigned AUTO_INCREMENT,
  `ptype` varchar(100) DEFAULT NULL,
  `v0` varchar(100) DEFAULT NULL,
  `v1` varchar(100) DEFAULT NULL,
  `v2` varchar(100) DEFAULT NULL,
  `v3` varchar(100) DEFAULT NULL,
  `v4` varchar(100) DEFAULT NULL,
  `v5` varchar(100) DEFAULT NULL,
  `v6` varchar(25) DEFAULT NULL,
  `v7` varchar(25) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `idx_casbin_rule` (`ptype`,`v0`,`v1`,`v2`,`v3`,`v4`,`v5`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 插入一条测试管理员账号 (密码是 "123456" 的 bcrypt 加密结果)
-- 由于密码是强哈希的，所以直接插库的话，你需要用下方我为你预先算好的哈希值
INSERT IGNORE INTO `users` (`created_at`, `updated_at`, `username`, `password`, `role`, `phone`, `real_name`) VALUES 
(NOW(), NOW(), 'admin', '$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O', 'Admin', '13800138000', '超级宿管王师傅'),
(NOW(), NOW(), 'student1', '$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O', 'Student', '13900139000', '张三同学'),
(NOW(), NOW(), 'worker1', '$2a$10$wO0X54261iO7bI2uL4gE8u.O9nU9G5/t00iRIf9j.UfR0X07Y.y0O', 'Worker', '13700137000', '李四维修工');

-- 插入一条 Casbin 的权限测试数据（如果没有自动生成的话）
-- 这表明 admin 可以访问任何路径，student 和 worker 会根据中间件代码走逻辑
INSERT IGNORE INTO `casbin_rule` (`ptype`, `v0`, `v1`, `v2`, `v3`, `v4`, `v5`) VALUES 
('p', 'Student', '/api/v1/workorder', 'POST', '', '', ''),
('p', 'Student', '/api/v1/workorder', 'GET', '', '', ''),
('p', 'Worker', '/api/v1/workorder', 'GET', '', '', ''),
('p', 'Admin', '/*', '.*', '', '', '');
