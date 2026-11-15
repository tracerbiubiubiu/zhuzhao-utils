# 开发与提交

本库最低支持 Go 1.26.11，推荐开发工具链 Go 1.27.4（`go.mod` 的 `toolchain`）；消费方不因工具链推荐被要求采用 1.27 语言特性。Go 1.26 已装环境可用 `GOTOOLCHAIN=local` 验证最低线，ordinary 档 CI 分别使用1.26.11/1.27.4运行vet、格式与race。依赖与消费者基线见 [版本升级记录](../ops/version-upgrade.md)。

分档入口依赖 Git、Python 3 与 Make；仅 ordinary 产品检查需要上述 Go 工具链。

```sh
make hooks
make quality-plan                 # 先审阅文件范围、档位与计划
make quality-check                # 文档/针对检查；ordinary提示手工完整门禁后退出
# 基线不是 origin/main 时显式指定：make quality-check BASE=<本地已存在的基线ref>
```

CI（`.github/workflows/ci.yml`）保持所有分支push、v* tag push与PR触发；按下述计划选择产品检查，独立commit-policy与最终quality-gate始终运行。

## 分档门禁

`make quality-plan`只输出计划，`make quality-check`执行同源分类及document/reference检查；ordinary档退出并指向指南中的原门禁，须在独占环境手工执行，不自动连接服务或把打印清单当验证通过。两者默认`BASE=origin/main`，对基线到HEAD的完整差异加暂存、未暂存、未跟踪文件分类，避免只看末次提交。正式push/PR由事件精确SHA窗口分类；base缺失强制ordinary并披露无法历史比较，异常计划或候选不一致拒绝，不猜测成文档豁免。

- `document`：纯说明、设计草稿、报告与导航，仅跑文档/本地链接检查及始终启用的提交规范；不声称运行产品测试。
- `reference`：候选Schema、规划JSON、已隔离参考或工具，增加受影响文件的Schema/JSON/工具语法与专属验证；不能据此认定产品集成已通过。
- `ordinary`：业务代码、运行配置、依赖、迁移、正式协议、产品测试、CI、分类器、门禁/豁免规则及未知文件，运行完整适用产品门禁；混合改动取最高档。门禁改动自身也在此档，测试先行纪律保持。

分类器以仓内明确路径规则判定，文件扩展名本身不豁免。`document-check`始终执行；`reference-check`按计划执行；原`test` job只在`runtime=true`执行。最终`quality-gate`以`always()`汇总实际job结果：应跑项缺失、失败、取消或skipped均拒绝，计划外产品job的skipped须明确。必需检查策略是否已在远端启用需另核，不能从本地配置推断。

完整产品门禁保留以下既有命令与环境要求：

- `make lint`（含vet/gofmt/test）、`make test`、`make build`入口保留。
- CI在Go 1.26.11与1.27.4分别执行`go vet ./...`、`test -z "$(gofmt -l .)"`、`go test -race ./...`及`go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -test ./...`；本地完整验证也须覆盖两条支持线。

## 提交规范


遵 zhuzhao `docs/standards.md` §12-9 / §12-10；关联主仓[2026-11-08复核报告](https://github.com/tracerbiubiubiu/zhuzhao/blob/phase5/docs/review/reports/zhuzhao-report-followup-2026-11-08.md)的 **RR-GIT-01**。链接指向当前维护分支，报告中的固定提交与验证范围仍按原时点解读。
本仓脚本与主仓同源、自包含，仅依赖 Git + Python 3 标准库，不依赖主仓目录或新增 Go/npm 包。
主题为 `type(scope): 摘要`（scope 可省略），摘要去前缀后最多 50 个 Unicode 字符、不得以句号结尾；
主题与正文间须空行，正文不得为空或只有空白。所有新 merge 提交也须显式提供规范主题与正文。
修复正文仍按公约填写「改动摘要 / 影响面 / 验证证据」与同类扫描，硬检查①–⑤不替代人工验证证据。

```sh
make hooks              # 新 clone / 新工作树开工时安装
make commit-lint        # origin/当前分支..HEAD；初推无同名远端时从启用基线检查
make test-commit-lint    # 临时独立仓库黑盒反例，不触碰业务栈
# 显式历史审计（完整 SHA；不豁免历史）
python3 scripts/commit-policy.py range --before <完整SHA> --after <完整SHA> --strict
```

`make hooks` 启用 Git 的 worktree 配置，仅在当前工作树设置相对 `.githooks`；common
`core.hooksPath` 保留，尚未承接本批的 main/其它工作树不被改装。保留在途显影与跨主题纯重题
amend 保护，`commit-msg` 先挡消息违规，`pre-push` 按 Git 提供的每个 ref 新旧 SHA 检查完整窗口。
`GIT_INDEX_FILE` 保持原值，兼容显式 pathspec 提交，不暂存或重写其它会话文件。

固定启用基线见 `scripts/commit-policy-baseline`：仅其祖先为已推送存量，不改史、不自动前移。
CI push 使用事件 before/after，PR 使用真实 base/head SHA，初推与 force push 均有覆盖；
缺对象精确获取仍失败、非法 SHA 或浅克隆一律拒绝，不回退近 20 笔。产品测试命令保留，按本页分档计划运行。
