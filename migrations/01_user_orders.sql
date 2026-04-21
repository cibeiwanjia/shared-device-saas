-- 统一订单表（聚合多业务线消费记录）
CREATE TABLE IF NOT EXISTS `user_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `order_no` VARCHAR(64) NOT NULL COMMENT '订单编号',
  `source` TINYINT UNSIGNED NOT NULL COMMENT '业务来源: 1=门票 2=共享单车 3=充电宝 4=智能快递柜',
  `order_type` VARCHAR(32) NOT NULL DEFAULT '' COMMENT '订单类型（各业务线自定义）',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=待支付 2=已支付 3=已完成 4=已取消 5=已退款',
  `total_amount` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总金额（分）',
  `currency` VARCHAR(8) NOT NULL DEFAULT 'CNY' COMMENT '币种',
  `payment_method` TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '支付方式: 0=未支付 1=微信 2=支付宝 3=钱包余额',
  `title` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '订单标题',
  `description` VARCHAR(512) NOT NULL DEFAULT '' COMMENT '订单描述',
  `extra_json` JSON DEFAULT NULL COMMENT '扩展信息（业务线特有字段）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_tenant_user_created` (`tenant_id`, `user_id`, `created_at` DESC),
  KEY `idx_tenant_user_source` (`tenant_id`, `user_id`, `source`),
  KEY `idx_tenant_user_status` (`tenant_id`, `user_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户统一订单表';
