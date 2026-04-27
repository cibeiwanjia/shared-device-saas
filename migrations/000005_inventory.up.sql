-- ========================================
-- 库存相关表（快递柜 + 充电宝）
-- 与 orders/wallets 在同一 MySQL 实例，保证事务一致性
-- ========================================

-- 快递柜表
CREATE TABLE IF NOT EXISTS `cabinets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `cabinet_no` VARCHAR(32) NOT NULL COMMENT '柜子编号',
  `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '柜子名称/位置描述',
  `longitude` DECIMAL(10,7) DEFAULT NULL COMMENT '经度',
  `latitude` DECIMAL(10,7) DEFAULT NULL COMMENT '纬度',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=在线 2=离线 3=维护中',
  `total_cells` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '格口总数',
  `available_cells` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '可用格口数（冗余计数，减少实时聚合查询）',
  `created_at` BIGINT NOT NULL COMMENT '创建时间（Unix时间戳）',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间（Unix时间戳）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_cabinet_no` (`tenant_id`, `cabinet_no`),
  KEY `idx_tenant_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='快递柜表';

-- 格口表（快递柜的库存单位）
CREATE TABLE IF NOT EXISTS `cells` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `cabinet_id` BIGINT UNSIGNED NOT NULL COMMENT '所属快递柜ID',
  `cell_no` VARCHAR(16) NOT NULL COMMENT '格口号（如 A01、B12）',
  `cell_type` TINYINT UNSIGNED NOT NULL COMMENT '格口类型: 1=小 2=中 3=大',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=空闲 2=已占用 3=超时未取 4=故障',
  `current_order_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前关联订单ID',
  `created_at` BIGINT NOT NULL COMMENT '创建时间（Unix时间戳）',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间（Unix时间戳）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cabinet_cell` (`cabinet_id`, `cell_no`),
  KEY `idx_cabinet_type_status` (`cabinet_id`, `cell_type`, `status`),
  KEY `idx_status_order` (`status`, `current_order_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='快递柜格口表';

-- 充电宝站点表
CREATE TABLE IF NOT EXISTS `stations` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `station_no` VARCHAR(32) NOT NULL COMMENT '站点编号',
  `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '站点名称',
  `location_text` VARCHAR(256) NOT NULL DEFAULT '' COMMENT '位置描述',
  `longitude` DECIMAL(10,7) DEFAULT NULL COMMENT '经度',
  `latitude` DECIMAL(10,7) DEFAULT NULL COMMENT '纬度',
  `total_slots` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '总槽数',
  `available_count` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '可借充电宝数量（冗余计数）',
  `return_slots` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '可还空槽数',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=在线 2=离线 3=维护中',
  `created_at` BIGINT NOT NULL COMMENT '创建时间（Unix时间戳）',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间（Unix时间戳）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_station_no` (`tenant_id`, `station_no`),
  KEY `idx_tenant_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充电宝站点表';

-- 充电宝表（充电宝的库存单位）
CREATE TABLE IF NOT EXISTS `power_banks` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `bank_no` VARCHAR(32) NOT NULL COMMENT '充电宝编号',
  `station_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前所在站点ID',
  `slot_no` VARCHAR(16) DEFAULT NULL COMMENT '当前所在槽位号',
  `battery_level` TINYINT UNSIGNED NOT NULL DEFAULT 100 COMMENT '电量百分比',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=在仓可借 2=借出 3=故障 4=退役',
  `charge_cycles` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '充放电次数',
  `created_at` BIGINT NOT NULL COMMENT '创建时间（Unix时间戳）',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间（Unix时间戳）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_bank_no` (`tenant_id`, `bank_no`),
  KEY `idx_station_status` (`station_id`, `status`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充电宝表';

-- 计费规则表
CREATE TABLE IF NOT EXISTS `pricing_rules` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL COMMENT '租户ID',
  `name` VARCHAR(64) NOT NULL COMMENT '规则名称',
  `source` TINYINT UNSIGNED NOT NULL COMMENT '适用业务: 3=充电宝 4=快递柜',
  `free_minutes` INT UNSIGNED NOT NULL DEFAULT 5 COMMENT '免费时长（分钟）',
  `rate_per_hour` BIGINT NOT NULL COMMENT '每小时费用（分）',
  `daily_cap` BIGINT NOT NULL DEFAULT 0 COMMENT '每日封顶（分），0=不封顶',
  `grace_minutes` INT UNSIGNED NOT NULL DEFAULT 0 COMMENT '宽限时长（分钟）',
  `deposit_amount` BIGINT NOT NULL DEFAULT 9900 COMMENT '押金金额（分）',
  `status` TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '状态: 1=启用 2=禁用',
  `created_at` BIGINT NOT NULL COMMENT '创建时间（Unix时间戳）',
  `updated_at` BIGINT NOT NULL COMMENT '更新时间（Unix时间戳）',
  PRIMARY KEY (`id`),
  KEY `idx_tenant_source_status` (`tenant_id`, `source`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='计费规则表';
