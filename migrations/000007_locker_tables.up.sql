-- 柜机实例表（device 的业务投影）
CREATE TABLE IF NOT EXISTS `cabinets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户/园区ID',
  `device_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 device 表',
  `device_sn` VARCHAR(32) NOT NULL COMMENT '柜机序列号',
  `name` VARCHAR(64) NOT NULL COMMENT '柜机名称',
  `location_name` VARCHAR(128) NOT NULL COMMENT '安装位置',
  `total_cells` INT NOT NULL COMMENT '格口总数',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=正常 2=维护中 3=停用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_device_id` (`device_id`),
  KEY `idx_tenant_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='柜机实例表';

-- 格口表
CREATE TABLE IF NOT EXISTS `cells` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `cabinet_id` BIGINT UNSIGNED NOT NULL COMMENT '所属柜机',
  `slot_index` INT NOT NULL COMMENT '对应 device 服务的 slot 编号',
  `cell_type` TINYINT NOT NULL DEFAULT 1 COMMENT '1=小 2=中 3=大',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=空闲 2=占用 3=开门中 4=故障 5=停用',
  `current_order_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前占用订单',
  `pending_action` TINYINT DEFAULT NULL COMMENT '1=等投递确认 2=等取件确认 3=临时开柜 4=等寄存确认',
  `opened_at` DATETIME DEFAULT NULL COMMENT '开门时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cabinet_slot` (`cabinet_id`, `slot_index`),
  KEY `idx_tenant_status` (`tenant_id`, `status`),
  KEY `idx_cabinet_status` (`cabinet_id`, `status`),
  KEY `idx_open_timeout` (`status`, `opened_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='格口表';

-- 快递柜订单表
CREATE TABLE IF NOT EXISTS `storage_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `order_no` VARCHAR(32) NOT NULL COMMENT '业务订单号',
  `order_type` VARCHAR(16) NOT NULL COMMENT 'delivery_in / delivery_out / storage',
  `status` INT NOT NULL DEFAULT 10 COMMENT 'FSM 状态码',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '主角色',
  `operator_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '副角色',
  `cabinet_id` BIGINT UNSIGNED NOT NULL,
  `cell_id` BIGINT UNSIGNED NOT NULL,
  `device_sn` VARCHAR(32) NOT NULL,
  `slot_index` INT NOT NULL,
  `pickup_code` VARCHAR(6) DEFAULT NULL COMMENT '取件码',
  `deposited_at` DATETIME DEFAULT NULL COMMENT '物品放入时间',
  `picked_up_at` DATETIME DEFAULT NULL COMMENT '取出时间',
  `total_amount` INT NOT NULL DEFAULT 0 COMMENT '总费用（分）',
  `paid_amount` INT NOT NULL DEFAULT 0 COMMENT '已付金额（分）',
  `overtime_minutes` INT NOT NULL DEFAULT 0 COMMENT '超时分钟数',
  `remark` VARCHAR(256) DEFAULT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_tenant_user` (`tenant_id`, `user_id`),
  KEY `idx_tenant_cabinet` (`tenant_id`, `cabinet_id`, `status`),
  KEY `idx_status_timeout` (`status`, `deposited_at`),
  KEY `idx_tenant_pickup` (`tenant_id`, `pickup_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='快递柜订单表';

-- 计费规则表
CREATE TABLE IF NOT EXISTS `pricing_rules` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `rule_type` VARCHAR(32) NOT NULL COMMENT 'storage_overtime / storage_daily / delivery_fee',
  `free_hours` INT NOT NULL DEFAULT 24 COMMENT '免费时长（小时）',
  `price_per_hour` INT NOT NULL DEFAULT 0 COMMENT '每小时费用（分）',
  `price_per_day` INT NOT NULL DEFAULT 0 COMMENT '每天封顶（分）',
  `max_fee` INT NOT NULL DEFAULT 0 COMMENT '总封顶（分）',
  `cell_type` TINYINT DEFAULT NULL COMMENT 'NULL=全部类型',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级，越大越优先',
  `effective_from` DATETIME DEFAULT NULL COMMENT '生效起始',
  `effective_until` DATETIME DEFAULT NULL COMMENT '生效截止',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_type` (`tenant_id`, `rule_type`, `enabled`),
  KEY `idx_effective` (`tenant_id`, `rule_type`, `effective_from`, `effective_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='计费规则表';
