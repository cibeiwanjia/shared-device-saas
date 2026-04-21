-- 充值订单表
CREATE TABLE IF NOT EXISTS `recharge_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `order_no` VARCHAR(64) NOT NULL COMMENT '充值订单号',
  `amount` DECIMAL(12,2) NOT NULL COMMENT '充值金额（元）',
  `payment_method` TINYINT UNSIGNED NOT NULL COMMENT '支付方式: 1=微信 2=支付宝',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=待支付 2=已支付 3=失败',
  `channel_order_no` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '第三方支付订单号',
  `paid_at` DATETIME DEFAULT NULL COMMENT '支付完成时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user_status` (`tenant_id`, `user_id`, `status`),
  KEY `idx_channel_order` (`channel_order_no`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充值订单表';

-- JWT 会话表
CREATE TABLE IF NOT EXISTS `jwt_sessions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `session_id` VARCHAR(64) NOT NULL COMMENT '会话ID',
  `device_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备指纹',
  `device_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备名称',
  `ip` VARCHAR(45) NOT NULL DEFAULT '' COMMENT '登录IP',
  `user_agent` VARCHAR(256) NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `refresh_token_hash` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'Refresh Token 哈希',
  `expires_at` DATETIME NOT NULL COMMENT '过期时间',
  `revoked_at` DATETIME DEFAULT NULL COMMENT '撤销时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_session_id` (`session_id`),
  KEY `idx_user_active` (`tenant_id`, `user_id`, `revoked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='JWT会话表';
