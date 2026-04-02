-- 项目管理系统数据库初始化脚本
-- MySQL 8.0+

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- 创建数据库
CREATE DATABASE IF NOT EXISTS `project_management` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `project_management`;

-- 1. user (用户表)
DROP TABLE IF EXISTS `user`;
CREATE TABLE `user` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `username` VARCHAR(50) NOT NULL COMMENT '用户名',
  `email` VARCHAR(100) NOT NULL COMMENT '邮箱',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '密码哈希',
  `real_name` VARCHAR(50) DEFAULT NULL COMMENT '真实姓名',
  `phone` VARCHAR(20) DEFAULT NULL COMMENT '手机号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 2=inactive, 3=locked',
  `last_login_at` DATETIME DEFAULT NULL COMMENT '最后登录时间',
  `last_login_ip` VARCHAR(45) DEFAULT NULL COMMENT '最后登录 IP',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  UNIQUE KEY `uk_email` (`email`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';

-- 2. role (角色表)
DROP TABLE IF EXISTS `role`;
CREATE TABLE `role` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `name` VARCHAR(50) NOT NULL COMMENT '角色名称',
  `code` VARCHAR(50) NOT NULL COMMENT '角色代码',
  `description` VARCHAR(255) DEFAULT NULL COMMENT '角色描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：0=archived/disabled, 1=active',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色表';

-- 3. permission (权限表)
DROP TABLE IF EXISTS `permission`;
CREATE TABLE `permission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `name` VARCHAR(50) NOT NULL COMMENT '权限名称',
  `code` VARCHAR(100) NOT NULL COMMENT '权限代码',
  `type` TINYINT NOT NULL COMMENT '类型：1=menu, 2=action, 3=data',
  `resource` VARCHAR(255) DEFAULT NULL COMMENT '资源标识',
  `description` VARCHAR(255) DEFAULT NULL COMMENT '权限描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 2=archived',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_type` (`type`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='权限表';

-- 4. user_role (用户角色关联表)
DROP TABLE IF EXISTS `user_role`;
CREATE TABLE `user_role` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role` (`user_id`, `role_id`),
  KEY `idx_role_id` (`role_id`),
  CONSTRAINT `fk_user_role_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_user_role_role` FOREIGN KEY (`role_id`) REFERENCES `role` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户角色关联表';

-- 5. role_permission (角色权限关联表)
DROP TABLE IF EXISTS `role_permission`;
CREATE TABLE `role_permission` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色 ID',
  `permission_id` BIGINT UNSIGNED NOT NULL COMMENT '权限 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_perm` (`role_id`, `permission_id`),
  KEY `idx_permission_id` (`permission_id`),
  CONSTRAINT `fk_role_perm_role` FOREIGN KEY (`role_id`) REFERENCES `role` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_perm_perm` FOREIGN KEY (`permission_id`) REFERENCES `permission` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色权限关联表';

-- 6. project_template (项目模板表)
DROP TABLE IF EXISTS `project_template`;
CREATE TABLE `project_template` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `name` VARCHAR(100) NOT NULL COMMENT '模板名称',
  `code` VARCHAR(50) NOT NULL COMMENT '模板代码',
  `version` VARCHAR(20) NOT NULL DEFAULT 'v1' COMMENT '版本号',
  `type` VARCHAR(50) NOT NULL COMMENT '项目类型：software, construction, consulting, audit, government, marketing',
  `description` TEXT DEFAULT NULL COMMENT '模板描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 2=archived',
  `created_by` BIGINT UNSIGNED DEFAULT NULL COMMENT '创建人 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_code_version` (`code`, `version`),
  KEY `idx_type` (`type`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目模板表';

-- 7. template_stage (模板阶段表)
DROP TABLE IF EXISTS `template_stage`;
CREATE TABLE `template_stage` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `template_id` BIGINT UNSIGNED NOT NULL COMMENT '模板 ID',
  `name` VARCHAR(100) NOT NULL COMMENT '阶段名称',
  `code` VARCHAR(50) NOT NULL COMMENT '阶段代码',
  `order_num` INT NOT NULL DEFAULT 0 COMMENT '排序号',
  `description` TEXT DEFAULT NULL COMMENT '阶段描述',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_template_id` (`template_id`),
  KEY `idx_order_num` (`order_num`),
  CONSTRAINT `fk_template_stage_template` FOREIGN KEY (`template_id`) REFERENCES `project_template` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='模板阶段表';

-- 7b. menu (菜单表)
DROP TABLE IF EXISTS `menu`;
CREATE TABLE `menu` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `name` VARCHAR(50) NOT NULL COMMENT '菜单名称',
  `code` VARCHAR(50) NOT NULL COMMENT '菜单代码',
  `icon` VARCHAR(50) DEFAULT 'fa-circle' COMMENT '图标类名',
  `url` VARCHAR(100) DEFAULT '#' COMMENT '菜单 URL',
  `parent_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '父菜单 ID',
  `order_num` INT NOT NULL DEFAULT 0 COMMENT '排序号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 0=inactive',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_parent` (`parent_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='菜单表';

-- 7c. system_function (系统功能表)
DROP TABLE IF EXISTS `system_function`;
CREATE TABLE `system_function` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `menu_id` BIGINT UNSIGNED NOT NULL COMMENT '所属菜单 ID',
  `name` VARCHAR(50) NOT NULL COMMENT '功能名称',
  `code` VARCHAR(50) NOT NULL COMMENT '功能代码',
  `url` VARCHAR(100) DEFAULT '' COMMENT 'API URL',
  `method` VARCHAR(10) DEFAULT 'GET' COMMENT 'HTTP 方法',
  `description` VARCHAR(200) DEFAULT '' COMMENT '功能描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 0=inactive',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_menu` (`menu_id`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统功能表';

-- 7d. role_menu (角色菜单权限表)
DROP TABLE IF EXISTS `role_menu`;
CREATE TABLE `role_menu` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色 ID',
  `menu_id` BIGINT UNSIGNED NOT NULL COMMENT '菜单 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_menu` (`role_id`, `menu_id`),
  KEY `idx_menu_id` (`menu_id`),
  CONSTRAINT `fk_role_menu_role` FOREIGN KEY (`role_id`) REFERENCES `role` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_menu_menu` FOREIGN KEY (`menu_id`) REFERENCES `menu` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色菜单权限表';

-- 7e. role_function (角色功能权限表)
DROP TABLE IF EXISTS `role_function`;
CREATE TABLE `role_function` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色 ID',
  `function_id` BIGINT UNSIGNED NOT NULL COMMENT '功能 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_function` (`role_id`, `function_id`),
  KEY `idx_function_id` (`function_id`),
  CONSTRAINT `fk_role_function_role` FOREIGN KEY (`role_id`) REFERENCES `role` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_role_function_function` FOREIGN KEY (`function_id`) REFERENCES `system_function` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='角色功能权限表';

-- 8. project (项目表)
DROP TABLE IF EXISTS `project`;
CREATE TABLE `project` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `name` VARCHAR(200) NOT NULL COMMENT '项目名称',
  `code` VARCHAR(50) NOT NULL COMMENT '项目编码',
  `template_id` BIGINT UNSIGNED NOT NULL COMMENT '使用的模板 ID',
  `manager_id` BIGINT UNSIGNED NOT NULL COMMENT '项目经理 ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=planning, 2=in_progress, 3=completed, 4=archived',
  `start_date` DATE DEFAULT NULL COMMENT '计划开始日期',
  `end_date` DATE DEFAULT NULL COMMENT '计划结束日期',
  `actual_start_date` DATE DEFAULT NULL COMMENT '实际开始日期',
  `actual_end_date` DATE DEFAULT NULL COMMENT '实际结束日期',
  `progress` INT NOT NULL DEFAULT 0 COMMENT '整体进度百分比 0-100',
  `description` TEXT DEFAULT NULL COMMENT '项目描述',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_code` (`code`),
  KEY `idx_template_id` (`template_id`),
  KEY `idx_manager_id` (`manager_id`),
  KEY `idx_status` (`status`),
  CONSTRAINT `fk_project_template` FOREIGN KEY (`template_id`) REFERENCES `project_template` (`id`),
  CONSTRAINT `fk_project_manager` FOREIGN KEY (`manager_id`) REFERENCES `user` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目表';

-- 9. project_member (项目成员表)
DROP TABLE IF EXISTS `project_member`;
CREATE TABLE `project_member` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
  `role` VARCHAR(50) DEFAULT 'member' COMMENT '项目内角色',
  `joined_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '加入时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_user` (`project_id`, `user_id`),
  KEY `idx_user_id` (`user_id`),
  CONSTRAINT `fk_project_member_project` FOREIGN KEY (`project_id`) REFERENCES `project` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_project_member_user` FOREIGN KEY (`user_id`) REFERENCES `user` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目成员表';

-- 10. project_stage (项目阶段表)
DROP TABLE IF EXISTS `project_stage`;
CREATE TABLE `project_stage` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目 ID',
  `template_stage_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '模板阶段 ID',
  `name` VARCHAR(100) NOT NULL COMMENT '阶段名称',
  `code` VARCHAR(50) NOT NULL COMMENT '阶段代码',
  `order_num` INT NOT NULL DEFAULT 0 COMMENT '排序号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=pending, 2=in_progress, 3=completed',
  `plan_start_date` DATE DEFAULT NULL COMMENT '计划开始日期',
  `plan_end_date` DATE DEFAULT NULL COMMENT '计划结束日期',
  `actual_start_date` DATE DEFAULT NULL COMMENT '实际开始日期',
  `actual_end_date` DATE DEFAULT NULL COMMENT '实际结束日期',
  `progress` INT NOT NULL DEFAULT 0 COMMENT '进度百分比 0-100',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_status` (`status`),
  CONSTRAINT `fk_project_stage_project` FOREIGN KEY (`project_id`) REFERENCES `project` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目阶段表';

-- 11. document (文档表)
DROP TABLE IF EXISTS `document`;
CREATE TABLE `document` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `project_id` BIGINT UNSIGNED NOT NULL COMMENT '项目 ID',
  `stage_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '阶段 ID',
  `name` VARCHAR(200) NOT NULL COMMENT '文档名称',
  `type` VARCHAR(50) NOT NULL COMMENT '文档类型：requirement, design, test, manual, contract, report, other',
  `file_path` VARCHAR(500) NOT NULL COMMENT '文件存储路径',
  `file_name` VARCHAR(200) NOT NULL COMMENT '原始文件名',
  `file_size` BIGINT NOT NULL DEFAULT 0 COMMENT '文件大小 (字节)',
  `file_ext` VARCHAR(20) DEFAULT NULL COMMENT '文件扩展名',
  `version` VARCHAR(20) DEFAULT 'v1' COMMENT '版本号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：0=deleted, 1=active',
  `uploaded_by` BIGINT UNSIGNED NOT NULL COMMENT '上传人 ID',
  `description` TEXT DEFAULT NULL COMMENT '文档描述',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_project_id` (`project_id`),
  KEY `idx_stage_id` (`stage_id`),
  KEY `idx_type` (`type`),
  KEY `idx_status` (`status`),
  CONSTRAINT `fk_document_project` FOREIGN KEY (`project_id`) REFERENCES `project` (`id`) ON DELETE CASCADE,
  CONSTRAINT `fk_document_stage` FOREIGN KEY (`stage_id`) REFERENCES `project_stage` (`id`) ON DELETE SET NULL,
  CONSTRAINT `fk_document_user` FOREIGN KEY (`uploaded_by`) REFERENCES `user` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='文档表';

-- 12. system_constant (系统常量表)
DROP TABLE IF EXISTS `system_constant`;
CREATE TABLE `system_constant` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `category` VARCHAR(50) NOT NULL COMMENT '分类：ProjectType, DocumentType, ActionType, etc.',
  `const_key` VARCHAR(100) NOT NULL COMMENT '常量键',
  `const_value` VARCHAR(200) NOT NULL COMMENT '常量值',
  `display_name` VARCHAR(100) NOT NULL COMMENT '显示名称',
  `order_num` INT NOT NULL DEFAULT 0 COMMENT '排序',
  `description` VARCHAR(255) DEFAULT NULL COMMENT '描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=active, 2=inactive',
  `is_system` TINYINT NOT NULL DEFAULT 0 COMMENT '是否系统内置：0=否，1=是',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_category_key` (`category`, `const_key`),
  KEY `idx_category` (`category`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='系统常量表';

-- 13. audit_log (审计日志表)
DROP TABLE IF EXISTS `audit_log`;
CREATE TABLE `audit_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户 ID',
  `action_type` VARCHAR(50) NOT NULL COMMENT '动作类型：login, logout, create, update, delete, export, upload, download',
  `action_category` VARCHAR(50) NOT NULL COMMENT '动作分类：system, project, document, user, role, constant',
  `table_name` VARCHAR(50) DEFAULT NULL COMMENT '操作的表名',
  `record_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '操作的记录 ID',
  `old_data` JSON DEFAULT NULL COMMENT '旧数据快照',
  `new_data` JSON DEFAULT NULL COMMENT '新数据快照',
  `ip_address` VARCHAR(45) DEFAULT NULL COMMENT 'IP 地址',
  `user_agent` VARCHAR(500) DEFAULT NULL COMMENT 'User-Agent',
  `request_url` VARCHAR(500) DEFAULT NULL COMMENT '请求 URL',
  `request_method` VARCHAR(10) DEFAULT NULL COMMENT '请求方法',
  `status_code` INT DEFAULT NULL COMMENT 'HTTP 状态码',
  `error_message` VARCHAR(500) DEFAULT NULL COMMENT '错误消息',
  `duration_ms` INT DEFAULT NULL COMMENT '执行时长 (毫秒)',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_user_id` (`user_id`),
  KEY `idx_action_type` (`action_type`),
  KEY `idx_action_category` (`action_category`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='审计日志表';

-- 14. notification_log (通知日志表)
DROP TABLE IF EXISTS `notification_log`;
CREATE TABLE `notification_log` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键 ID',
  `type` VARCHAR(50) NOT NULL COMMENT '通知类型：email, sms, internal',
  `recipient` VARCHAR(200) NOT NULL COMMENT '接收者',
  `subject` VARCHAR(200) DEFAULT NULL COMMENT '主题',
  `content` TEXT NOT NULL COMMENT '内容',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=pending, 2=sent, 3=failed',
  `error_message` VARCHAR(500) DEFAULT NULL COMMENT '错误消息',
  `sent_at` DATETIME DEFAULT NULL COMMENT '发送时间',
  `related_type` VARCHAR(50) DEFAULT NULL COMMENT '关联类型：project, stage, document',
  `related_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联 ID',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  KEY `idx_type` (`type`),
  KEY `idx_status` (`status`),
  KEY `idx_created_at` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='通知日志表';

SET FOREIGN_KEY_CHECKS = 1;

-- ============================================
-- 初始化数据
-- ============================================

-- 插入默认管理员用户 (密码：Admin@123)
INSERT INTO `user` (`username`, `email`, `password_hash`, `real_name`, `status`) VALUES
('admin', 'admin@example.com', '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', '系统管理员', 1);

-- 插入角色
INSERT INTO `role` (`name`, `code`, `description`, `status`) VALUES
('系统管理员', 'admin', '系统管理员，拥有所有权限', 1),
('项目经理', 'pm', '项目经理，管理项目', 1),
('项目成员', 'member', '项目成员，参与项目', 1);

-- 插入系统常量
INSERT INTO `system_constant` (`category`, `const_key`, `const_value`, `display_name`, `order_num`, `is_system`) VALUES
('ProjectType', 'software', 'software', '软件开发', 1, 1),
('ProjectType', 'construction', 'construction', '建筑工程', 2, 1),
('ProjectType', 'consulting', 'consulting', '咨询服务', 3, 1),
('ProjectType', 'audit', 'audit', '审计合规', 4, 1),
('ProjectType', 'government', 'government', '政府/科研', 5, 1),
('ProjectType', 'marketing', 'marketing', '市场活动', 6, 1),
('DocumentType', 'requirement', 'requirement', '需求文档', 1, 1),
('DocumentType', 'design', 'design', '设计文档', 2, 1),
('DocumentType', 'test', 'test', '测试报告', 3, 1),
('DocumentType', 'manual', 'manual', '用户手册', 4, 1),
('DocumentType', 'contract', 'contract', '合同', 5, 1),
('DocumentType', 'report', 'report', '报告', 6, 1),
('DocumentType', 'other', 'other', '其他', 7, 1),
('ActionCategory', 'system', 'system', '系统操作', 1, 1),
('ActionCategory', 'project', 'project', '项目相关', 2, 1),
('ActionCategory', 'document', 'document', '文档相关', 3, 1),
('ActionCategory', 'user', 'user', '用户管理', 4, 1),
('ActionCategory', 'role', 'role', '角色权限', 5, 1),
('ActionCategory', 'constant', 'constant', '系统常量', 6, 1),
('ActionType', 'login', 'login', '登录', 1, 1),
('ActionType', 'logout', 'logout', '登出', 2, 1),
('ActionType', 'create', 'create', '创建', 3, 1),
('ActionType', 'update', 'update', '更新', 4, 1),
('ActionType', 'delete', 'delete', '删除', 5, 1),
('ActionType', 'export', 'export', '导出', 6, 1),
('ActionType', 'upload', 'upload', '上传', 7, 1),
('ActionType', 'download', 'download', '下载', 8, 1);

-- 插入菜单
INSERT INTO `menu` (`name`, `code`, `icon`, `url`, `parent_id`, `order_num`, `status`) VALUES
('首页', 'dashboard', 'fa-home', '/', 0, 1),
('项目管理', 'project', 'fa-folder', '/projects', 0, 2),
('用户管理', 'user', 'fa-users', '/users', 0, 3),
('角色管理', 'role', 'fa-user-tag', '/roles', 0, 4),
('菜单管理', 'menu', 'fa-bars', '/menus', 0, 5),
('功能管理', 'function', 'fa-cog', '/functions', 0, 6),
('审计日志', 'audit', 'fa-history', '/audit-logs', 0, 7),
('文档管理', 'document', 'fa-file', '/documents', 0, 8);

-- 插入系统功能
INSERT INTO `system_function` (`menu_id`, `name`, `code`, `url`, `method`, `description`, `status`) VALUES
(1, '查看仪表盘', 'dashboard:view', '/dashboard', 'GET', '查看首页仪表盘', 1),
(2, '查看项目列表', 'project:list', '/projects', 'GET', '查看项目列表', 1),
(2, '创建项目', 'project:create', '/projects/api', 'POST', '创建新项目', 1),
(2, '编辑项目', 'project:edit', '/projects/api', 'PUT', '编辑项目信息', 1),
(2, '删除项目', 'project:delete', '/projects/api', 'DELETE', '删除项目', 1),
(2, '项目管理', 'project:manage', '/projects/api', 'POST', '项目管理（包含创建/编辑/删除）', 1),
(3, '查看用户列表', 'user:list', '/users', 'GET', '查看用户列表', 1),
(3, '创建用户', 'user:create', '/users/api', 'POST', '创建新用户', 1),
(3, '编辑用户', 'user:edit', '/users/api', 'PUT', '编辑用户信息', 1),
(3, '删除用户', 'user:delete', '/users/api', 'DELETE', '删除用户', 1),
(3, '重置密码', 'user:reset_password', '/users/api', 'POST', '重置用户密码', 1),
(3, '用户管理', 'user:manage', '/users/api', 'POST', '用户管理（包含创建/编辑/删除）', 1),
(4, '查看角色列表', 'role:list', '/roles', 'GET', '查看角色列表', 1),
(4, '创建角色', 'role:create', '/roles/api', 'POST', '创建新角色', 1),
(4, '编辑角色', 'role:edit', '/roles/api', 'PUT', '编辑角色信息', 1),
(4, '删除角色', 'role:delete', '/roles/api', 'DELETE', '删除角色', 1),
(4, '角色管理', 'role:manage', '/roles/api', 'POST', '角色管理（包含创建/编辑/删除）', 1),
(4, '分配菜单权限', 'role:menu', '/roles/menu', 'POST', '为角色分配菜单权限', 1),
(4, '分配功能权限', 'role:function', '/roles/function', 'POST', '为角色分配功能权限', 1),
(5, '查看菜单列表', 'menu:list', '/menus', 'GET', '查看菜单列表', 1),
(5, '编辑菜单', 'menu:edit', '/menus/api', 'POST', '编辑菜单（包含创建/更新）', 1),
(5, '删除菜单', 'menu:delete', '/menus/api', 'DELETE', '删除菜单', 1),
(6, '查看功能列表', 'function:list', '/functions', 'GET', '查看功能列表', 1),
(6, '功能管理', 'function:manage', '/functions/api', 'POST', '功能管理（包含创建/编辑/删除）', 1),
(7, '查看审计日志', 'audit:view', '/audit-logs', 'GET', '查看审计日志', 1),
(8, '查看文档列表', 'document:list', '/documents', 'GET', '查看文档列表', 1),
(8, '文档管理', 'document:manage', '/documents/api', 'POST', '文档管理（包含上传/删除）', 1);

-- 插入角色（管理员）
INSERT INTO `role` (`name`, `code`, `description`, `status`) VALUES
('系统管理员', 'admin', '拥有所有权限的系统管理员', 1),
('普通用户', 'user', '普通用户，只有基本查看权限', 1);

-- 为管理员角色分配所有菜单权限
INSERT INTO `role_menu` (`role_id`, `menu_id`) 
SELECT 1, id FROM menu;

-- 为管理员角色分配所有功能权限
INSERT INTO `role_function` (`role_id`, `function_id`) 
SELECT 1, id FROM system_function;

-- 为普通用户角色分配基本查看权限
INSERT INTO `role_menu` (`role_id`, `menu_id`) VALUES
(2, 1), -- 首页
(2, 2); -- 项目管理

INSERT INTO `role_function` (`role_id`, `function_id`) VALUES
(2, 1), -- dashboard:view
(2, 2); -- project:list

-- 插入项目模板（软件开发 v1）
INSERT INTO `project_template` (`name`, `code`, `version`, `type`, `description`, `status`, `created_by`) VALUES
('软件开发模板', 'software', 'v1', 'software', '标准软件开发项目模板', 1, 1);

-- 插入模板阶段
INSERT INTO `template_stage` (`template_id`, `name`, `code`, `order_num`, `description`) VALUES
(1, '需求分析', 'requirement', 1, '需求调研、需求分析、需求规格说明书'),
(1, '系统设计', 'design', 2, '概要设计、详细设计、数据库设计'),
(1, '编码实现', 'development', 3, '代码编写、单元测试'),
(1, '测试验证', 'testing', 4, '集成测试、系统测试、验收测试'),
(1, '上线部署', 'deployment', 5, '生产环境部署、上线'),
(1, '运维支持', 'maintenance', 6, '运维支持、bug 修复');

