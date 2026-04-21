# Go 版本升级变更记录

## 变更概述

将全项目 Go 版本从 `1.19`（Dockerfile）/ `1.25.5`（go.mod）统一升级至 `1.26.2`。

## 变更日期

2026-04-21

## 变更原因

- 原 Dockerfile 中使用 `golang:1.19`，版本过旧
- go.mod 中使用 `go 1.25.5`，与 Dockerfile 版本不一致
- Go 1.26.2 为当前最新稳定版（2026-04-08 发布），包含安全修复（CVE-2026-32281）

## Go 1.26.2 版本验证

- **官方发布日期**：2026-04-08
- **发布类型**：安全修复版本（minor point release）
- **修复内容**：修复 Linux 下 `Root.Chmod` 在操作过程中若目标被替换为符号链接可能导致的安全问题（CVE-2026-32281）
- **官方下载页**：https://go.dev/dl/
- **发布公告**：https://groups.google.com/g/golang-announce/c/0uYbvbPZRWU

## 变更文件清单

### 1. Dockerfile（4 个文件）

| 文件路径 | 变更前 | 变更后 |
|---------|--------|--------|
| `app/user/Dockerfile` | `FROM golang:1.19 AS builder` | `FROM golang:1.26.2 AS builder` |
| `app/storage/Dockerfile` | `FROM golang:1.19 AS builder` | `FROM golang:1.26.2 AS builder` |
| `app/portal/Dockerfile` | `FROM golang:1.19 AS builder` | `FROM golang:1.26.2 AS builder` |
| `app/device/Dockerfile` | `FROM golang:1.19 AS builder` | `FROM golang:1.26.2 AS builder` |

### 2. go.mod（1 个文件）

| 文件路径 | 变更前 | 变更后 |
|---------|--------|--------|
| `go.mod` | `go 1.25.5` | `go 1.26.2` |

## 变更后全项目 Go 版本一致性

| 位置 | 版本 |
|------|------|
| Dockerfile（构建镜像） | `golang:1.26.2` |
| go.mod（模块要求） | `go 1.26.2` |
| 本地开发环境（建议） | `go1.26.2` |

## Go 1.26 新特性

> 以下为 Go 1.26 相对于 1.25 的主要变更，完整内容见官方文档：https://go.dev/doc/go1.26

### 语言变更

#### 1. `new` 支持表达式初始化

`new` 函数现在接受表达式作为参数，可以直接指定初始值：

```go
// Go 1.25 及之前：需要先声明变量再取地址
x := int64(300)
ptr := &x

// Go 1.26：直接用 new 表达式
ptr := new(int64(300))
```

对 `encoding/json`、Protocol Buffers 等序列化场景特别实用，可以简化指针字段初始化。

#### 2. 泛型类型自引用解禁

泛型类型现在可以在自身的类型参数列表中引用自身：

```go
// Go 1.25 及之前：编译报错
// Go 1.26：合法
type Tree[T comparable] struct {
    Left  *Tree[T]
    Right *Tree[T]
    Value T
}
```

树形结构、链表等递归数据结构定义更加自然。

### 运行时

#### 3. Green Tea 垃圾回收器正式启用

之前在 Go 1.25 中作为实验特性的 Green Tea GC，现在默认开启：

- 降低 GC 暂停延迟
- 减少内存占用
- 可通过 `GOEXPERIMENT=nogreenteagc` 回退到旧 GC

#### 4. CGO 调用开销降低约 30%

Go 与 C 互操作的基线开销降低约 30%，对项目中有 CGO 依赖的场景（如数据库驱动）有直接性能提升。

### 工具链

#### 5. `go fix` 全面重写

基于 Go analysis 框架完全重写，内置数十个"现代化器"（modernizer），一键将代码升级到最新惯用写法：

```bash
go fix ./...
```

#### 6. `go mod init` 默认版本调整

新建模块时默认写入较低的 `go` 版本，提升兼容性。

### 标准库新增

#### 7. `crypto/hpke` — 混合公钥加密

实现了 RFC 9180 规范的 Hybrid Public Key Encryption，支持后量子混合 KEM，适合需要端到端加密的场景。

#### 8. `crypto/mlkem/mlkemtest` — ML-KEM 测试工具

为后量子密码算法 ML-KEM（Module-Lattice-Based Key-Encapsulation Mechanism）提供测试支持。

#### 9. `testing/cryptotest` — 密码学测试助手

简化密码学相关代码的测试编写。

### 实验性特性

#### 10. `simd/archsimd` — SIMD 向量化操作（实验性）

通过 `GOEXPERIMENT=simd` 启用，提供架构特定的 SIMD 指令访问。目前支持 amd64 架构的 128 位操作，预期未来版本正式发布。

### 与本项目的关联

| 特性 | 对本项目的价值 |
|------|---------------|
| Green Tea GC 默认启用 | 微服务运行时 GC 暂停更短，接口响应更稳定 |
| CGO 开销降低 30% | MySQL 驱动等 CGO 调用性能提升 |
| `new` 表达式初始化 | 简化 Proto/JSON 序列化中的指针字段写法 |
| 泛型自引用 | 可用于构建通用树形/链表工具结构 |
| `crypto/hpke` | 未来可用于设备端与平台间的端到端加密通信 |
| `go fix` 重写 | 方便一键升级项目代码到最新风格 |

## 注意事项

- 升级后首次构建需执行 `go mod tidy` 确保 `go.sum` 同步更新
- Go 1.26 向下兼容 1.25/1.22 的代码，无需修改业务代码
- 建议本地开发环境也升级至 `go1.26.2`，保持与 Docker 构建环境一致
