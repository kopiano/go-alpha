# API

### 用户认证

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | /api/v1/login | 用户登录，登录后 status 更新为 active |
| POST | /api/v1/logout | 用户退出，退出后 status 更新为 inactive，并记录 last_login_at |
| GET | /api/v1/me | 获取当前登录用户信息 |

退出登录成功响应：

```json
{
  "code": 200,
  "message": "退出成功",
  "data": null,
  "error": null
}
```

### Markdown 文档评论

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | /api/v1/comment | 获取所有 Markdown 文档评论 |
| POST | /api/v1/comment | 提交 Markdown 文档评论 |
| POST | /api/v1/comment/:id/likes | 评论点赞/取消点赞 |

兼容路径：`GET /api/v1/commment`

请求参数：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| content | string | 是 | 评论内容 |
| username | string | 是 | 用户名 |
| email | string | 否 | 邮箱 |
| website | string | 否 | 网站 |
| parent_id | number | 否 | 父评论 ID，传入时表示回复某条评论 |
| comment_time | string | 否 | 评论时间，RFC3339 格式；不传时使用服务端当前时间 |

请求示例：

```json
{
  "content": "这篇文档很有帮助",
  "username": "alice",
  "email": "alice@example.com",
  "website": "https://example.com",
  "parent_id": null,
  "comment_time": "2026-06-30T12:00:00+08:00"
}
```

成功响应：

```json
{
  "code": 200,
  "message": "评论成功",
  "data": {
    "id": 1,
    "content": "这篇文档很有帮助",
    "username": "alice",
    "email": "alice@example.com",
    "website": "https://example.com",
    "parent_id": null,
    "comment_time": "2026-06-30T12:00:00+08:00",
    "like_count": 0,
    "created_at": "2026-06-30T12:00:00+08:00",
    "updated_at": "2026-06-30T12:00:00+08:00"
  },
  "error": null
}
```

回复请求示例：

```json
{
  "content": "同意",
  "username": "bob",
  "email": "",
  "website": "",
  "parent_id": 1,
  "comment_time": "2026-06-30T12:05:00+08:00"
}
```

失败响应：

```json
{
  "code": 400,
  "message": "参数错误，username、content 为必填",
  "data": 0,
  "error": null
}
```

获取评论成功响应：

```json
{
  "code": 200,
  "message": "获取评论成功",
  "data": [
    {
      "id": 1,
      "content": "这篇文档很有帮助",
      "username": "alice",
      "email": "alice@example.com",
      "website": "https://example.com",
      "parent_id": null,
      "comment_time": "2026-06-30T12:00:00+08:00",
      "like_count": 0,
      "created_at": "2026-06-30T12:00:00+08:00",
      "updated_at": "2026-06-30T12:00:00+08:00",
      "replies": [
        {
          "id": 2,
          "parent_id": 1,
          "content": "同意",
          "username": "bob",
          "email": "",
          "website": "",
          "comment_time": "2026-06-30T12:05:00+08:00",
          "like_count": 0,
          "created_at": "2026-06-30T12:05:00+08:00",
          "updated_at": "2026-06-30T12:05:00+08:00"
        }
      ]
    }
  ],
  "error": null
}
```

点赞成功响应：

```json
{
  "code": 200,
  "message": "点赞成功",
  "data": {
    "id": 1,
    "likes": 12,
    "like_count": 12
  },
  "error": null
}
```

取消点赞成功响应：

```json
{
  "code": 200,
  "message": "取消点赞成功",
  "data": {
    "id": 1,
    "likes": 11,
    "like_count": 11
  },
  "error": null
}
```

likes 请求示例：

```json
{ "action": "like" }
```

```json
{ "action": "unlike" }
```


### 访客接口总览

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| POST | /api/v1/visit | 前端页面访问埋点，记录首次访问或一次访问快照 |
| POST | /api/v1/visit/heartbeat | 前端定时上报停留时长，补充 last_seen 和 total_browse_time |
| GET | /api/v1/visitor | 获取访客信息表和汇总统计 |

### 访客信息表

推荐埋点流程：

1. 前端生成并持久化 `visitor_id`
2. 首次进入页面调用 `POST /api/v1/visit`
3. 每 30-60 秒调用 `POST /api/v1/visit/heartbeat`
4. 页面离开时再补一次 `POST /api/v1/visit/heartbeat`

POST 请求参数：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| visitor_id | string | 是 | 前端生成 UUID，长期标识一个访客 |
| duration | number | 否 | 本次页面浏览时长，单位秒 |
| os | string | 否 | 操作系统，例如 macOS、Windows、iOS |
| browser | string | 否 | 浏览器，例如 Chrome、Safari |
| device | string | 否 | 设备类型，例如 Desktop、Mobile、Tablet |
| country | string | 否 | 国家 |
| city | string | 否 | 城市 |
| location | string | 否 | 国家地区字符串 |
| status | string | 否 | 访客状态，默认 online |
| user_name | string | 否 | 登录用户名 |
| avatar | string | 否 | 用户头像 |

POST 请求示例：

```json
{
  "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "duration": 35,
  "os": "macOS",
  "browser": "Chrome",
  "device": "Desktop",
  "country": "United States",
  "city": "Mountain View",
  "location": "United States Mountain View",
  "status": "online",
  "user_name": "alice",
  "avatar": "/api/v1/avatar/avatar-1.jpg"
}
```

heartbeat 请求示例：

```json
{
  "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "duration": 15
}
```

heartbeat 成功响应：

```json
{
  "code": 200,
  "message": "更新成功",
  "data": {
    "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
    "duration": 15
  },
  "error": null
}
```

POST 成功响应：

```json
{
  "code": 200,
  "message": "记录成功",
  "data": {
    "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
    "total_visitors": 10234,
    "duration": 35,
    "ip": "8.8.8.8",
    "country": "United States",
    "city": "Mountain View",
    "location": "United States Mountain View",
    "os": "macOS",
    "browser": "Chrome",
    "brower": "Chrome",
    "device": "Desktop",
    "status": "online",
    "user_name": "alice",
    "avatar": "/api/v1/avatar/avatar-1.jpg",
    "visitor": {
      "id": 1,
      "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
      "ip": "8.8.8.8",
      "country": "United States",
      "city": "Mountain View",
      "location": "United States Mountain View",
      "first_seen": "2026-06-30T12:00:00+08:00",
      "last_seen": "2026-06-30T12:30:00+08:00",
      "total_browse_time": 360,
      "os": "macOS",
      "browser": "Chrome",
      "device": "Desktop",
      "status": "online",
      "user_name": "alice",
      "avatar": "/api/v1/avatar/avatar-1.jpg",
      "created_at": "2026-06-30T12:00:00+08:00",
      "updated_at": "2026-06-30T12:30:00+08:00"
    }
  },
  "error": null
}
```

GET 返回字段：

| 字段 | 说明 |
| --- | --- |
| pv / total_pv | 总访问量 |
| uv / total_uv | 真实访客人数 |
| active_visitors | 在线活跃人数 |
| total_browse_time | 总浏览时间 |
| visitors | 访客列表 |

GET 成功响应示例：

```json
{
  "code": 200,
  "message": "获取统计成功",
  "data": {
    "pv": 45678,
    "uv": 10234,
    "total_pv": 45678,
    "total_uv": 10234,
    "today_pv": 156,
    "today_uv": 42,
    "weekly_uv": 389,
    "active_visitors": 8,
    "total_browse_time": 98765,
    "visitors": [
      {
        "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
        "ip": "8.8.8.8",
        "country": "United States",
        "city": "Mountain View",
        "region": "United States Mountain View",
        "first_seen": "2026-06-30T12:00:00+08:00",
        "last_seen": "2026-06-30T12:30:00+08:00",
        "duration": 360,
        "total_browse_time": 360,
        "os": "macOS",
        "browser": "Chrome",
        "brower": "Chrome",
        "device": "Desktop",
        "status": "online",
        "user_name": "alice",
        "avatar": "/api/v1/avatar/avatar-1.jpg"
      }
    ]
  },
  "error": null
}
```

说明：
- `region` 是国家地区字符串，前端可直接展示
- `brower` 保留为兼容字段，建议前端优先使用 `browser`
- `active_visitors` 按最近 5 分钟有访问记录计算
