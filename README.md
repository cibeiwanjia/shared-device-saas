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

## 目录结构功能说明
shared-device-saas/
│
├── api/                              # 【API 定义层】集中管理所有微服务的 Proto 接口定义
│   ├── user/v1/                      #   用户服务接口（登录、注册、个人信息 CRUD）
│   ├── storage/v1/                   #   物品寄存服务接口（寄存、取件、计费）
│   ├── portal/v1/                    #   首页聚合服务接口（Banner、公告、推荐位）
│   ├── device/v1/                    #   设备运维服务接口（设备状态、告警、工单）
│   └── tenant/v1/                    #   租户管理接口（租户创建、配置、隔离）
│
├── app/                              # 【微服务实现层】每个子目录是一个独立的微服务
│   ├── user/                         #   👤 用户服务 — 登录注册、JWT 签发、个人中心管理
│   │   ├── cmd/user/                 #     服务启动入口（main.go）
│   │   ├── configs/config.yaml       #     服务配置文件（DB/Redis/端口等）
│   │   ├── internal/                 #     DDD 分层实现（见下方说明）
│   │   │   ├── server/               #       接口层 — HTTP/gRPC 路由与中间件挂载
│   │   │   ├── service/              #       应用层 — 参数校验、调用 biz、组装响应
│   │   │   ├── biz/                  #       领域层 — 核心业务逻辑、领域实体、仓储接口定义
│   │   │   ├── data/                 #       基础设施层 — DB/Redis 操作，实现 biz 层接口
│   │   │   └── conf/                 #       配置结构体定义（Proto 生成的配置类型）
│   │   ├── Dockerfile                #     服务容器化构建文件
│   │   └── Makefile                  #     构建/生成脚本（proto 生成、编译、运行）
│   │
│   ├── storage/                      #   📦 物品寄存服务 — 智能快递柜寄存、取件码生成、超时计费
│   │   └── (同 user 结构)
│   │
│   ├── portal/                       #   🏠 首页聚合服务(BFF) — 聚合各服务数据、Banner 管理、通知公告
│   │   └── (同 user 结构)
│   │
│   └── device/                       #   🔧 设备运维服务 — 共享设备（单车/充电宝/快递柜）状态监控、告警、运维工单
│       └── (同 user 结构)
│
├── pkg/                              # 【跨服务公共包】所有微服务共享的工具库
│   ├── middleware/tenant/             #   🏢 多租户中间件 — 从 JWT/Header 解析 tenant_id，注入 Context
│   ├── auth/                         #   🔐 JWT 认证工具 — Token 生成、解析、校验
│   ├── redis/                        #   ⚡ Redis 通用工具 — 连接池封装、缓存操作封装
│   ├── errx/                         #   ❌ 统一错误码 — 业务错误码定义、标准化错误响应格式
│   └── paginate/                     #   📄 分页工具 — 通用分页参数解析与响应封装
│
├── third_party/                      # 【第三方依赖】Proto 文件的第三方依赖（如 Google API、Kratos Proto）
├── deploy/                           # 【部署配置】Docker Compose 开发环境 / Kubernetes 生产部署 YAML
├── go.mod                            # Go Module 定义（Mono-Repo 统一依赖管理）
├── go.sum                            # 依赖校验文件
├── Makefile                          # 根目录构建脚本（全局命令）
└── .gitignore                        # Git 忽略规则


### 各目录职责速查
| 目录 | 一句话说明 | 对应你的哪个业务 |
|------|-----------|----------------|
| `api/user/` | 用户服务的 API 契约 | 登录注册 |
| `api/storage/` | 物品寄存服务的 API 契约 | 智能快递柜 |
| `api/portal/` | 首页聚合的 API 契约 | 园区首页 |
| `api/device/` | 设备运维的 API 契约 | 共享单车、充电宝 |
| `api/tenant/` | 租户管理的 API 契约 | 多园区 SaaS 隔离 |
| `app/user/` | 用户服务完整实现 | 登录注册、个人中心 |
| `app/storage/` | 物品寄存服务完整实现 | 快递柜寄存、取件码、计费 |
| `app/portal/` | 首页聚合服务完整实现 | Banner、公告、首页数据聚合 |
| `app/device/` | 设备运维服务完整实现 | 单车/充电宝/快递柜 设备管理 |
| `pkg/middleware/tenant/` | 多租户中间件 | 所有业务都依赖，自动隔离数据 |
| `pkg/auth/` | JWT 认证 | 用户登录鉴权 |
| `pkg/redis/` | Redis 工具 | 缓存、分布式锁 |
| `pkg/errx/` | 统一错误码 | 全局错误响应标准化 |
| `pkg/paginate/` | 分页工具 | 列表查询统一分页 |
| `third_party/` | 第三方 Proto | proto 编译依赖 |
| `deploy/` | 部署配置 | Docker/K8s 部署 |

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
