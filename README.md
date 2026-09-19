# 数据中心热约束机柜布局规划器

面向数据中心容量规划团队的离线决策支持系统。系统维护热区、机柜与待规划设备负载，以确定性算法生成机柜候选布局，并给出功率、气流、U 位、冷量、回风温度、邻接热影响与冗余隔离证据。它不会连接或控制 BMS、空调、PDU 或机柜设备。

## Docker 一键启动

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

服务健康后访问：

- 前端工作台：`http://localhost:18523`
- 后端健康检查：`http://localhost:19523/healthz`
- 后端就绪检查：`http://localhost:19523/readyz`
- PostgreSQL：`localhost:57523`

测试账号：

| 角色 | 用户名 | 密码 | 权限 |
| --- | --- | --- | --- |
| 规划员 | `planner` | `planner123` | 维护边界和输入、创建并评估方案 |
| 复核员 | `reviewer` | `reviewer123` | 查看数据、比较版本、批准无严重违规的方案 |
| 管理员 | `admin` | `admin123` | 全部权限，包括归档 |

停止并删除本项目数据卷：

```bash
docker compose down -v --remove-orphans
```

## 主要功能

- `/zones`：编辑热区冷量、送风/最大回风温度、邻接权重与区域状态，查看机柜功率边界占比。
- `/racks`：按热区显示稳定机柜网格，维护唯一位置、功率、气流、U 位和可用状态。
- `/loads`：维护设备负载、冗余组和偏好热区，对 ready 输入执行批量业务校验。
- `/planner`：选择负载创建草稿，执行确定性候选布局，查看逐机柜结果、热传播、评分和约束证据。
- `/audit`：处理待复核方案、比较两个评估版本、检索带 request ID 的审计事件。

布局算法先按负载对可用机柜的约束紧度排序，再按容量余量、邻接热惩罚、热点惩罚、保留机柜惩罚和偏好奖励评分；评分相同时按机柜编码排序。相同输入快照与算法版本会得到相同结果。无法放置的负载会返回 `LOAD_UNPLACED` 及候选约束证据，不会静默忽略。

## 技术栈

- 后端：Go 1.22、Gin、GORM、JWT、bcrypt、结构化 `slog`
- 数据库：Compose 使用 PostgreSQL 16；运行时冒烟支持 GORM SQLite 内存模式
- 前端：Angular 17 standalone components、Angular Material、RxJS、Lucide icons
- 部署：Docker Compose、Nginx SPA fallback 与 `/api` 反向代理

## 目录

```text
backend/cmd/server/          服务入口和优雅停机
backend/internal/model/     四个领域模型
backend/internal/dto/       请求校验与响应类型
backend/internal/repository/事务与持久化
backend/internal/service/   业务规则和状态流
backend/internal/planner/   候选、约束、热传播、评分
backend/internal/handler/   HTTP 输入输出
backend/internal/router/    实体路由
backend/internal/middleware/请求 ID、日志、认证、RBAC、恢复、限流
frontend/src/app/api/       强类型 API clients 与拦截器
frontend/src/app/stores/    认证和方案状态
frontend/src/app/hooks/     useAuth、useScenarioEvaluation
frontend/src/app/pages/     五个业务页面与登录页
frontend/src/app/components/common/ 共享容量、约束、版本组件
frontend/src/types/         前端共享领域类型
output/                     验收报告与 Browser 截图
```

## 枚举贯穿位置

`RackStatus = available | reserved | unavailable | maintenance`

- 数据库/model：`backend/internal/model/rack.go`
- 后端共享枚举：`backend/internal/constants/rack.go`
- DTO/service/handler/router：`backend/internal/dto/rack.go`、`backend/internal/service/rack.go`、`backend/internal/handler/rack.go`、`backend/internal/router/rack.go`
- 前端 type/API/page：`frontend/src/types/rack.ts`、`frontend/src/app/api/rack.api.ts`、`frontend/src/app/pages/racks.page.ts`

`ScenarioStatus = draft | evaluating | pending_review | approved | archived`

- 数据库/model：`backend/internal/model/layout_scenario.go`
- 后端共享枚举与状态机：`backend/internal/constants/scenario.go`
- DTO/service/handler/router：`backend/internal/dto/layout_scenario.go`、`backend/internal/service/layout_scenario.go`、`backend/internal/handler/layout_scenario.go`、`backend/internal/router/layout_scenario.go`
- 前端 type/store/component/page：`frontend/src/types/scenario.ts`、`frontend/src/app/stores/scenario.store.ts`、`frontend/src/app/components/common/version-compare-panel.component.ts`、`frontend/src/app/pages/planner.page.ts`、`frontend/src/app/pages/audit.page.ts`

共享组件 `CapacityMeter` 同时用于热区、机柜相关容量展示，`ConstraintBadge` 用于规划与审计，`VersionComparePanel` 用于方案版本比较。`useAuth` 提供全局登录身份，`useScenarioEvaluation` 唯一负责评估提交和 evaluating 状态轮询。

## 环境变量

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `COMPOSE_PROJECT_NAME` | `datacenter-thermal-capacity-planner` | Compose 项目和容器前缀 |
| `FRONTEND_PORT` / `BACKEND_PORT` / `DB_PORT` | `18523` / `19523` / `57523` | 主机端口 |
| `POSTGRES_DB` / `POSTGRES_USER` / `POSTGRES_PASSWORD` | 见 `.env.example` | PostgreSQL 初始化参数 |
| `DB_DRIVER` | `postgres` | `postgres` 或本地冒烟的 `sqlite` |
| `DB_DSN` | 见 `.env.example` | GORM 连接串 |
| `DB_AUTO_MIGRATE` | `true` | 启动时迁移并写入幂等种子数据 |
| `JWT_SECRET` | 开发示例值 | 至少 32 字符，生产必须替换 |
| `JWT_TTL_MINUTES` | `480` | 会话有效期 |
| `CORS_ORIGINS` | `http://localhost:18523` | 逗号分隔允许来源 |
| `RATE_LIMIT_PER_MINUTE` | `180` | 单客户端本地固定窗口限流 |
| `PLANNER_MAX_ITERATIONS` | `500` | 单次候选搜索上限 |
| `LOG_LEVEL` | `info` | `debug/info/warn/error` |
| `SHUTDOWN_TIMEOUT_SECONDS` | `10` | 优雅停机期限 |

## API

所有业务响应使用 `{ "data": ..., "request_id": "..." }`，错误使用 `{ "error": { "code": "...", "message": "..." }, "request_id": "..." }`。除登录与健康检查外均需 `Authorization: Bearer <JWT>`。

| 方法与路径 | 说明 |
| --- | --- |
| `POST /api/v1/auth/login` | 登录并签发 JWT |
| `GET/POST /api/v1/zones`、`GET/PUT /api/v1/zones/:id` | 热区查询与维护 |
| `GET/POST /api/v1/racks`、`GET/PUT /api/v1/racks/:id` | 机柜查询与乐观锁更新 |
| `GET/POST /api/v1/loads`、`GET/PUT /api/v1/loads/:id` | 设备负载查询与维护 |
| `POST /api/v1/loads/validate` | 批量校验 ready 输入 |
| `GET/POST /api/v1/scenarios` | 方案查询与创建 |
| `POST /api/v1/scenarios/:id/evaluate` | 版本校验后执行规划 |
| `POST /api/v1/scenarios/:id/transition` | 复核、批准或归档状态流 |
| `GET /api/v1/scenarios/:id/compare?right_id=` | 比较两个方案 |
| `GET /api/v1/audit-events` | 审计检索 |
| `GET /healthz`、`GET /readyz` | 存活与数据库就绪检查 |

## 模型假设与安全边界

热传播模型采用“本区设备热量 + 邻区热量乘邻接权重”，并假设达到冷量上限时送风至回风温升为 12 C。输出仅用于离线布局比较，不是 CFD、传感器读数或制冷控制结论。批准要求没有 `critical` 违规，但仍需具备资质的工程人员复核。

服务不会连接 BMS、空调、PDU、机柜控制器，不会下发设备动作，也不包含库存、采购、订单、工单或财务能力。日志不记录 JWT 和数据库密码。写操作带角色门禁，机柜与方案使用版本号避免并发覆盖，场景评估/审批和审计在事务内完成。

## 本地开发与测试

后端 SQLite 开发：

```bash
cd backend
PORT=20523 DB_DRIVER=sqlite DB_DSN='file:local?mode=memory&cache=shared' DB_AUTO_MIGRATE=true JWT_SECRET='local-development-secret-at-least-32-bytes' go run ./cmd/server
```

前端开发（需后端代理或 Compose API）：

```bash
cd frontend
npm ci
npm run typecheck
npm run build
```

后端验证：

```bash
go work sync
go build ./backend/...
go vet ./backend/...
go test ./backend/...
```

## 常见问题

- `JWT_SECRET must contain at least 32 characters`：替换 `.env` 中的密钥并重启后端。
- 前端返回 502：检查 `docker compose ps`，确认 `db` 和 `backend` 均为 healthy。
- 方案无法批准：在规划页或审计页查看 `critical` 约束；修改输入后退回 draft 并重新评估。
- 更新机柜或方案返回 409：数据版本已变化，刷新后基于最新 `version` 再提交。
- 端口冲突：只修改 `.env` 中三个主机端口，容器端口保持不变。

## License

[MIT](LICENSE)
