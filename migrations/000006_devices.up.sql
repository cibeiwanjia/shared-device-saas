-- 设备表
CREATE TABLE IF NOT EXISTS `devices` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `device_type` VARCHAR(32) NOT NULL COMMENT '设备类型: power_bank/bike/locker',
  `device_sn` VARCHAR(64) NOT NULL COMMENT '设备序列号',
  `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '设备名称',
  `status` TINYINT NOT NULL DEFAULT 0 COMMENT '状态: 0=离线, 1=在线空闲, 2=占用, 3=故障',
  `location_lat` DECIMAL(10,7) DEFAULT NULL COMMENT '纬度',
  `location_lng` DECIMAL(10,7) DEFAULT NULL COMMENT '经度',
  `location_name` VARCHAR(255) DEFAULT '' COMMENT '位置描述',
  `station_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '所属站点ID',
  `battery_level` TINYINT UNSIGNED DEFAULT NULL COMMENT '电量百分比',
  `metadata_json` JSON DEFAULT NULL COMMENT '扩展元数据(格口信息等)',
  `last_online_at` DATETIME DEFAULT NULL COMMENT '最后上线时间',
  `last_offline_at` DATETIME DEFAULT NULL COMMENT '最后下线时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_tenant_device_sn` (`tenant_id`, `device_sn`),
  KEY `idx_tenant_type_status` (`tenant_id`, `device_type`, `status`),
  KEY `idx_station_id` (`station_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='设备表';

-- 设备连接事件表
CREATE TABLE IF NOT EXISTS `device_connection_events` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `device_id` BIGINT UNSIGNED NOT NULL COMMENT '设备ID',
  `event_type` VARCHAR(16) NOT NULL COMMENT '事件类型: connected/disconnected',
  `reason_code` INT DEFAULT NULL COMMENT 'MQTT断开原因码',
  `ip_address` VARCHAR(45) DEFAULT '' COMMENT '客户端IP',
  `client_id` VARCHAR(128) DEFAULT '' COMMENT 'MQTT Client ID',
  `occurred_at` DATETIME NOT NULL COMMENT '事件发生时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_device_time` (`device_id`, `occurred_at`),
  KEY `idx_tenant_time` (`tenant_id`, `occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='设备连接事件表';
