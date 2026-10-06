# 中间版本 1 的阅读说明

这是“结构整理”阶段。合并引擎 module、移动源码、提取启动函数、更新 import/Dockerfile/检查入口，归档旧实验；保持基线 Getter、Peer、SingleFlight、哈希和 Operator 行为。

从 geecache-engine/cmd/simplecache/main.go 开始阅读，再看 internal/app、internal/cache/group.go、internal/peer 与 internal/demo。

Group.Get、Peer HTTPPool、SingleFlight 与 Getter 的语义保持基线；本阶段只重新组织位置与依赖，不引入新可靠性机制。



Operator 阅读顺序为 api/v1 → internal/controller/simplecache_controller.go。
Operator 保留基线行为，后续修正在第四阶段恢复。

完整改造的架构说明仍保存在仓库外 final/docs/ARCHITECTURE.md；不能将那份目标版本说明误认为本阶段已有能力。
