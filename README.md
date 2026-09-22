# WorkBuddy2API Desktop (桌面增强版)

WorkBuddy2API Desktop 是一个面向腾讯 CodeBuddy 多账号管理与高性能 API 聚合网关的 Windows 桌面增强客户端。通过将多账号包装为统一兼容的 OpenAI / Responses / Messages 接口，配合 Tauri v2 打造的无控制台后台静默守护、系统托盘菜单以及现代化石墨暗黑 Web 管理面板，为开发者提供开箱即用、无感运行的桌面级服务。

---

## 核心特性

1. 无控制台静默后台运行
- 采用底层 CREATE_NO_WINDOW 机制消除黑色命令提示符窗口，应用随开即用，完全常驻后台运行，避免误关或窗口遮挡。

2. 全功能 Windows 系统托盘守护
- 单击托盘图标：快速显示或隐藏主控制面板窗口；
- 托盘右键菜单：支持打开面板、默认浏览器访问 WebUI、启停核心服务、平滑重启服务、查看版本信息、打开日志及检查更新。

3. 现代化石墨暗黑 Web 管理控制台
- 针对 Windows 与多浏览器深度定制极细半透明圆角滚动条（4px~5px），彻底消除系统原生粗白灰条；
- 重新设计的深色卡片布局与平滑微动效，自适应浅色/深色主题；
- 原生内置「关于与版本」看板，一目了然掌握宿主、内核、端点与协议状态；
- 全局断线与服务重启优雅遮罩（#reconnectShield），服务更新或重启时自动展示磨砂缓冲层并实时探活，恢复后无缝恢复数据。

4. 自动化增量静默更新
- 启动后自动检查 GitHub 官方 Releases 最新版本；
- 采用局部增量替换机制，仅更新二进制核心，完整保护本地账号凭证 (./auths) 与用户配置 (config.json) 不被覆盖。

5. 完整的 API 兼容与账号池治理
- 兼容 OpenAI (/v1/chat/completions)、Responses 与 Messages 三套主流协议；
- 支持三因子智能权重选号、熔断退避、软限流保护与会话粘性保持。

---

## 系统架构与运行机制

应用由两层构成：
- 桌面宿主层 (Tauri v2 + Rust)：负责管理生命周期、捕获窗口事件、托盘右键交互、无窗口子进程拉起与后台静默探活；
- 核心网关层 (Go 1.22.5)：单二进制文件 (wb2api.exe)，内嵌经过深度美化与强化的全套 Web 控制台静态资源，监听 127.0.0.1:7863 提供 HTTP API 与管理面板。

---

## 快速使用

### 运行方式
直接运行发布包中的 wb2api-panel-desktop.exe：
- 程序启动后将自动静默唤起 wb2api.exe 并最小化驻留于系统托盘；
- 自动打开主面板窗口，地址为 http://127.0.0.1:7863/panel/；
- 如关闭主窗口，程序将继续在托盘静默运行；需要完全退出时，在托盘图标右键选择「退出」即可。

### 常用操作
- 查看日志：托盘右键选择「查看运行日志」，自动调用记事本打开 logs/wb2api.log；
- 重启服务：托盘右键选择「重启核心服务」，界面平滑展示过渡层并在内核就绪后自动重新加载；
- 浏览器访问：托盘右键选择「在浏览器中打开 WebUI」，将在系统默认浏览器打开完整控制台。

---

## 项目构建指南

### 环境准备
- Go 1.22 或更高版本
- Rust 1.77+ 与 Cargo
- Windows 10 / 11 64位系统环境

### 编译步骤

1. 编译 Go 内核
`powershell
go build -ldflags="-s -w" -o wb2api.exe ./cmd/server
`

2. 编译 Tauri 桌面端
`powershell
cd desktop/src-tauri
cargo build --release
`

编译生成的桌面客户端位于 desktop/src-tauri/target/release/wb2api-panel-desktop.exe。将该文件与 wb2api.exe 放置于同级目录即可运行。

---

## 开源许可证与致谢

- 本项目采用 MIT 许可证开源。
- 特别鸣谢上游项目：
  - [linguo2625469/workbuddy2api-panel](https://github.com/linguo2625469/workbuddy2api-panel)
  - [Sliverkiss/workbuddy2api](https://github.com/Sliverkiss/workbuddy2api)
