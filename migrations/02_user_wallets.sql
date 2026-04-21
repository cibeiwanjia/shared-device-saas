-- 用户钱包表
CREATE TABLE IF NOT EXISTS `user_wallets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `user_id` BIGINT UNSIGNED NOT NULL UNIQUE COMMENT '用户ID（每用户一个钱包）',
  `balance` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '可用余额（元）',
  `frozen_amount` DECIMAL(12,2) NOT NULL DEFAULT 0.00 COMMENT '冻结金额（元）',
  `version` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '乐观锁版本号',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_user` (`tenant_id`, `user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户钱包表';

-- 钱包流水表
CREATE TABLE IF NOT EXISTS `wallet_transactions` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `wallet_id` BIGINT UNSIGNED NOT NULL COMMENT '钱包ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `type` TINYINT UNSIGNED NOT NULL COMMENT '类型: 1=充值 2=消费 3=退款 4=冻结 5=解冻',
  `amount` DECIMAL(12,2) NOT NULL COMMENT '金额（正数=入账，负数=扣款）',
  `balance_after` DECIMAL(12,2) NOT NULL COMMENT '操作后余额',
  `order_no` VARCHAR(64) NOT NULL DEFAULT '' COMMENT '关联订单号',
  `description` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '描述',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_wallet_created` (`wallet_id`, `created_at` DESC),
  KEY `idx_user_type` (`tenant_id`, `user_id`, `type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='钱包流水表';
