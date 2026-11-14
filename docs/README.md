# zhuzhao-utils文档导航

> **用途**：按使用、开发或版本核验选择入口。**设计状态**：现行公共包与R07目标合同分列。**实施状态**：本仓源码、正式tag、消费者pin及部署分别核对，目标词典不表示已发布新API。

## 当前公共包与接口

先读[接入指南](guides/usage.md)，再查[包/API](api/README.md)及[AK/SK协议](api/aksk.md)。函数签名以包源码/Go doc为准，消费者以实际不可变版本为准；模块和嵌入资源路径保持原用途。

## 目标设计与计划

[平台公共机制合同](api/platform-contracts.md)承接R07/P7的机制边界、调用/身份兼容和消费者版本。统一能力及词典按主仓[共同合同](../../zhuzhao/docs/design/capability-invocation-contract.md)；排期、状态和新增范围回[唯一总账](../../zhuzhao/docs/review/planning-backlog.md)，不在本仓复制预算或预建shared-core。

## 开发与运行验证

[开发门禁](guides/development.md)给工具链、测试及提交，[AGENTS](../AGENTS.md)给全仓协作约束。库单测不替代签名/序列化两端和实际消费者的组合验证；本库不建立空服务部署、业务模块或阶段目录。

## 发布与历史

使用新接口前核[版本/tag与pin](ops/releases.md)，工具链候选和消费者基线看[升级记录](ops/version-upgrade.md)。已发布版本、未发布源码候选和实际运行版本不能混称；旧tag与固定报告保留其时点含义。
