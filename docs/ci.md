# CI 检查范围

PR 和 main push 使用相同的路径规则；发版 tag 仍运行完整构建矩阵。
普通文档修改不触发构建和专项验收。nightly fuzz、Homebrew 和发布同步任务保持独立。

## 基础检查与专项验收

| 检查 | 相关改动 |
| --- | --- |
| 三平台 Go 测试、race、JSON golden、全平台构建 | Go 文件、testdata、依赖、PGO、WinDivert 嵌入资源和各自配置；构建另包含嵌入的 Web 资源 |
| lint | Go 文件、testdata、依赖、PGO 及 lint 配置 |
| TOS、fwmark | trace、routeprobe、公共配置及工具、探测 CLI、MTR JSON / RAW 输出契约、WinDivert 资源、各自验收脚本 |
| MTR PTY / ConPTY | MTR CLI、printer、hoprender、mtrsession、trace、公共初始化依赖、所有 PTY 脚本及共享屏幕模型 |
| Doctor 集成 | Doctor、routeprobe、trace、公共初始化依赖及 CLI 入口 |
| DNS q 对照 | dnsclient、DNS CLI、公共 CLI 注册和配置 |
| 安装验收 | nt_install.sh、scripts/test_nt_install.sh |
| WebUI JS | server/web 下资源及测试 |
| Shell 语法 | .cross_compile.sh、scripts/perf 下 shell 脚本 |
| Runtime Regression | CLI、探测、输出、Geo/DNS、Web/API、内部共享包及三平台回归脚本 |
| 性能证据 | 现有 benchmark 包及其仓库内依赖、嵌入 Web 资源、性能脚本 |

专项按包保守匹配 Go 文件与 testdata，不分析函数或 diff 内容。公共配置或探测后端变化仍会运行多个专项。
性能 benchmark 的依赖包含 printer 和 Web/API 共享实现，因此这些包的变化仍运行性能证据。

`go.mod`、`go.sum`、`default.pgo`、`.cross_compile.sh`、共享筛选规则或筛选脚本变化会触发全部相关检查。
单个 workflow 变化只触发自身，不再使用 `.github/workflows/*.yml` 扩散到其他 workflow。

## 修改规则

路径策略保存在 `.github/ci-paths.json`。独立 workflow 使用原生 `paths`，没有相关改动时不创建运行。
Test workflow 内的专项由 `scripts/ci/select_checks.py` 选择：只改 PTY 脚本时运行三平台 PTY，
只改安装脚本时运行 Linux/macOS 安装验收，只改 Web 资源时不启动 Go 测试矩阵。
Go 源码修改始终保留三平台基础测试、race 和 JSON golden 检查。

修改策略后同步原生路径过滤，并验证：

```sh
python3 scripts/ci/select_checks.py --sync-workflows
python3 scripts/ci/select_checks.py --check-workflows
python3 -m unittest discover -s scripts/ci -p 'test_*.py' -v
```

Test 每次运行先执行上述规则校验。新增专项依赖、脚本或嵌入资源时同步补充策略与分类用例。

## 差异与手动验收

PR 比较 merge-base 到 PR head 的完整差异；push 比较事件 before/after，包含全部提交。
关闭 Git rename 检测以同时保留旧、新路径；删除文件同样参与分类。
无法读取事件、提交或差异时，Test 记录原因并完整运行，不把失败当作无改动。

在 Actions 中手动运行对应 workflow 可忽略路径过滤，完整执行该 workflow 的检查。
Test 的手动运行启用所有基础检查及专项步骤。原生路径过滤受 GitHub diff 限制，
涉及大量文件或无法确认覆盖时应额外手动完整运行相关 workflow。
