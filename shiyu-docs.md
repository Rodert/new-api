# Shiyu 定制功能说明

本文记录 `shiyu-newapi` 分支新增的 JimengZZVideo 渠道、游乐场和对外 API 约定。部署流程与镜像规则见 [shiyu-newapi.md](shiyu-newapi.md)。

## 1. JimengZZVideo 渠道

- 渠道类型固定为 `1000`，刻意避开主分支当前连续使用的类型编号，降低未来合并上游时的冲突风险。
- 后台新建渠道时选择 `JimengZZVideo`。
- 渠道 `Base URL` 填上游服务根地址，**不要带 `/v1`**。适配器会向上游请求 `<Base URL>/v1/videos`。
- 填写上游 API Key，并配置此渠道实际可用的视频模型；模型名可通过渠道模型映射改写成上游模型名。

上游协议和公开视频协议使用相同核心字段：

```json
{
  "model": "as-sd2.0-fast",
  "prompt": "视频描述",
  "seconds": "15",
  "aspect_ratio": "16:9",
  "images": ["https://public.example/reference.png"],
  "videos": ["https://public.example/motion.mp4"],
  "audios": ["https://public.example/music.mp3"]
}
```

- `seconds` 必须是字符串，不能传 JSON 数字。
- `images` 最多 4 项，`videos` 最多 3 项，`audios` 最多 1 项。
- 三种参考素材都必须是上游能访问的公网 HTTP(S) URL；不能传本机路径、Data URL 或 Base64。

## 2. 对外 API

站点 Base URL：

```text
https://ai.silicogrove.com/v1
```

认证统一使用用户在控制台创建的 API Key：

```http
Authorization: Bearer YOUR_API_KEY
```

已对齐参考站使用方式的主要接口：

| 用途 | 方法与路径 |
| --- | --- |
| 查询模型 | `GET /v1/models` |
| 文本聊天 | `POST /v1/chat/completions` |
| Responses | `POST /v1/responses` |
| 图片生成 | `POST /v1/images/generations` |
| 图片编辑 | `POST /v1/images/edits` |
| 视频创建 | `POST /v1/videos` |
| 视频任务查询 | `GET /v1/videos/{task_id}` |
| 下载生成视频 | `GET /v1/videos/{task_id}/content` |
| 语音合成 | `POST /v1/audio/speech` |
| 音频转写 | `POST /v1/audio/transcriptions` |
| 音频翻译 | `POST /v1/audio/translations` |

视频必须通过 `POST /v1/videos` 创建，不应把视频模型发到 `/v1/chat/completions`。提交后应轮询 `GET /v1/videos/{task_id}`，仅在任务完成后请求 `/content` 下载或播放。

`/v1/videos/{task_id}/content` 由本系统代理上游媒体：客户端仍请求本站域名，需携带 API Key，不会直接暴露或要求客户端访问上游视频 URL。

## 3. 临时素材上传

`POST /pg/assets` 是对外可使用的临时素材上传接口，同时支持两种认证：

- API 用户的 `Authorization: Bearer YOUR_API_KEY`。
- 已登录控制台用户的登录令牌，供游乐场使用。

请求为 `multipart/form-data`：

```bash
curl -X POST "https://ai.silicogrove.com/pg/assets" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -F "kind=image" \
  -F "file=@/path/to/reference.jpg"
```

`kind` 和限制：

| kind | 支持类型 | 单文件上限 |
| --- | --- | --- |
| `image` | jpeg、png、webp | 20 MiB |
| `video` | mp4、webm、quicktime/mov | 100 MiB |
| `audio` | aac、mpeg/mp3、mp4/m4a、ogg、wav、webm | 20 MiB |

返回结构：

```json
{
  "success": true,
  "data": {
    "kind": "image",
    "url": "https://ai.silicogrove.com/pg/assets/xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
    "filename": "reference.jpg",
    "content_type": "image/jpeg",
    "size": 123456
  }
}
```

视频请求应将 `data.url` 放到相应数组中：图片放 `images`，视频放 `videos`，音频放 `audios`。

### 存储与清理

- 文件和元数据保存在应用容器的 `/data/playground-assets/`，随 `/data` Docker volume 持久化。
- 单个资源有效期为 24 小时。
- 应用启动时清理一次，之后每小时扫描并删除过期资源及超过 24 小时的孤立残留文件。
- `GET /pg/assets/{asset_id}` 不需要认证，因为视频上游需要自行下载该 URL。资源 ID 是不可预测 UUID，且会过期；不要把它当作长期私有文件存储。

本地的 `http://localhost:3002/pg/assets/...` 只能用于本地界面检查，外部视频上游无法访问。实际生成视频时，资源 URL 必须使用可从公网访问的线上域名。

### 反向代理要求

上传成功后的资源 URL 由收到请求时的域名和协议生成。HTTPS 在 Nginx 终止时，代理必须转发原始域名和协议：

```nginx
proxy_set_header Host $host;
proxy_set_header X-Forwarded-Proto $scheme;
```

否则服务可能返回内部地址或 `http` URL，上游无法下载素材。Caddy 的常规 `reverse_proxy` 通常会自动保留这些信息。

当前素材存储使用本地 `/data` volume，适用于单机 Compose 部署。若以后部署多个应用实例或多台机器，必须将 `/data/playground-assets` 改为共享存储，或改用对象存储；否则上传和上游下载可能被负载均衡分配到不同实例。

## 4. 游乐场

游乐场使用控制台会话，不是给 SDK 或二次中转直接调用的 API：

| 用途 | 游乐场内部路径 |
| --- | --- |
| 聊天 | `POST /pg/chat/completions` |
| 图片生成 | `POST /pg/images/generations` |
| 视频创建 | `POST /pg/videos` |
| 视频任务查询 | `GET /pg/videos/{task_id}` |
| 视频内容 | `GET` / `HEAD /pg/videos/{task_id}/content` |
| 上传参考素材 | `POST /pg/assets` |

功能约定：

- 切换聊天、图片、视频时，模型列表会按当前模式和分组筛选；例如 `gpt-image-2` 归类为图片模型。
- 视频任务轮询转发上游的扁平状态结构，包含 `queued`、`processing`、`completed` 或 `failed`，以及进度、结果或错误信息。
- 浏览器播放视频时，前端用带控制台 Bearer 凭证的请求获取 `/pg/videos/{task_id}/content`，再使用 Blob URL 播放。不能直接让 `<video>` 请求受控 URL，因为原生 `<video>` 无法附带该认证头。
- 聊天、图片和视频的工作区状态保存在浏览器 `localStorage` 的 `playground_workspace_v1`。这是客户端缓存，清理浏览器站点数据或切换浏览器后不会保留。
- 图片和视频各保留最近 24 条生成记录。图片记录包含提示词、模型、分组、尺寸、比例、清晰度、数量和参考图；视频记录还包含任务 ID、状态、进度、时长、参考视频和参考音频。
- 每条生成记录可重新生成或单独删除；图片可逐张下载，已完成视频可播放和下载；每个工作区可单独清空全部历史记录。视频工作区会同时轮询所有未结束的历史任务。

## 5. 用户接入文档

面向用户的单文件 HTML 已放在：

```text
docs/silicogrove-api-docs.html
```

它的示例均使用 `https://ai.silicogrove.com`，覆盖文本、图片、视频、音频、素材上传和二次中转说明。该文件可直接打开预览；如需在生产站点提供 `https://ai.silicogrove.com/docs`，需由 Nginx/Caddy 配置静态文件路由，或后续接入前端路由。

## 6. 本地验证和重启

本地游乐场 Compose：

```bash
docker compose -f docker-compose-playground.yml build new-api
docker compose -f docker-compose-playground.yml up --force-recreate -d
```

本地地址：

```text
http://localhost:3002/playground
```
