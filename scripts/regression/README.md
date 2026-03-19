# Regression Scripts

三平台一键回归脚本放在这个目录：

- `linux.sh`
- `macos.sh`
- `windows.ps1`

## 用法

Linux：

```bash
cd /path/to/NTrace-dev
scripts/regression/linux.sh
```

macOS：

```bash
cd /path/to/NTrace-dev
scripts/regression/macos.sh
```

Windows：

```powershell
Set-Location 'C:\path\to\NTrace-dev'
powershell -ExecutionPolicy Bypass -File .\scripts\regression\windows.ps1
```

说明：

- Windows 运行态回归目前面向本机管理员环境
- 在 GitHub-hosted Windows runner 上，脚本会先做能力探测；不能稳定运行的项目会记为 `SKIP`

## 自定义产物目录

Linux / macOS：

```bash
ART_ROOT=/tmp/ntrace-regression-custom scripts/regression/linux.sh
ART_ROOT=/tmp/ntrace-regression-custom scripts/regression/macos.sh
```

Windows：

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\regression\windows.ps1 -ArtifactsRoot C:\Temp\ntrace-regression
```

## 脚本会做什么

- 构建当前仓库的 `nexttrace`、`nexttrace-tiny`、`ntr`
- 先跑 `go test ./...`
- 再跑完整回归
- 输出：
  - `summary.tsv`
  - `artifacts/`
  - `bin/`

## 前置条件

Linux / macOS：

- 已安装 `go`
- 已安装 `python3`
- 已安装 `curl`
- 需要 `sudo`
- 如果要跑抓包校验，需要 `tcpdump`
- 如果要验证 MTU TTY 彩色输出，需要 `script`
- 如果机器没有 IPv6，脚本会把 IPv6 相关检查记为 `SKIP`

Windows：

- 已安装 `go`
- 需要管理员 PowerShell
- 如果要跑抓包校验，需要 `tshark`
- `mtu_tty_color` 在 Windows 下固定记为 `SKIP`，当前脚本不做真实 PTY 捕获

## 输出位置

Linux：

- 默认输出到 `/tmp/ntrace-regression-linux-<timestamp>/`

macOS：

- 默认输出到 `/tmp/ntrace-regression-macos-<timestamp>/`

Windows：

- 默认输出到 `%TEMP%\ntrace-regression-windows-<timestamp>\`

## 结果说明

- `PASS`：该项通过
- `FAIL`：该项失败
- `SKIP`：当前机器缺少可选依赖，或当前环境没有 IPv6，相关检查跳过

脚本结尾会打印：

- `__SUMMARY__`
- `pass/fail/skip/total`
- `artifacts_root=...`

只要存在 `FAIL`，脚本退出码就是非 0；只有 `PASS` / `SKIP` 时才返回 0。
