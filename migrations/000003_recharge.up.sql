-- 充值订单表（金额以分为单位，BIGINT 存储）
CREATE TABLE IF NOT EXISTS `recharge_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `order_no` VARCHAR(64) NOT NULL COMMENT '充值订单号',
  `amount` BIGINT NOT NULL COMMENT '充值金额（分）',
  `payment_method` TINYINT UNSIGNED NOT NULL COMMENT '支付方式: 1=微信 2=支付宝',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=待支付 2=已支付 3=失败',
  `channel_order_no` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '第三方支付订单号',
  `paid_at` DATETIME DEFAULT NULL COMMENT '支付完成时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_user_status` (`tenant_id`, `user_id`, `status`),
  KEY `idx_channel_order` (`channel_order_no`),
  CONSTRAINT `chk_amount_positive` CHECK (`amount` > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充值订单表';
