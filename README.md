# SimpleCache

基于 Go 的只读分布式缓存与 Kubernetes Operator 项目。目前缓存引擎已完成请求可靠性改造，成员配置仍使用静态列表。架构与调用关系见 [ARCHITECTURE.md](docs/ARCHITECTURE.md)，后续目标见 [DESIGN.md](DESIGN.md)。

## 当前能力

- 本地 LRU、TTL、容量限制和 ByteView 数据隔离；Bloom 从完整只读演示数据源初始化。
- API 读取优先访问一致性哈希选出的 owner；Peer 服务端只执行本地加载，避免循环转发。
- routeFlight 合并路由工作，originFlight 合并实际回源；调用者取消不会取消其他等待者的共享工作。
- Peer 请求超时或网络/协议故障时本地回源；明确 Key 缺失直接返回，未知 Group 作为配置错误降级。
- 每进程共享回源并发上限；等待名额计入源工作预算。
- 独立 HTTP Server、健康/就绪接口和 SIGTERM 优雅退出。
- API、本地命中、Peer、回源、降级、名额等待等基础 Prometheus 指标。

动态成员、原子路由快照和完整 Operator 改造尚未接入当前版本。静态路由使用锁保护，Operator 仍保留原实现，尚不能把编译成功作为 Kubernetes 部署成功的证据。

## 目录

```text
geecache-engine/
  cmd/simplecache/          参数、信号与启动入口
  internal/app/             配置、API、服务生命周期
  internal/cache/           缓存策略、两层 SingleFlight、回源限制
  internal/peer/            HTTP 客户端/服务端与静态路由
  internal/demo/            有限只读演示源
  internal/telemetry/       进程级指标
simplecache-operator/       现有 Kubebuilder 项目
```

## 快速检查

需要兼容 Go 1.25.3 的工具链（Operator 的要求；缓存引擎仍声明 Go 1.24.4）。

```bash
make test    # 引擎功能测试，不启动负载压测
make race
make build  # 编译引擎与 Operator
make vet
make check
```

Operator 原 envtest 是单独的 make test-operator，不属于当前默认快速检查。构建检查已关闭自动 VCS 查询。

## 本地单节点运行

```bash
cd geecache-engine
go run -buildvcs=false ./cmd/simplecache -port=8001 -api=true
```

默认静态成员仅包含自己。多个节点可设置 SELF_ADDR 和逗号分隔的 PEERS（http URL）。CLI 默认 peer port 为 8002，API 默认关闭；上面的命令显式启用 API。

```bash
curl -i 'http://localhost:9999/api?key=Auto-666'
curl -i 'http://localhost:9999/api?key=missing'
curl -i 'http://localhost:9999/readyz'
curl -s 'http://localhost:9999/metrics'
```

已知 Key 返回值，缺失返回 404，空/超长 Key 返回 400。Peer 超时或故障在源可用且剩余预算允许时降级；源异常返回 503，超时返回 504。

| 参数 | 默认值 |
| --- | --- |
| CACHE_BYTES | 64MiB 的 Key/Value 逻辑容量 |
| TTL_SECONDS | 60 |
| SOURCE_MAX_CONCURRENCY | 32，按进程限制 |
| API_ADDR | 0.0.0.0:9999 |
| DISCOVERY_MODE | static；当前仅支持静态模式 |

## 时间预算与边界

API 等待上限 1500ms，共享路由工作 1200ms，单次 Peer 最多 300ms，独立源工作 800ms（包含等待回源名额）。等待独立源工作不能延长路由预算；路由已返回时，源工作可在自己的剩余预算内完成并缓存。进程退出先停止接收/进入不就绪，再等待在途 HTTP 请求，最后取消共享工作。

演示源只读，包含 Tom/Jack/Sam 和 Auto-0 到 Auto-9999，模拟 100ms 延迟。它不是真实数据库；项目没有写入 API、复制协议或强一致性保证。SingleFlight 与回源上限都按进程生效；Peer 成功结果不写入入口缓存，fallback 的本地副本可保留到 TTL/淘汰。

本轮只做功能与并发性质验证，不发布吞吐或高并发性能数字。
