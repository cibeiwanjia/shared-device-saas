-- 用户表（核心身份数据，存 MySQL）
CREATE TABLE IF NOT EXISTS `users` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `phone` VARCHAR(20) NOT NULL COMMENT '手机号',
  `username` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '用户名',
  `password_hash` VARCHAR(128) NOT NULL COMMENT '密码哈希',
  `nickname` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '昵称',
  `avatar` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '头像URL',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=正常 2=禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_phone` (`tenant_id`, `phone`),
  KEY `idx_tenant_username` (`tenant_id`, `username`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户表';
