# yatori-go-console 重构计划（2026-04）

## 1. 这次排查到的现状

本地仓库位置：`/home/yibi/yatori/yatori-go-console`

### 1.1 结构层面的问题

当前 Go 代码主要散落在这些目录：

- `main.go`
- `init/`
- `logic/`
- `web/`
- `dao/`
- `entity/`
- `global/`
- `utils/`
- `config/`
- `examples/`
- `command/`

其中有几个明显的“越层调用 / 责任混杂”点：

1. **入口过薄，但业务总入口过胖**
   - `main.go` 只有几行，但 `logic/Lunch.go` 承担了：
     - 配置文件存在性检测
     - 首次初始化交互
     - 默认配置修正
     - 日志初始化
     - Web 模式启动
     - 多平台任务编排
   - 等于把“启动流程 + 配置管理 + 调度器 + 运行器”全部塞在一个文件里。

2. **`logic/Lunch.go` 直接硬编码所有平台依赖**
   - 目前直接 import：
     - `logic/cqie`
     - `logic/enaea`
     - `logic/haiqikeji`
     - `logic/icve`
     - `logic/ketangx`
     - `logic/qingshuxuetang`
     - `logic/welearn`
     - `logic/xuexitong`
     - `logic/yinghua`
     - `web`
   - 这是典型的“总线式 God file”。以后每加一个平台，几乎都要改这个文件。

3. **`global/` 是共享状态桶**
   - `global.GlobalDB`
   - `global.UserActivityMap`
   - `global.AccountTypeStr`
   - 这会导致：测试困难、并发风险、生命周期不清晰、调用链不可追踪。

4. **`web/` 目录职责混杂**
   - `web/ServerInit.go` 同时做：数据库初始化、调度启动、Gin 路由、静态资源处理、中间件。
   - `web/service/UserService.go` 既碰 DAO、也碰配置、也碰 activity、也碰 global。
   - `web/activity/` 这个命名也比较模糊，本质上更像“运行时状态/会话状态”。

5. **配置、运行状态、存储没有边界**
   - 配置来自 `config.yaml`
   - Web 又会写本地配置 / SQLite / activity 状态
   - CLI 和 Web 模式共享同一批全局对象，但没有统一的 runtime/container。

6. **目录命名不统一**
   - `init`、`logic`、`dao`、`entity` 是偏旧式 Java 风格
   - `web/service/UserService.go`、`dao/UserMapper.go`、`entity/pojo`、`entity/vo`、`dto` 混在一起
   - Go 项目更适合按领域/能力分层，而不是 PO/DTO/VO 横向铺开

7. **仓库混入运行产物/示例/发布辅助文件**
   - `web/service/yatori.db`
   - `command/config.yaml`
   - `command/start.bat`
   - `examples/*.go`
   - `assets/web/` 为前端构建产物
   - 这些内容需要重新定义：哪些是源码、哪些是分发文件、哪些是开发测试资源。

### 1.2 这次快速统计到的耦合热点

- Go 文件总数：**47**
- import 扇出最高：`logic/Lunch.go`（内部依赖 **12** 个）
- fan-in 最高的包：
  - `config`：18
  - `utils`：15
  - `global`：14

这基本说明：

- `config` 已经不只是配置，实际上承担了很多“基础能力”入口。
- `utils` 变成杂项回收站。
- `global` 是跨层耦合核心。

---

## 2. 重构目标

目标不是一次性“大搬家”，而是分阶段把项目变成下面这种结构：

1. **入口清晰**：CLI / Web 启动分离
2. **运行时清晰**：配置、数据库、调度器、平台 runner 由统一 `app/runtime` 管理
3. **平台扩展清晰**：每个平台实现统一接口，新增平台不需要改一堆 switch / import
4. **Web 边界清晰**：HTTP、service、repository、runtime state 分层
5. **全局变量最小化**：尽量把 `global` 收敛到依赖注入
6. **目录语义清晰**：源码、构建产物、示例、分发模板拆开
7. **可测试**：平台调度、配置加载、账号更新、Web handler 至少能做单元测试

---

## 3. 建议的目标目录结构

> 这是目标结构，不是一步到位；可以逐步迁移。

```text
cmd/
  yatori/
    main.go            # 标准启动入口

internal/
  app/
    bootstrap/
      cli.go           # CLI 启动流程
      web.go           # Web 启动流程
    runtime/
      runtime.go       # AppRuntime: config/db/logger/scheduler/registry
      paths.go         # 路径与数据目录
  config/
    loader.go
    validator.go
    defaults.go
    prompts.go         # 首次启动交互
  platform/
    registry.go
    runner.go          # 统一接口
    yinghua/
    xuexitong/
    enaea/
    cqie/
    ketangx/
    welearn/
    icve/
    qingshu/
    hqkj/
  service/
    execution/
    account/
    localconfig/
  repository/
    sqlite/
      user_repository.go
  web/
    server.go
    middleware/
    handler/
    dto/
    state/             # 原 web/activity 迁移到这里
  notify/
    email/
    sound/
  pkg/
    logx/
    osx/
    slicesx/

docs/
  architecture/
  refactor-plan-2026-04.md

test/
  integration/

build/
  windows/
  templates/
    config.yaml
```

如果你不想一下子引入 `internal/`，也可以做一个过渡版：

```text
cmd/
app/
platform/
web/
repository/
service/
config/
pkg/
```

但从长期维护看，`internal/` 更稳。

---

## 4. 分阶段重构方案

## Phase 0：先做“止血整理”，不改行为

### 目标
先把最乱但低风险的部分分出来，保证后面重构不会越做越乱。

### 动作
1. 新增 `docs/architecture/` 和本计划文档
2. 把运行产物和模板分开：
   - `command/config.yaml` → `build/templates/config.yaml`
   - `command/start.bat` → `build/windows/start.bat`
   - `web/service/yatori.db` 从仓库源码目录移走，改到运行时数据目录
3. 增加 `.gitignore` / 运行目录规范：
   - `data/`
   - `tmp/`
   - `assets/log/`
   - `assets/web/`（如果决定不提交构建产物）
4. 统一命名基线：
   - 新文件一律 snake_case
   - 避免再新增 `XXXService.go` / `UserMapper.go` 这种 Java 风格命名

### 交付物
- 仓库内“源码 / 模板 / 运行数据 / 构建产物”四类内容分开

---

## Phase 1：拆启动流程，把 `logic/Lunch.go` 变薄

### 目标
把“应用启动”从“大函数”拆成可读的步骤。

### 动作
把 `logic/Lunch.go` 拆成几个职责明确的函数/文件：

1. `internal/app/bootstrap/bootstrap.go`
   - `Run()`
2. `internal/config/loader.go`
   - 加载 config.yaml
   - 检测默认配置
3. `internal/config/prompts.go`
   - 首次启动交互生成配置
4. `internal/config/validator.go`
   - 校验 URL / 账号 / 模式
5. `internal/app/runtime/runtime.go`
   - 日志初始化
   - 数据目录初始化
6. `internal/app/bootstrap/mode.go`
   - 决定走 CLI 还是 Web

### 这一阶段完成后
`main.go` 最好只剩：

```go
func main() {
    bootstrap.Run()
}
```

---

## Phase 2：做平台注册表，去掉 `logic/Lunch.go` 对所有平台的硬编码

### 目标
新增平台时，不再修改总调度文件。

### 统一接口建议

```go
type Platform interface {
    Name() string
    AccountType() string
    FilterUsers(cfg *config.Config) []config.User
    Login(users []config.User) any
    Run(setting config.Setting, users []config.User, session any)
}
```

或者更 Go 一点：

```go
type Runner interface {
    AccountType() string
    Run(ctx context.Context, app *runtime.AppRuntime, users []config.User) error
}
```

### 动作
1. 建 `platform/registry.go`
2. 每个平台包实现统一 Runner
3. 启动时遍历 registry，并发执行
4. 把 `platformLock` 收敛到统一调度器里

### 收益
- 新平台接入只需注册
- 并发和错误收集统一管理
- 以后可以支持按平台禁用/重试/超时控制

---

## Phase 3：消灭 `global`，引入 `AppRuntime`

### 目标
把全局状态改成显式依赖。

### 建议结构

```go
type AppRuntime struct {
    Config        *config.Config
    DB            *gorm.DB
    Logger        Logger
    ActivityStore ActivityStore
    Scheduler     *Scheduler
    Paths         Paths
}
```

### 动作
1. `global.GlobalDB` → `AppRuntime.DB`
2. `global.UserActivityMap` → `web/state.Store`
3. `global.AccountTypeStr` → `platform/metadata.go`
4. service / handler / runner 通过参数拿 runtime

### 收益
- 单元测试可 mock
- 生命周期可控
- 不会再因为包级变量到处被写而难追踪

---

## Phase 4：重构 Web 层，按 HTTP / Service / Repository 分层

### 当前主要问题
`web/ServerInit.go` 和 `web/service/UserService.go` 都太胖。

### 目标结构

```text
internal/web/
  server.go
  routes.go
  middleware/
  handler/
    user_handler.go
    config_handler.go
  dto/
  state/

internal/service/
  account/
  config/
  scheduler/

internal/repository/sqlite/
  user_repository.go
```

### 动作
1. `server.go` 只负责启动 HTTP server
2. `routes.go` 只注册路由
3. handler 只处理 HTTP 输入输出
4. service 承担业务逻辑
5. repository 专门读写 SQLite
6. `web/activity` 改名为 `web/state` 或 `runtime/activity`

### 额外建议
当前 `LoggerMiddleware()` 会打印整个请求体，后面最好加：
- 请求体大小限制
- 敏感字段脱敏（密码、cookie、token）

---

## Phase 5：整理配置模型，减少 `config` 包大杂烩

### 当前问题
`config` 既包含：
- 配置结构体
- Logo
- 交互输入
- 配置读取
- 字符串转数字等工具

### 目标
拆成：

```text
internal/config/
  types.go
  loader.go
  defaults.go
  validator.go
  interactive.go
  logo.go   # 如果还需要
```

### 规则
- `config` 只负责“配置相关”
- 通用工具迁出到 `pkg/`
- Logo/公告这种 UI 展示内容不要和配置逻辑强耦合

---

## Phase 6：清理 `utils` 和 `entity` 的杂糅设计

### `utils` 建议拆分
把杂项工具按能力迁移：

- `utils/EmailUtils.go` → `internal/notify/email/`
- `utils/AnnouncementUtils.go` → `internal/app/bootstrap/announcement.go`
- `utils/IpProxyFileUtils.go` → `internal/service/proxy/`
- `utils/regedit_*` → `pkg/osx/`
- `utils/ObjectUtils.go` → 如果只是零散 helper，要么删掉，要么归入明确小包

### `entity` 建议拆分
当前 `pojo/dto/vo` 是 Java 风格。

建议改成：
- 持久化模型：`repository/sqlite/model/`
- HTTP 输入输出：`web/dto/`
- 内部业务对象：直接跟随 service 或 platform 定义

---

## Phase 7：测试与回归保护

### 先补的测试
1. 配置加载/默认配置识别
2. 配置校验逻辑
3. Web 用户更新/删除/新增接口
4. 平台注册表筛选逻辑
5. Web 模式启动 smoke test

### 工具链建议
- `go test ./...`
- `golangci-lint`
- `staticcheck`
- 如果前端也要一起收敛，再加：
  - `pnpm lint`
  - `pnpm typecheck`

---

## 5. 建议的落地顺序

按风险和收益排序，我建议你这样做：

### 第一批（低风险，高收益）
1. 清理目录语义：`command/`、`examples/`、运行产物位置
2. 写清楚目标结构文档
3. 拆启动流程（不改业务行为）

### 第二批（中风险，最高收益）
4. 平台注册表
5. `AppRuntime` 替代 `global`
6. Web 三层拆分

### 第三批（中高风险）
7. `config`/`utils`/`entity` 体系重整
8. 补测试和 lint
9. 再决定是否继续拆前端构建/静态资源发布流程

---

## 6. 我建议优先动的几个文件

如果现在就开始下手，我会先改这几个：

1. `main.go`
2. `logic/Lunch.go`
3. `web/ServerInit.go`
4. `web/service/UserService.go`
5. `global/global.go`
6. `config/Config.go`
7. `dao/*.go`

原因很简单：这几处是当前耦合中心，先把这些拆薄，后面平台文件才能稳定迁移。

---

## 7. 一句话版本

这个项目现在的问题，不是“代码不够多”，而是：

- 启动流程、平台编排、Web 服务、配置管理、全局状态全都交叉在一起；
- 目录按历史习惯长出来了，但没有稳定边界。

所以最优解不是直接全量重写，而是：

**先拆启动、再做平台注册、再去全局变量、最后重整 Web / config / utils。**

这样风险最低，且每一步都能持续可运行。
