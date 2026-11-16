# AGENTS.md — zhuzhao-utils协作入口

适用于本仓全部文件和每次AI/人工协作。长规则只维护权威来源，本页不另立一套公约。

## 文档与工作资料归属

- 新增或移动文档前，先按[文档维护约定](../zhuzhao/docs/documentation.md)确定唯一归属、Git保留范围及入口；详细规则只维护该页。
- 可复用方法、空白模板和开发提示词从[工作资料库](../zhuzhao/docs/toolkit/README.md)查找并复用；填写后的执行实例按正式结论或本地过程材料归档，不能回填到模板原件。
- 当前计划归主仓planning，合同/指南归实现仓docs，正式结论归review/reports；原始调研、日志、截图、备份和操作收据按[材料目录](../zhuzhao/docs/materials.md)留仓库外档案。ignored `.codex/`仅作执行工作区，收尾提炼结论并登记材料ID。
- 不在src下新建散落报告目录或第二套长期知识库；模板/提示词不扩大操作授权，也不改变本仓门禁。迁移须检查链接、脚本和真实消费者，收尾核Git状态及材料去向。

## 职责与权威

- 本仓只提供签名、令牌、错误/响应、日志、连接构造和纯校验机制；不收业务表、授权策略事实、目录或运行状态机。
- [文档导航](docs/README.md)区分现行包/API、目标合同、验证与发布历史；准确函数签名以包源码/Go doc和消费者实际pin为准。
- 全生态工程、API、安全和测试纪律遵[主仓standards](../zhuzhao/docs/standards.md)；边界见[平台公共机制合同](docs/api/platform-contracts.md)，计划归[唯一总账](../zhuzhao/docs/planning/planning-backlog.md)。
- 目标词典/公共信封不表示新SDK、wire字段或版本已发布；候选源码、正式tag与实际消费者部署分别核验。

## 开发与验证

- 开工先`make hooks`，核对分支、工作树和在途文件；写任务独立worktree，只有真实重复消费与稳定合同才提取公共机制。
- 保留standards §11测试先行：当前批准批全量正/负/边界/安全拒绝用例先写，取得目标行为的红证据，再分批实现到绿。
- 修复必带回归；并发修复遵§9/§12移除修复后连续至少8次稳定变红，不以`-race`全绿替代；红证据不等于允许提交红测试。
- 从仓库根目录先跑`make quality-plan`审阅分档，再跑`make quality-check`；ordinary档须按输出在独占环境手工跑原完整门禁并留证，不以计划输出代替通过。`BASE`默认`origin/main`，本地含base到HEAD及暂存/未暂存/未跟踪文件。分类规则和完整产品验证命令见[开发指南](docs/guides/development.md#分档门禁)。
- 纯说明/设计/报告走文档检查；候选Schema/规划JSON/隔离参考或工具加针对验证；业务、运行配置、依赖、迁移、正式协议、产品测试及CI/分类器/豁免规则走完整适用门禁。混合取最高档，未知从严；测试先行不变。
- 最低/推荐Go版本以`go.mod`及[开发指南](docs/guides/development.md)为准；验证最低支持线和实际消费者，不因推荐工具链静默抬最低语言版本。
- 公共API/签名/序列化变化先改合同和向量，再验证生产端/消费端及兼容；不得以库单测绿代替跨仓验证。
- 完整适用门禁未绿不标完成；首次失败、重试、skip及NOT_RUN如实列出，不改断言、删语料或降阈值制造绿灯。

## 交付与边界

- 交付固定三节：改动摘要、影响面、验证证据；沿用[短审阅卡](../zhuzhao/docs/phase6/implementation-handoff.md#单维护者短审阅卡)，同步API/版本边界及主仓控制台。
- 仅提交本批明确路径，使用`git commit -F <消息文件> -- <paths>`等显式路径形态；禁`git add -A`、`git commit -a`和裸提交。
- 提交前核归属，提交后核`git show --stat`；全绿后提交，不擅自amend/reset/rebase他人历史。
- 合入走已确认的PR/单维护者策略及实际生效规则；默认不push、合main或发tag，不声称未启用的远端保护已生效。
- 不将模型/MCP厂商SDK、业务payload或通用编排塞入utils；新增依赖说明理由、替代方案和对消费者的影响。
- 文档、代码、mock均不得引入公司真实名称、人员、接口、地址或凭据；私有SDK留connector，秘密不入Git/日志。
- 不为仓库改名顺手变module/import；不重写旧tag，发布与消费者升级须有准确不可变版本和回执。

接入看[用法](docs/guides/usage.md)，版本可用性和pin看[发布记录](docs/ops/releases.md)；本仓不建立空服务部署、阶段或业务模块目录。
