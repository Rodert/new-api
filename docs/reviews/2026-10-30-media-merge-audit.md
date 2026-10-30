# 图片、视频与计费合并检查（2026-10-30）

基线：`shiyu-newapi` 合并提交 `83fe73536`，主分支父提交 `a2defd2a8`。
范围：自定义图片/视频渠道、游乐场、异步图片任务、价格配置及预扣—结算—退款流程。对相关自定义差异和上游交叉点进行了检查；不代表逐行审查全部上游更新。

## 已确认并修复

| 优先级 | 问题与影响 | 修复位置 |
| --- | --- | --- |
| P1 | 异步图片 worker 缺少用户 ID，游乐场任务报 `invalid userId`；任务执行阶段还可能再次预扣并调整不存在的令牌 | `controller/image_task.go`、`relay/image_handler.go` |
| P1 | 异步图片未保存表达式和完整价格快照，表达式或 token 计费在后台结算时可能失真；任务额度也没有使用实际结算结果 | `controller/image_task.go`、`model/task.go`、`service/text_quota.go` |
| P1 | 异步图片失败退款时减去了尚未累计的使用量，用户和渠道已用额度可能变为负数 | `service/task_billing.go` |
| P1 | 同时提交合法 `duration` 和超大/无效 `seconds` 可绕过共用时长校验 | `relay/common/relay_utils.go` |
| P1 | Grok 原生渠道忽略按次/按秒设置，按次价格仍乘以秒数 | `relay/channel/task/grokvideo/adaptor.go` |
| P1 | 前端缓存重置只保留旧的全局历史键，会删除按用户隔离的新历史键 | `web/src/lib/frontend-cache.ts` |
| P2 | zzone 模型映射后的时长上限未校验，`duration` 别名也未校验模型能力；按秒价格允许缺省时长而只扣一个单位 | `relay/channel/task/jimengzzvideo/adaptor.go` |
| P2 | ChongPlus 计费默认 8 秒，但请求未显式传相同时长 | `relay/channel/task/chongplusvideo/adaptor.go` |
| P2 | 视频 multipart 先消费请求体再解析，单张参考图、分辨率、时长和 metadata 不能可靠传递 | `relay/common/relay_utils.go` |
| P2 | Imagen 部分图片被过滤后，按张计费仍沿用请求数量 | `relay/channel/gemini/relay-gemini.go` |
| P2 | 历史超过 24 条会自动截断；视频百分比进度、null 错误/结果可使整个工作区解析失败；参考图模式未保存 | 游乐场 `storage.ts`、`storage-schema.ts`、`types.ts` |
| P2 | 图片刷新后重新生成依赖已丢失的 File 对象，无法使用历史中的多张参考图 | 游乐场 `image-workspace.tsx` |
| P2 | 视频轮询任一任务失败会阻断全部更新，固定 interval 还会重叠请求 | 游乐场 `video-workspace.tsx` |
| P2 | 游乐场给 Grok video 1.5 提供了后端不支持的 4 秒选项 | 游乐场 `video-workspace.tsx` |
| P2 | 生图失败详情未使用现有脱敏逻辑，可能向普通用户返回上游地址及凭据 | `controller/image_task.go` |
| 检查修复 | 两份自定义前端测试仍用 node:test，当前 Vitest 无法发现测试套件 | 渠道表单和 frontend-cache 测试 |

计费快照保存在任务私有数据中，不通过任务 API 返回；不保存认证、Cookie、Key、Token 或 Secret 请求头。保留原有 JSON/multipart 图片接口，游乐场改图使用规范的 `images: [{image_url: ...}]` JSON，支持多张参考图。

## 回归范围

- SQLite 中实际创建并执行异步图片任务，核对钱包、令牌、消费日志和任务额度；覆盖普通 API/游乐场、完整/部分结果、按张/表达式/token 价格、提交后管理员改价、失败退款与重复收尾。
- 对 zzone、Grok、ChongPlus 分别验证按次及按秒价格，核对上游收到请求前已预留额度、鉴权、路径和模型。
- 视频时长边界、映射模型能力、multipart 字段、默认时长与计费一致性。
- 现有任务成功结算、失败退款、并发终态 CAS、token 和订阅计费回归。
- 历史超过 24 条不截断、缓存重置保留所有用户历史、百分比进度/null 字段、多参考图刷新后重新生成、独立视频轮询。
- 前端类型检查、改动文件 lint、生产构建，根 Go 构建及 relaykit 独立构建。

## 检查结果

- 前端全量：208 个测试文件、2,465 个用例通过；随后补充的 2 个渠道时长选项用例也已通过。
- 后端：controller、relay、relay/common、原生任务渠道、Gemini、OpenAI、helper、service、model、ratio_setting 相关测试通过；最后的图片错误脱敏回归通过。
- 前端类型检查、所有改动文件 lint、前端生产构建、根 Go 构建、`relaykit` 的 `GOWORK=off go build ./...` 均通过。
- `git diff --check` 通过。

## 行为边界

- 原生渠道按秒收费使用请求发送的有效秒数；按次只扣一次。Grok 现在严格服从配置，需要按秒收费的模型须配置 `VideoBillingMode=per_second`。
- zzone 按秒收费时必须提供时长；模型能力按映射后的上游模型校验。
- 异步图片已预扣后，渠道参数覆盖不能增加图片张数；增加时会在调用上游前失败并退款。普通同步图片仍可按有效张数补足预扣。
- 新快照保护新提交任务；历史任务缺少的表达式/价格信息不能可靠补造。
- 历史不再主动截断或随页面缓存重置清空；浏览器存储空间不足、手动清除浏览器数据或远程素材过期仍属于现有存储边界。以前已被截断的记录无法由本次修改恢复。
- 回归使用本地模拟上游，未调用付费生产生成接口，也未验证真实供应商的可用性和 R2 生产配置。

本次检查修复未提交、未推送。
