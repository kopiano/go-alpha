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
{ "action": "likes" }
```

```json
{ "action": "unlikes" }
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

### weather
```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "city": "Hangzhou",
    "date": "2026-07-05",
    "week_day": "周日",
    "aqi": 60,
   "temp_current": 26,
   "temp_high": 26,
   "temp_low": 24,
   "aqi": 60,
   "condition": "Light Rain Shower",
   "humidity": "89",
   "uv": "2",
   "wind": "7"
  },
  "error": null
}     
```

┌──────────┬─────────────┬─────────────────────────────┐                                                                                                                  
│   字段   │    来源     │            含义             │                                                                                                                  
├──────────┼─────────────┼─────────────────────────────┤                                                                                                                  
│ humidity │ wttr.in API │ 湿度（百分比），如 "89"     │                                                                                                                  
├──────────┼─────────────┼─────────────────────────────┤                                                                                                                  
│ wind     │ wttr.in API │ 风速（km/h），如 "7"        │                                                                                                                  
├──────────┼─────────────┼─────────────────────────────┤                                                                                                                  
│ uv       │ wttr.in API │ 紫外线指数（0-11+），如 "2" │                                                                                                                  
└──────────┴─────────────┴─────────────────────────────┘
humidity、uv、wind 这三个字段 只出现在 Python 脚本输出的结果中，weather 表里确实没有对应列。 

## chat 
/api/v1/chat 这组接口主要负责三件事：会话列表、消息收发、在线状态同步。

接口                                           作用
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
GET /api/v1/chat/conversations                 获取当前用户的会话列表。返回私聊/群聊会话、联系人目录 users，以及 Team 群信息 team。支持 Redis 缓存。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
GET /api/v1/chat/groups                        获取群聊信息。当前实现里实际返回的是 Team 群数据，格式为 {"groups":[teamInfo]}。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
POST /api/v1/chat/conversations                创建或获取一个与指定用户的私聊会话。传 user_id，返回 conversation_id。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
GET /api/v1/chat/conversations/:id/messages    拉取某个会话的消息记录。支持 limit、offset，也支持 before_id 做向前翻页。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
PUT /api/v1/chat/conversations/:id/read        将指定会话标记为已读。会更新已读状态，并通过 WebSocket 广播 conversation.read / conversation.update。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
POST /api/v1/chat/messages                     发送消息。支持私聊和群聊，自动补全会话、保存消息、广播到在线客户端。
─────────────────────────────────────────────  ───────────────────────────────────────────────────────────────────────────────────────────────────────
GET /api/v1/chat/ws                            WebSocket 入口。用于实时接收消息、在线/离线状态、会话更新、已读通知等事件。

补充两点：

- GET /api/v1/chat/conversations 返回的 users 不是简单通讯录，而是“聊天目录”，会带上最近消息、未读数、在线状态。
- GET /api/v1/chat/ws 需要带 token（Authorization: Bearer ... 或 query token=）完成鉴权。

### 获取联系人列表(用户和群聊)
接口：GET /api/v1/chat/conversations
返回示例
  - 如果没有会话，conversations 会是空数组
```json
{
  "code": 200,
  "message": "ok",
  "data": {
    "conversations": [],
    "team": {
      "id": 1,
      "members": [
        {
          "avatar": "/api/v1/avatar/avatar-1.jpg",
          "user_id": 1,
          "username": "shville"
        },
        {
          "avatar": "/api/v1/avatar/avatar-2.jpg",
          "user_id": 2,
          "username": "test"
        },
        {
          "avatar": "/api/v1/avatar/avatar-3.png",
          "user_id": 3,
          "username": "admin"
        },
        {
          "avatar": "/api/v1/avatar/avatar-4.jpg",
          "user_id": 4,
          "username": "kope"
        },
        {
          "avatar": "/api/v1/avatar/avatar-5.jpg",
          "user_id": 5,
          "username": "amoe"
        }
      ],
      "name": "Team"
    },
    "users": [
      {
        "user_id": 2,
        "username": "test",
        "avatar": "/api/v1/avatar/avatar-2.jpg"
      },
      {
        "user_id": 3,
        "username": "admin",
        "avatar": "/api/v1/avatar/avatar-3.png"
      },
      {
        "user_id": 4,
        "username": "kope",
        "avatar": "/api/v1/avatar/avatar-4.jpg"
      },
      {
        "user_id": 5,
        "username": "amoe",
        "avatar": "/api/v1/avatar/avatar-5.jpg"
      }
    ]
  },
  "error": null
}
```


### 切换选中联系人显示消息
GET /api/v1/chat/conversations/:id/messages
返回示例；
```json
{
"code": 200,
"message": "ok",
"data": {
  "messages": [
    {
      "id": 101,
      "conversation_id": "p_1_2",
      "chat_type": "private",
      "sender_id": 1,
      "receiver_id": 2,
      "group_id": 0,
      "content": "你好",
      "message_type": 1,
      "file_url": "",
      "reply_to_id": null,
      "edited_at": null,
      "deleted_at": null,
      "status": 1,
      "created_at": "2026-07-08T09:58:00Z",
      "updated_at": "2026-07-08T09:58:00Z",
      "sender_username": "alice",
      "sender_avatar": "/api/v1/avatar/avatar-1.jpg"
    },
    {
      "id": 102,
      "conversation_id": "p_1_2",
      "chat_type": "private",
      "sender_id": 2,
      "receiver_id": 1,
      "group_id": 0,
      "content": "在吗",
      "message_type": 1,
      "file_url": "",
      "reply_to_id": null,
      "edited_at": null,
      "deleted_at": null,
      "status": 0,
      "created_at": "2026-07-08T09:59:10Z",
      "updated_at": "2026-07-08T09:59:10Z",
      "sender_username": "bob",
      "sender_avatar": "/api/v1/avatar/avatar-2.jpg"
    }
  ]
},
"error": null
}
```
群聊示例会多出 group_id，私聊则带 receiver_id，这两个字段取决于消息类型。

### 发送消息
POST /api/v1/chat/messages
请求参数
接口支持两种发消息方式：私聊和群聊。
```shell
{
"conversation_id": "p_1_2",
"recipient_id": 2,
"receiver_id": 2,
"chat_type": "private",
"group_id": 0,
"message_type": 1,
"content": "hello",
"file_name": "",
"file_url": ""
}
```
字段说明：

- conversation_id：会话 ID。私聊一般是 p_用户1_用户2，群聊一般是 g_群组ID
- recipient_id：私聊接收方用户 ID，和 receiver_id 二选一即可
- receiver_id：私聊接收方用户 ID，和 recipient_id 二选一即可
- chat_type：private 或 group
- group_id：群聊时必填
- message_type：消息类型
    - 1 文本
    - 2 Emoji / 表情
    - 3 图片
    - 4 文件

- content：消息内容。文本消息必填
- file_name：文件名，文件消息可用
- file_url：文件/图片地址，可选

补充规则：
  - 如果 chat_type 为空，后端会根据 group_id、conversation_id、receiver_id 自动推断
  - 文本消息 message_type=1 时，content 必填
  - 私聊消息必须保证不是给自己发
  - 群聊消息必须是群成员

返回示例
```bash
  {
    "code": 200,
    "message": "Message sent",
    "data": {
      "id": 123,
      "conversation_id": "p_1_2",
      "sender_id": 1,
      "sender_username": "alice",
      "sender_avatar": "/api/v1/avatar/avatar-1.jpg",
      "message_type": 1,
      "content": "hello",
      "status": 0,
      "created_at": "2026-07-08T10:00:00Z",
      "updated_at": "2026-07-08T10:00:00Z",
      "chat_type": "private",
      "receiver_id": 2
    },
    "error": null
  }
```
群聊返回示例：
```sh
  {
    "code": 200,
    "message": "Message sent",
    "data": {
      "id": 124,
      "conversation_id": "g_1",
      "sender_id": 1,
      "sender_username": "alice",
      "sender_avatar": "/api/v1/avatar/avatar-1.jpg",
      "message_type": 1,
      "content": "大家好",
      "status": 0,
      "created_at": "2026-07-08T10:01:00Z",
      "updated_at": "2026-07-08T10:01:00Z",
      "chat_type": "group",
      "group_id": 1
    },
    "error": null
  }
```
失败示例
```bash
{
"code": 400,
"message": "content is required for text/reply messages",
"data": 0,
"error": null
}
```