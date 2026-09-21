# 只读 API 文档

供 App / 小程序 / 第三方客户端对接。**只读**：不含任何写操作，无需鉴权，也不涉及用户数据。

所有接口均为 `GET`，返回 `JSON`。

---

## 通用约定

### 请求地址

```
http://<你的域名>/api/...
```

当前部署为 `http://novel.linbei.de`，应用监听 `8089`。若走 Nginx 反代，把 `/api` 转发到该端口即可（见文末）。

### 响应结构

所有接口统一返回：

```json
{
  "ret": 0,
  "msg": "",
  "data": { }
}
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `ret` | int | `0` 成功；非 0 失败 |
| `msg` | string | 错误说明，成功时为空 |
| `data` | object / array / null | 数据体，失败时为 `null` |

### 错误码

| ret | 含义 | 常见原因 |
|---|---|---|
| `0` | 成功 | — |
| `1` | 参数错误 | 缺少必填参数、参数非法 |
| `1001` | 资源不存在 | 小说 / 章节已被删除 |

错误响应示例：

```json
{"ret": 1001, "msg": "该小说不存在或者已被删除", "data": null}
{"ret": 1, "msg": "参数错误：需要 id 与 novid", "data": null}
```

### 跨域

已放开 CORS，可直接跨域调用；`OPTIONS` 预检请求会返回 `200`。

### 封面地址

`cover` 字段返回**绝对地址**（如 `http://novel.linbei.de/public/up/xxx.jpg`），
客户端可直接加载，无需自行拼接域名。该地址取自后台配置的 `WebURL`。

### 时间字段

时间统一为 **Unix 时间戳（秒）**，客户端自行格式化。

### 分页

需要分页的接口支持：

| 参数 | 默认 | 上限 | 说明 |
|---|---|---|---|
| `p` | 1 | — | 页码，从 1 开始 |
| `size` | 20 | **100** | 每页条数 |

非法值会被收敛（`p=0` 或负数 → 1；`size` 超过 100 → 100）。

返回体统一包含：

```json
{ "list": [], "total": 32, "page": 1, "size": 20, "pages": 2 }
```

| 字段 | 说明 |
|---|---|
| `total` | 总条数 |
| `pages` | 总页数（`total` 为 0 时返回 0）|

---

## 1. 首页聚合

```
GET /api/home
```

一次返回 App 首屏所需全部数据，减少请求数。

**参数**：无

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "theme": "default",
    "banners": [
      {
        "id": 1,
        "name": "宅魔女",
        "cover": "http://novel.linbei.de/public/up/2022/09/13/1663062922810304681.jpg",
        "link": "http://xs.lanyw.top/book/index?id=509",
        "desc": ""
      }
    ],
    "today_recs": [ /* 今日推荐，6 本 */ ],
    "recs":       [ /* 编辑推荐，12 本 */ ],
    "hots":       [ /* 热门，12 本 */ ],
    "new_ups":    [ /* 最新更新，12 本 */ ],
    "ranks":      [ /* 排行榜，10 本 */ ]
  }
}
```

各列表元素结构详见下方「书籍简要信息」。

> `banners` 可能为空数组（未配置轮播时），客户端需处理。

---

## 2. 分类列表

```
GET /api/cates
```

**参数**：无

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "total": 9,
    "list": [
      { "id": 1, "name": "玄幻魔法" },
      { "id": 2, "name": "武侠修真" }
    ]
  }
}
```

只返回在导航菜单中展示的分类。

---

## 3. 书籍列表

```
GET /api/books
```

**参数**

| 参数 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `cate` | int | 0 | 分类 ID，`0` 表示全部 |
| `status` | int | 0 | `1` 连载中、`2` 完结、`3` 太监；`0` 全部 |
| `text_num` | int | 0 | 字数区间，见下表 |
| `uptime` | int | 0 | 更新时间范围，见下表 |
| `ot` | int | 1 | 排序方式，见下表 |
| `kw` | string | — | 关键词，按书名模糊匹配 |
| `p` | int | 1 | 页码 |
| `size` | int | 20 | 每页条数（上限 100）|

**`text_num` 取值**

| 值 | 区间 |
|---|---|
| 1 | 30 万字以下 |
| 2 | 30 ~ 50 万字 |
| 3 | 50 ~ 100 万字 |
| 4 | 100 ~ 200 万字 |
| 5 | 200 万字以上 |

**`uptime` 取值**：`1` 三天内、`2` 七天内、`3` 十五天内、`4` 三十天内

**`ot` 取值**：`1` 按人气、`2` 按最近更新、`3` 按字数

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "total": 32,
    "page": 1,
    "size": 1,
    "pages": 32,
    "list": [
      {
        "id": 1,
        "name": "诡秘之主",
        "author": "爱潜水的乌贼",
        "cover": "http://novel.linbei.de/public/up/2026/09/21/1789959612249662529.jpg",
        "cate_id": 1,
        "cate_name": "玄幻魔法",
        "status": 1,
        "status_name": "连载中",
        "text_num": 923126,
        "chapter_num": 200,
        "chapter_title": "第一百九十七章 行动（第二更求月票）",
        "chapter_updated_at": 1789887513,
        "views": 0
      }
    ]
  }
}
```

---

## 4. 搜索

```
GET /api/search
```

参数与响应**与 `/api/books` 完全一致**，仅路径更直观。
搜索时关键词用 `kw`：

```
GET /api/search?kw=诡秘
```

> 返回的书名是**纯文本**，不含网页版的高亮标签。

---

## 5. 排行榜

```
GET /api/rank
```

**参数**

| 参数 | 类型 | 默认 | 说明 |
|---|---|---|---|
| `cate` | int | 0 | 分类 ID，指定则返回该分类热门；`0` 为全站排行 |

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "list": [ /* 最多 20 本，结构同「书籍简要信息」 */ ]
  }
}
```

---

## 6. 书籍详情

```
GET /api/book/{id}
```

**路径参数**

| 参数 | 说明 |
|---|---|
| `id` | 小说 ID |

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "first_chap_id": 1000001,
    "novel": {
      "id": 1,
      "name": "诡秘之主",
      "author": "爱潜水的乌贼",
      "cover": "http://novel.linbei.de/public/up/2026/09/21/1789959612249662529.jpg",
      "desc": "蒸汽与机械的浪潮中，谁能触及非凡？……",
      "cate_id": 1,
      "cate_name": "玄幻魔法",
      "status": 1,
      "status_name": "连载中",
      "text_num": 923126,
      "chapter_num": 200,
      "chapter_id": 1000200,
      "chapter_title": "第一百九十七章 行动（第二更求月票）",
      "chapter_updated_at": 1789887513,
      "views": 93,
      "created_at": 1789872302,
      "updated_at": 1789971925,
      "is_original": 0
    }
  }
}
```

| 字段 | 说明 |
|---|---|
| `desc` | 简介，已转为纯文本（不含 HTML 标签），未截断 |
| `first_chap_id` | 第一章 ID，用于「开始阅读」；为 `0` 表示暂无章节 |
| `chapter_id` / `chapter_title` | 最新章节 |
| `is_original` | `1` 原创、`0` 采集 |

> 调用该接口会使 `views` 浏览次数 +1（与网页版行为一致）。

---

## 7. 章节目录

```
GET /api/book/{id}/chapters
```

**路径参数**：`id` 小说 ID

**查询参数**

| 参数 | 默认 | 说明 |
|---|---|---|
| `p` | 1 | 页码 |
| `size` | 20 | 每页条数（上限 100）|
| `order` | `asc` | `asc` 从第 1 章开始；`desc` 从最新章开始 |

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "novel_id": 1,
    "novel_name": "诡秘之主",
    "total": 200,
    "page": 1,
    "size": 2,
    "pages": 100,
    "list": [
      { "id": 1000001, "chapter_no": 1, "title": "第一章 绯红", "text_num": 3661, "updated_at": 0 },
      { "id": 1000002, "chapter_no": 2, "title": "第二章 情况", "text_num": 4569, "updated_at": 0 }
    ]
  }
}
```

> **`desc` 排序常用于「目录默认展示最新章节」**；阅读时建议用 `asc`。

---

## 8. 章节正文

```
GET /api/chapter/{id}?novid={novelId}
```

**路径参数**

| 参数 | 说明 |
|---|---|
| `id` | 章节 ID（由目录接口的 `list[].id` 获得）|

**查询参数**

| 参数 | 必填 | 默认 | 说明 |
|---|---|---|---|
| `novid` | **是** | — | 小说 ID。章节数据按小说号分表存储，缺少该参数查不到 |
| `with_nav` | 否 | `1` | `1` 返回上下章 ID；`0` 不返回（可略减响应体积）|

**响应**

```json
{
  "ret": 0,
  "msg": "",
  "data": {
    "id": 1000001,
    "novel_id": 1,
    "chapter_no": 1,
    "title": "第一章 绯红",
    "content": "\n<br/>    痛！...(共3661字符)",
    "text_num": 3661,
    "created_at": 1789886862,
    "pre_id": 0,
    "next_id": 1000002
  }
}
```

| 字段 | 说明 |
|---|---|
| `content` | 正文，**HTML 片段**（含 `<br/>`、`&nbsp;`）|
| `pre_id` | 上一章 ID；为 `0` 表示已是第一章 |
| `next_id` | 下一章 ID；为 `0` 表示已是最后一章 |

**关于 `content` 的处理建议**

正文原样返回采集内容（HTML 片段），客户端可：

1. 直接交给富文本组件渲染；或
2. 自行转纯文本——把 `<br>`、`</p>` 替换为换行，去除其余标签，反转义实体。

**翻页流程**

```
1. GET /api/book/1            → 拿 first_chap_id 开始阅读
2. GET /api/chapter/1000001?novid=1  → 拿 content 与 next_id
3. GET /api/chapter/{next_id}?novid=1 → 继续
4. next_id 为 0 时表示读完
```

---

## 附：书籍简要信息结构

出现在 `/api/home` 各列表、`/api/books`、`/api/search`、`/api/rank` 中。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int | 小说 ID |
| `name` | string | 书名 |
| `author` | string | 作者 |
| `cover` | string | 封面绝对地址，可能为空 |
| `cate_id` | int | 分类 ID |
| `cate_name` | string | 分类名 |
| `status` | int | `1` 连载中、`2` 完结、`3` 太监、`0` 未开始 |
| `status_name` | string | 状态中文名 |
| `text_num` | int | 总字数 |
| `chapter_num` | int | 章节数 |
| `chapter_title` | string | 最新章节标题 |
| `chapter_updated_at` | int | 最新章节更新时间（Unix 秒）|
| `views` | int | 浏览次数 |

---

## 附：Nginx 反代参考

若要把 `/api` 暴露到 80/443（应用本身监听 8089）：

```nginx
location /api/ {
    proxy_pass http://127.0.0.1:8089;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}
```

封面等静态资源同理，需另配 `/public/`：

```nginx
location /public/ {
    alias /www/novel/static/;
    expires 30d;
}
```

---

## 附：完整调用示例

```bash
# 首页
curl "http://novel.linbei.de/api/home"

# 分类列表
curl "http://novel.linbei.de/api/cates"

# 玄幻魔法分类下、按人气排序的第 1 页
curl "http://novel.linbei.de/api/books?cate=1&ot=1&p=1&size=20"

# 搜索
curl "http://novel.linbei.de/api/search?kw=诡秘"

# 排行榜
curl "http://novel.linbei.de/api/rank"

# 书籍详情
curl "http://novel.linbei.de/api/book/1"

# 目录（前 50 章）
curl "http://novel.linbei.de/api/book/1/chapters?size=50&order=asc"

# 章节正文
curl "http://novel.linbei.de/api/chapter/1000001?novid=1"
```

---

## 附：尚未提供的功能

以下能力当前 API **不支持**，如需请另行评估：

- 用户体系（注册 / 登录 / token）
- 书架、收藏、阅读进度同步
- TXT 下载（网页端已有 `/book/{id}/download.html`，App 可直接访问该地址下载）

> 书架与阅读进度可在**客户端本地**实现（本地数据库或文件），无需服务端参与。
