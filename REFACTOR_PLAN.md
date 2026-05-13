# Yatori Go Console Refactor Implementation Plan

> **For Hermes:** Use `subagent-driven-development` to implement this plan task-by-task.

**Goal:** 在不破坏现有功能的前提下，逐步把 `yatori-go-console` 从“全局状态 + 大文件 + 多层耦合”重构为“清晰入口、显式依赖、可扩展平台层、可测试应用层”的结构。

**Architecture:** 采用渐进式重构，不推倒重来。先统一启动入口与依赖注入，再消灭 `global`，随后拆分 `web/service` 与平台执行逻辑，最后统一配置/基础设施/HTTP 适配层边界。保留现有功能路径，优先做低风险结构收敛。

**Tech Stack:** Go, Gin, GORM/SQLite, YAML config, `yatori-go-core`

---

## 0. Current Findings

### 0.1 当前代码热点
- `web/service/UserService.go`：约 1413 行
- `logic/xuexitong/XueXiTongPart.go`：约 1274 行
- `web/activity/XueXiTongActivity.go`：约 552 行
- `logic/yinghua/YinghuaPart.go`：约 526 行
- `web/service/local_config.go`：约 497 行

### 0.2 当前关键耦合点
- `global.GlobalDB`
- `global.UserActivityMap`
- `global.AccountTypeStr`
- `web/service/*` 直接依赖 `dao + global + activity + config`
- `logic/*` 与 `web/activity/*` 同时承载平台执行逻辑

### 0.3 当前已存在的半重构痕迹
- 已有 `cmd/yatori/main.go`
- 已有 `internal/app/bootstrap/bootstrap.go`
- 当前分支：`feature/config-manager-and-auto-execution`
- 当前工作区存在未提交修改，实施前先确认是否沿这条线继续

---

## 1. Target Directory Shape

目标不是一次性迁移完，而是逐步往下面结构靠：

```text
cmd/
  yatori/
    main.go

internal/
  app/
    bootstrap/
    execution/
    scheduler/
    user/

  domain/
    model/
    platform/

  platform/
    xuexitong/
      runner.go
      login.go
      course.go
      chapter.go
      exam.go
    yinghua/
      runner.go
      login.go
      course.go
    qingshuxuetang/
    ketangx/
    cqie/
    enaea/
    welearn/
    icve/

  infra/
    db/
    configfile/
    notify/
      email/
      sound/
    logging/

  interfaces/
    http/
      handler/
      router/
      middleware/
```

---

## 2. Refactor Rules

1. **不一次性重写**，每阶段都要保证能编译、能跑、最好能测。
2. **先搬依赖边界，后搬业务逻辑**。
3. **所有新代码禁止继续依赖 `global.GlobalDB` / `global.UserActivityMap`**。
4. **所有平台执行逻辑统一收敛到 runner 抽象**。
5. **大文件拆分优先按职责拆，不按长度机械拆**。
6. **每个阶段都保留验证命令和回滚点**。

---

## 3. Phase Plan

---

### Phase 1: 统一程序入口与启动流程

**Objective:** 让项目只有一个正式启动入口，启动链可读、可注入依赖。

**Files:**
- Modify: `cmd/yatori/main.go`
- Modify: `main.go`
- Modify: `internal/app/bootstrap/bootstrap.go`
- Review/possibly replace: `init/YatoriConsoleInit.go`
- Review/possibly replace: `logic/Lunch.go`
- Review/possibly replace: `web/ServerInit.go`

#### Task 1.1: 确认唯一正式入口
**Step 1:** 让 `cmd/yatori/main.go` 成为正式入口。  
**Step 2:** 根 `main.go` 仅保留薄转发，或标注 deprecated。  
**Step 3:** 检查所有构建脚本、README、部署方式是否仍引用根入口。  

**Verify:**
- `go build ./...`
- 二进制仍可正常启动 Web 服务

#### Task 1.2: 建立 Bootstrap 对象
**Step 1:** 在 `internal/app/bootstrap/bootstrap.go` 定义 `App` 或 `Bootstrap` 结构体。  
**Step 2:** 让它显式持有：
- `*gorm.DB`
- 配置仓库
- activity manager
- scheduler
- http server

**Step 3:** 将启动顺序写成清晰函数：
- `LoadConfig()`
- `InitDB()`
- `InitServices()`
- `BuildRouter()`
- `Run()`

**Verify:**
- `go test ./...`
- 启动日志顺序清晰，不再分散在多个包里

#### Task 1.3: 下沉旧初始化残留
**Step 1:** 清点 `init/YatoriConsoleInit.go`、`logic/Lunch.go`、`web/ServerInit.go` 的职责。  
**Step 2:** 保留真正需要的逻辑，迁入 `bootstrap`。  
**Step 3:** 删除重复调用链，避免“这里 init 一次，那里 server 再 init 一次”。

**Verify:**
- 启动一次只初始化一次 DB/Router/Config
- 无重复日志、无双重 side effects

---

### Phase 2: 消灭全局状态，建立显式依赖注入

**Objective:** 用组件管理器替换 `global` 中的运行时状态与 DB 全局变量。

**Files:**
- Modify: `global/global.go`
- Modify: `web/service/UserService.go`
- Modify: `web/service/auto_execution_schedule.go`
- Modify: `web/service/local_config.go`
- Modify: `web/ServerInit.go`
- Create: `internal/app/execution/manager.go`
- Create: `internal/infra/db/provider.go`

#### Task 2.1: 替换 `global.GlobalDB`
**Step 1:** 新建 DB provider/repository 注入方式。  
**Step 2:** 把 `dao.UpsertUser(global.GlobalDB, ...)` 这类调用改成显式依赖。  
**Step 3:** 将 service struct 化，例如：

```go
type UserService struct {
    db *gorm.DB
}
```

**Verify:**
- `search_files("global\\.GlobalDB")` 结果为 0
- `go test ./...`

#### Task 2.2: 替换 `global.UserActivityMap`
**Step 1:** 新建 `ActivityManager`：

```go
type ActivityManager interface {
    Get(uid string) (platform.Runner, bool)
    Put(uid string, runner platform.Runner)
    Delete(uid string)
    Start(uid string) error
    Stop(uid string) error
}
```

**Step 2:** 用 `sync.RWMutex` 或 `sync.Map` 保证并发安全。  
**Step 3:** 改造 `UserService` / `auto_execution_schedule.go` 调用链。  

**Verify:**
- `search_files("UserActivityMap")` 只剩过渡兼容代码或为 0
- 并发启动/停止不会触发 map 竞争

#### Task 2.3: 收敛平台文案映射
**Step 1:** 将 `global.AccountTypeStr` 挪到更合理的位置，例如 `internal/domain/platform/types.go`。  
**Step 2:** 为 account type 提供常量和显示名方法。  

**Verify:**
- 业务代码不再依赖 `global` 获取平台名称

---

### Phase 3: 拆分 Web Service 为应用服务层

**Objective:** 将 `web/service` 从“超级调度层”拆为边界清晰的 usecase/service。

**Files:**
- Modify: `web/service/UserService.go`
- Modify: `web/service/local_config.go`
- Modify: `web/service/auto_execution_schedule.go`
- Create: `internal/app/user/service.go`
- Create: `internal/app/execution/service.go`
- Create: `internal/app/scheduler/service.go`
- Create: `internal/app/user/config_repository.go`

#### Task 3.1: 抽出 `UserConfigService`
职责：
- 读取/写入 `config.yaml`
- 用户增删改查
- DTO 与内部模型转换

从以下文件迁出：
- `web/service/local_config.go`
- `web/service/UserService.go` 中与配置读写相关部分

**Verify:**
- Controller 不再直接碰配置文件细节
- 配置用户的 CRUD 仍然可用

#### Task 3.2: 抽出 `ExecutionService`
职责：
- 创建 runner
- 启动任务
- 停止任务
- 查询运行状态
- 拉课程列表

从以下文件迁出：
- `web/service/UserService.go`
- `web/service/auto_execution_schedule.go` 中执行控制部分

**Verify:**
- “启动账号 / 停止账号 / 拉课程” 功能仍可用

#### Task 3.3: 抽出 `SchedulerService`
职责：
- 自动执行时间窗校验
- 时间窗命中判断
- 巡检 tick
- 启停托管任务

从以下文件迁出：
- `web/service/auto_execution_schedule.go`

**Verify:**
- 自动执行仍按时间窗工作
- 定时逻辑可脱离 HTTP 层测试

---

### Phase 4: 统一平台执行接口

**Objective:** 将 `logic/*` 和 `web/activity/*` 两套执行模型统一为 runner 抽象。

**Files:**
- Modify: `web/activity/UserActivity.go`
- Modify: `web/activity/XueXiTongActivity.go`
- Modify: `web/activity/YinghuaActivity.go`
- Modify: `logic/xuexitong/XueXiTongPart.go`
- Modify: `logic/yinghua/YinghuaPart.go`
- Create: `internal/domain/platform/runner.go`
- Create: `internal/platform/factory.go`

#### Task 4.1: 定义统一 runner 接口
建议接口：

```go
type Runner interface {
    Login(ctx context.Context) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}

type CoursePuller interface {
    PullCourses(ctx context.Context) (any, error)
}
```

**Verify:**
- Web、调度器、CLI 都能通过相同接口操作平台

#### Task 4.2: 平台工厂统一创建 runner
将 `activity.BuildUserActivity(po)` 这类逻辑逐步收敛到：
- `internal/platform/factory.go`

**Verify:**
- 平台创建逻辑只有一个 authoritative source

#### Task 4.3: 清理过渡层
`web/activity/*` 若只是旧接口兼容层，应最终收敛为：
- 平台 runner 真实实现
- 或极薄适配层

**Verify:**
- 不再出现“web/activity 一套 + logic 一套”的双轨结构

---

### Phase 5: 先重构学习通模块（最高优先级）

**Objective:** 把最大热点 `XueXiTongPart.go` 拆成可维护文件，并形成平台层模板。

**Files:**
- Modify: `logic/xuexitong/XueXiTongPart.go`
- Modify: `web/activity/XueXiTongActivity.go`
- Create: `internal/platform/xuexitong/runner.go`
- Create: `internal/platform/xuexitong/login.go`
- Create: `internal/platform/xuexitong/course_list.go`
- Create: `internal/platform/xuexitong/chapter_study.go`
- Create: `internal/platform/xuexitong/work_exam.go`
- Create: `internal/platform/xuexitong/notify.go`

#### Task 5.1: 抽登录逻辑
把：
- 用户缓存准备
- 代理设置
- cookie/password 登录分支

从大文件里拆到 `login.go`。

#### Task 5.2: 抽课程拉取逻辑
把：
- PullCourseList
- 课程过滤
- 课程遍历入口

拆到 `course_list.go`。

#### Task 5.3: 抽章节学习逻辑
把：
- 章节列表拉取
- 点位检测
- 视频/文档/直播/超链接/BBS 分流

拆到 `chapter_study.go`。

#### Task 5.4: 抽作业/考试逻辑
把：
- AI 可用性检查
- 题库 API 检查
- 作业执行
- 考试执行

拆到 `work_exam.go`。

#### Task 5.5: 抽通知与收尾逻辑
把：
- 邮件通知
- 提示音
- 完成日志

拆到 `notify.go`。

**Verify:**
- 学习通完整流程仍然能登录、拉课、开始执行
- 大文件被实质性瘦身

---

### Phase 6: 套用模板重构英华及其他平台

**Objective:** 让其他平台沿相同 runner 模板演进，降低理解成本。

**Files:**
- Modify: `logic/yinghua/YinghuaPart.go`
- Modify: `logic/qingshuxuetang/QsxtPart.go`
- Modify: `logic/ketangx/KetangxPart.go`
- Modify: `logic/cqie/CqiePart.go`
- Modify: `logic/enaea/EnaeaPart.go`
- Modify: `logic/welearn/WeLearnPart.go`
- Modify: `logic/icve/IcvePart.go`
- Create: `internal/platform/<platform>/*.go`

#### Task 6.1: 统一公共骨架
每个平台至少拆成：
- `runner.go`
- `login.go`
- `course.go`
- `notify.go`

#### Task 6.2: 提炼共享辅助组件
若多个平台重复以下逻辑，可抽共享组件：
- 课程过滤
- 完成通知
- 提示音
- 日志前缀

**Verify:**
- 平台目录结构趋于一致
- 同类 bug 更容易批量修复

---

### Phase 7: 清理数据模型命名与层次

**Objective:** 降低 `dto/pojo/vo` 风格命名的跨层污染。

**Files:**
- Review: `entity/dto/*.go`
- Review: `entity/pojo/*.go`
- Review: `entity/vo/*.go`
- Create: `internal/domain/model/*.go`
- Create: `internal/interfaces/http/request/*.go`
- Create: `internal/interfaces/http/response/*.go`
- Create: `internal/infra/persistence/model/*.go`

#### Task 7.1: 定义内部 canonical model
优先为用户配置、账号信息、运行状态定义内部标准模型。

#### Task 7.2: 仅在边界层做转换
- HTTP 层：request/response
- 持久化层：db model
- 应用层/平台层：domain model

**Verify:**
- Service/Runner 不再到处使用 VO/DTO/POJO 混杂对象

---

### Phase 8: 清理配置、DAO、Utils 边界

**Objective:** 让基础设施职责清晰，不再把 I/O、配置定义、杂项工具搅在一起。

**Files:**
- Modify: `config/Config.go`
- Modify: `dao/SqliteInit.go`
- Modify: `dao/UserMapper.go`
- Review: `utils/*.go`
- Create: `internal/config/*.go`
- Create: `internal/infra/db/*.go`
- Create: `internal/infra/configfile/*.go`
- Create: `internal/infra/notify/email/*.go`
- Create: `internal/infra/notify/sound/*.go`

#### Task 8.1: `config` 只保留配置结构/解析
#### Task 8.2: `dao` 收敛成 repository/infra db
#### Task 8.3: `utils` 只保留真正纯函数

**Verify:**
- 任何带 I/O 的代码都能明确归类到 infra
- `utils` 不再成为垃圾场

---

## 4. Recommended Execution Order

按风险最低、收益最高排序：

1. **Phase 1** 统一入口
2. **Phase 2** 消灭 `global`
3. **Phase 3** 拆 `web/service`
4. **Phase 4** 统一 runner 接口
5. **Phase 5** 重构学习通
6. **Phase 6** 推广到其他平台
7. **Phase 7** 整理模型命名
8. **Phase 8** 清理 infra/config/utils

---

## 5. First Three Concrete Refactor Deliverables

如果只做第一波，优先做这三个：

### Deliverable A: 启动链收口
- 完成 `cmd/yatori` + `internal/app/bootstrap`
- 根 `main.go` 只做兼容
- 去掉重复 init 流程

### Deliverable B: ActivityManager + DB 注入
- 干掉 `global.GlobalDB`
- 干掉 `global.UserActivityMap`
- Service 改成显式依赖结构体

### Deliverable C: 拆 `UserService` 与学习通执行器
- `UserService.go` 拆为 user/execution/scheduler 三块
- 学习通形成新的 runner 模板

---

## 6. Verification Checklist Per Phase

每个阶段完成后至少执行：

```bash
go build ./...
go test ./...
```

如果是 Web 相关阶段，再额外检查：
- 配置管理页面是否可打开
- 用户增删改查是否正常
- 启动/停止账号是否正常
- 自动执行时间窗是否仍生效
- 前端静态资源 `/web/*` 是否正常加载

如果是平台相关阶段，再额外检查：
- 平台登录是否正常
- 课程拉取是否正常
- 执行开始/停止是否正常
- 收尾通知是否正常

---

## 7. Risk Notes

1. 当前分支已有未提交改动，重构前先决定：
   - 继续在 `feature/config-manager-and-auto-execution` 上做
   - 或新开 `feature/refactor-architecture` 分支

2. 学习通逻辑最重，不建议与入口重构同时大改。  
3. 自动执行调度依赖运行态状态管理，必须在 `ActivityManager` 稳定后再深拆。  
4. 现有测试较弱，重构过程中建议补足：
   - service 单测
   - 配置读写测试
   - 调度时间窗测试
   - web handler 基础测试

---

## 8. Suggested Commit Plan

建议按小步提交：

```bash
git checkout -b feature/refactor-architecture
```

提交粒度示例：
1. `refactor: consolidate bootstrap entrypoints`
2. `refactor: replace global db with injected dependency`
3. `refactor: add activity manager for runtime state`
4. `refactor: split user config service from web service`
5. `refactor: introduce platform runner interface`
6. `refactor: extract xuexitong login and course modules`
7. `refactor: extract xuexitong chapter and exam modules`

---

## 9. Definition of Done

满足以下条件才算本次重构完成：

- 唯一清晰入口存在
- `global.GlobalDB` / `global.UserActivityMap` 被移除或仅剩兼容壳
- `web/service/UserService.go` 不再是千行巨石
- 学习通被拆成职责文件
- 平台层有统一 runner 抽象
- 配置、DB、通知、HTTP、平台执行的边界清晰
- `go build ./...` 与 `go test ./...` 通过

---

## 10. Practical Starting Point

**建议现在就从这两个文件开始动：**
1. `internal/app/bootstrap/bootstrap.go`
2. `web/service/UserService.go`

原因：
- 一个决定启动边界
- 一个决定应用层边界
- 先稳住这两个，再拆平台逻辑最省力
