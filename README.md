# Shared Device SaaS — 智慧园区共享设备管理平台

## 项目简介

SaaS 模式的智慧园区共享设备管理平台，支持多园区（多租户）独立运营。覆盖用户管理、物品寄存、首页聚合、设备运维等核心业务场景。

## 技术栈

| 类别 | 技术选型 |
|------|---------|
| 语言 | Go 1.22+ |
| 微服务框架 | [go-kratos](https://github.com/go-kratos/kratos) v2 |
| API 定义 | Protocol Buffers + buf |
| 服务间通信 | gRPC |
| 对外接口 | HTTP（BFF 层） |
| 数据库 | MySQL 8.0 |
| 缓存 | Redis |
| 依赖注入 | Google Wire |
| 容器化 | Docker |

## 架构设计

### DDD 分层

项目采用领域驱动设计（DDD）分层架构，结合 Kratos layout 规范：

```
internal/
├── service/    # 应用层（Application）— DTO 转换、业务编排
├── biz/        # 领域层（Domain）— 核心业务逻辑、领域实体、仓储接口
├── data/       # 基础设施层（Infrastructure）— 实现 biz 层接口，封装 DB/Redis
└── server/     # 接口层（Interface）— HTTP/gRPC Server 配置与路由
```

### 多租户方案

- **共享数据库 + tenant_id 字段隔离**：同一套 MySQL，每张业务表都有 tenant_id
- Kratos 中间件从 JWT Token / HTTP Header 解析 tenant_id，注入 Context
- data 层查询自动追加 `WHERE tenant_id = ?`

## 目录结构

```
shared-device-saas/
├── api/                          # Proto API 定义（集中管理）
│   ├── user/v1/                  # 用户服务 API
│   ├── storage/v1/               # 物品寄存服务 API
│   ├── portal/v1/                # 首页服务 API
│   ├── device/v1/                # 设备运维服务 API
│   └── tenant/v1/                # 租户管理 API
│
├── app/                          # 微服务实现
│   ├── user/                     # 用户服务（登录注册 + 个人中心）
│   ├── storage/                  # 物品寄存服务
│   ├── portal/                   # 首页聚合服务（BFF）
│   └── device/                   # 设备运维服务
│
├── pkg/                          # 跨服务公共包
│   ├── middleware/tenant/        # 多租户中间件
│   ├── auth/                     # JWT 认证工具
│   ├── redis/                    # Redis 通用工具
│   ├── errx/                     # 统一错误码
│   └── paginate/                 # 分页工具
│
├── third_party/                  # 第三方 Proto 依赖
├── deploy/                       # 部署配置（Docker Compose / K8s）
├── go.mod                        # Mono-Repo 统一依赖管理
└── Makefile                      # 构建脚本
```

## 微服务说明

| 服务 | 职责 | 团队成员 | 核心领域 |
|------|------|---------|---------|
| **user** | 登录注册、用户个人中心 | - | User 聚合根 |
| **storage** | 物品寄存、取件码、超时计费 | - | StorageOrder 聚合根 |
| **portal** | 首页数据聚合、Banner、通知公告 | - | PortalConfig 聚合根 |
| **device** | 设备管理、告警、运维工单 | - | Device / WorkOrder 聚合根 |

## 快速开始

### 环境要求

- Go 1.22+
- Protocol Buffers (`protoc`)
- Docker & Docker Compose
- Kratos CLI: `go install github.com/go-kratos/kratos/cmd/kratos/v2@latest`

### 安装依赖

```bash
go mod tidy
```

### 生成 Proto 代码

```bash
# 在各微服务目录下执行
cd app/user && make api
cd app/storage && make api
cd app/portal && make api
cd app/device && make api
```

### 启动服务

```bash
# 启动依赖基础设施
docker-compose -f deploy/docker-compose.yml up -d

# 启动各微服务
cd app/user && go run cmd/user/main.go -conf configs/config.yaml
cd app/storage && go run cmd/storage/main.go -conf configs/config.yaml
cd app/portal && go run cmd/portal/main.go -conf configs/config.yaml
cd app/device && go run cmd/device/main.go -conf configs/config.yaml
```

## 开发规范

### DDD 分层职责

| 层 | 写什么 | 不写什么 |
|----|--------|---------|
| server/ | 路由配置、中间件挂载 | 业务逻辑 |
| service/ | 入参校验、调用 biz、组装响应 | 业务规则 |
| biz/ | 实体、领域服务、仓储接口 | DB 操作、外部调用 |
| data/ | DB 查询、Redis 操作、第三方调用 | 业务判断 |

### 依赖方向

```
server → service → biz ← data
```

- biz 层定义接口（Repo Interface），data 层实现接口
- service 层调用 biz 层，不直接调用 data 层
- 依赖倒置：biz 不依赖 data，data 依赖 biz 的接口定义

### 多租户开发

- 所有业务表必须包含 `tenant_id` 字段
- data 层查询必须携带 tenant_id 条件
- 通过 `pkg/middleware/tenant/` 中间件自动注入

## 项目规划

- [x] 项目骨架搭建（Mono-Repo + Kratos Layout）
- [ ] 公共包实现（多租户中间件、JWT 认证、错误码、分页）
- [ ] user 服务：登录注册 + 个人中心
- [ ] storage 服务：物品寄存
- [ ] portal 服务：首页聚合
- [ ] device 服务：设备运维
- [ ] Docker Compose 开发环境
- [ ] CI/CD 流水线

## License

MIT
