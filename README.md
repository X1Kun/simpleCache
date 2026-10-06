# SimpleCache — 中间版本 1

当前阶段：结构整理。本版本只包含到第 1 阶段的功能，后续完整成果保存在仓库外，见 [状态记录](docs/REFACTOR_STATE.md)。

合并引擎 module、移动源码、提取启动函数、更新 import/Dockerfile/检查入口，归档旧实验；保持基线 Getter、Peer、SingleFlight、哈希和 Operator 行为。

## 当前结构

- geecache-engine/cmd/simplecache：启动入口。
- geecache-engine/internal/cache：缓存策略与本地存储。
- geecache-engine/internal/peer：节点通信及哈希。
- geecache-engine/internal/demo：有限只读演示源。
- geecache-engine/internal/app：HTTP 服务组织。
- simplecache-operator：保持 Kubebuilder 结构，当前仍为原控制器实现。
- docs/legacy：旧实验源码，不进入默认检查。

## 快速检查

运行 make check。引擎功能/race、两个模块构建和 vet；Operator 原 envtest 不作为本阶段默认检查。
检查已关闭 Go 自动 VCS 查询，不包含 Git 命令，不运行旧负载测试。

## 单节点演示

进入 geecache-engine 后运行：

~~~bash
SELF_ADDR=http://localhost:8001 PEERS=http://localhost:8001 go run -buildvcs=false ./cmd/simplecache -port=8001 -api=true
~~~

读取 http://localhost:9999/api?key=Auto-666。未知 Key 返回 404。
数据为只读演示数据，不是真实数据库，没有写入 API 或强一致性保证。

本版本保留原请求语义：Peer 仍会调用完整 Get，没有新的共享超时/回源上限；这些属于第二阶段，不能将设计目标写成已实现。

Operator 尚未包含完整权限、探针、Status 和子资源监听，旧样例/RBAC 的不足留在第四阶段处理；不要把本阶段的编译通过当作 Kubernetes 部署通过。

提交与发布由用户管理；本轮没有自动创建提交。性能压测与真实 Kind/监控验证仍后置。
