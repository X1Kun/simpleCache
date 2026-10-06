# 缓存引擎架构与阅读顺序

缓存进程处理数据请求；Operator 管理 Kubernetes 资源。目前成员是静态列表，Operator 仍为原控制器实现。

## 阅读顺序

1. cmd/simplecache/main.go：参数、信号与退出错误。
2. internal/app/config.go：环境配置校验与静态成员规范化。
3. internal/app/server.go：组装数据源、缓存、路由和指标，创建 API/Peer Server；serve 负责在途请求排空。
4. internal/cache/group.go：Get 与 GetLocal、路由/源两层 SingleFlight 和工作预算。
5. internal/cache/limiter.go：实际 Getter 的并发名额。
6. internal/peer/client.go、server.go、status.go：单跳 HTTP + Protobuf、错误码、URL 编码和响应限长。
7. internal/peer/router.go：锁保护的静态环与客户端列表；hashring 保留原算法。
8. internal/demo/source.go、internal/telemetry/metrics.go：可取消的有限源和独立指标 registry。

## 依赖关系

cache 定义 Getter、PeerGetter、PeerPicker 和 Observer 接口，不依赖 HTTP、Protobuf 或 Kubernetes。peer 实现通信和选路，依赖 cache 接口。app 组装各包；不使用全局 Group 注册表或全局 HTTP mux。

## 请求链路

API：校验/本地缓存/Bloom → routeFlight → Peer 或本地源。
Peer 成功直接返回；只有 KEY_NOT_FOUND 是最终业务缺失。GROUP_NOT_FOUND、无类型 404、网络/解码故障等尝试本地回源。

Peer 接口只调用 GetLocal：本地缓存/Bloom → originFlight → 二次查缓存 → 获取源名额 → Getter → 缓存结果。它不选 Peer，所以成员视图不同不会产生 A→B→A 转发环。

routeFlight 与 originFlight 是独立实例，禁止同一 SingleFlight 同 Key 递归，也不为超时调用者调用 Forget。

## 取消与预算

每个调用者响应自身 context。共享工作使用进程生命周期派生的有限 deadline，不继承首个等待者的取消；等待者离开不会取消其他等待者。

路由到期停止等待；独立源工作可继续到自己的 deadline，并在成功后缓存。源 deadline 从 originFlight 闭包开始，包含排队；同 Key 等待者不重复占名额。源名额由进程中的所有 Group 共用。

Peer client 复用连接并设超时，客户端和服务端分别校验 Value 1MiB 与 Protobuf body 1MiB+1024 的边界。URL path 统一转义/还原，支持空格、加号、斜杠和中文。

## 生命周期与观测

监听端口全部绑定后才进入就绪。退出时先取消就绪、关闭 listener 并排空在途请求，再取消源共享工作。基础指标按实际 Peer 请求与 Getter 执行计数，合并等待者不计为额外回源。

当前没有 EndpointSlice watch 或无锁原子快照；它们属于后续成员管理改造。Operator 的资源/权限/Status 完善同样留在后续阶段。
