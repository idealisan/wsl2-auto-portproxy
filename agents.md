# agents.md

本文件记录用户的工作偏好，供在本仓库中工作的 AI agent 遵循。来源：用户全局
preferences（user-pref）与用户的明确要求。

## 语言

- 回复、说明与文档使用简体中文（`coding.language = zh-CN`）。

## 临时文件与目录访问

- 临时文件一律放在项目目录下的 `TMP/` 文件夹；目录不存在时创建。
- `TMP/` 加入 `.gitignore`，不提交到 git。
- 不访问项目外的目录，不使用系统临时目录（如 `/tmp`），不读取项目外的模块缓存/家目录内容。
- 不做删除或清理动作（`rm`、`rm -rf`、清理临时产物等），以免触发权限确认提示。
  （`filesystem.temporary_files` + 用户明确要求）

## 构建与打包

- Go 桌面程序发布时构建 3 个操作系统 × 2 种 CPU 架构，共 6 个二进制产物：
  `darwin/amd64`、`darwin/arm64`、`linux/amd64`、`linux/arm64`、
  `windows/amd64`、`windows/arm64`。（`packaging.go.desktop.release_matrix`）
- macOS 软件打包为 `.app` bundle，而不是控制台二进制；除非程序本身就是命令行
  专用工具。（`packaging.macos.prefer_app_bundle`）

## Git 工作流

- 远程仓库是 GitHub 时，每个小变更尽快 commit 并 push，避免变更丢失与大量
  diff 堆积。（`workflow.git.commit_and_push_each_change`）
- commit message 参考本仓库最近 10 个提交的风格，不添加 `Co-authored-by`
  trailer。（`workflow.git.commit_message_style`）

## 发布

- 项目含 GitHub Actions workflow 时，功能完成后发布 Release 以触发 CI 构建。
  （`workflow.github.release_after_feature`）

## 操作记录

- 每个命令或动作执行前先运行 `date '+%Y-%m-%d %H:%M:%S %Z'` 打印当前时间，
  便于回溯操作时间线。（`workflow.print_current_time_before_actions`）
