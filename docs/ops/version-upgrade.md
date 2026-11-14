# 版本升级记录（Phase6 开工前）

最低兼容 Go1.26.10 →1.26.11，推荐工具链 Go1.27.4；不使用1.27专属语言/API，不为工具链更新无理由提高公共库最低语言大版本。CI以GOTOOLCHAIN=local分别验证1.26.11与1.27.4，避免自动下载新SDK掩盖最低线不兼容。

本记录对应 2026-11-12 的独立维护候选，业务能力、API、运行时配置字段与数据库 schema 不变；保留 mock 接口，不启动 Phase6 业务功能。本地验证与后续 PR/部署分开记账。

构建器、运行镜像和 PG/Redis 均固定完整版本与 OCI 多架构摘要；后续更新需重新核官方摘要并跑相应门禁。CI 使用 checkout v7.0.1 / setup-go v7.0.0 的提交 SHA（Node24，托管 ubuntu-latest）；自托管 runner 至少 2.327.1。当前 checkout 的源代码以 govulncheck v1.9.0 扫描；服务另构建当前二进制扫描，不引用其它仓未推送提交。漏洞库会更新，扫描结论仅对当次环境有效；源码可达性与二进制符号扫描边界不同。

依赖按用途更新：pgx5.10→5.13修复协议响应校验、batch及连接重试安全问题；go-redis9.22→9.23（最低Go1.26，新pipeline池）；miniredis2.39→2.40跟进测试服务器；crypto0.58→0.59、sys0.49→0.50、protobuf1.36.10→1.36.12及必要MVS间接依赖。Gin1.12/JWT5.3.1/testify1.12.1等保持已选定稳定版；公共签名、token、错误码、PG/Redis配置结构不变。

消费者矩阵：主仓zhuzhao、activelist、taskrunner原最低Go1.26.10，本次服务候选升级Go1.27.4，均高于本库最低1.26.11；UI无Go消费。服务仍可保持其真实已发布/已推utils pin，通过MVS采用修复后的底层依赖；本库候选尚未发tag，不制造不可下载的新pin。老SDK低于最低线需升级到至少1.26.11，再承接本库候选。

官方依据：[Go发布](https://go.dev/dl/)、[pgx5.13](https://github.com/jackc/pgx/blob/v5.13.0/CHANGELOG.md)、[go-redis9.23](https://github.com/redis/go-redis/releases/tag/v9.23.0)、[miniredis2.40](https://github.com/alicebob/miniredis/releases/tag/v2.40.0)、[govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck)。

## 验证状态

Go1.26.11与Go1.27.4分别以 `GOTOOLCHAIN=local` 完成 `make lint`（含unit）、`make build` 和全包race；每个SDK均142个测试/子测试pass事件、0skip。两个SDK下的当前源码及项目测试（-test）的govulncheck1.9默认text扫描均明确No vulnerabilities found且退出0；最低线不会被自动下载新SDK掩盖。

完整模块图有compress、rsc.io/pdf目录记录，`go mod why`与生产/项目测试包图均确认未导入，已撤回OSV单列；这与当前调用图扫描分开陈述。目录库存按G05每月补丁调查（下一次2026-12）与季度支持版本复核承接，未来新增相关调用前先升级或替换；本轮不为未用依赖抬高SDK或扩展能力。

冷编译资源收敛时的主动中断已留证，随后低内存/单并发最低SDK门禁实际完成。模块校验均通过，公共源码未改，无可执行ELF。日志与模块/候选输入摘要保留在本轮主仓忽略目录 `.codex/audits/version-modernization-20261112/go-siblings/`。当前本地验证为darwin/arm64；CI的Linux双SDK矩阵、发布tag与消费者改pin本轮未执行，不将候选库写成已发布版本。
