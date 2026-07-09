# go-alpha

### 技术栈
* gin框架、Gorm框架
* mysql数据持久化
* redis缓存数据，去重
* CORS跨域：前后端port不同

### pkg
```sh
go get -u "github.com/gin-gonic/gin"
go get -u gorm.io/gorm
go get -u gorm.io/driver/mysql

"github.com/redis/go-redis/v9"
"github.com/gin-contrib/cors"
"github.com/gin-contrib/sessions"
"github.com/gin-contrib/sessions/cookie"
```

### 注意事项

用户信息存放在config.yaml文件中
```
mysql:
  host: localhost
  port: 3306
  user: root
  password: root123456
  dbname: go_alpha

redis:
  host: localhost
  port: 6379
```

docker容器中的mysql，redis改了映射，yaml文件也要改？

### mysql
```shell
mysql -u root -p
```
```sql
create database go_alpha;
```
更改字段要先删除再运行
```sql
DROP TABLE IF EXISTS user; 
```

### CORS
```go
r.Use(cors.New(cors.Config{
		AllowAllOrigins:  true,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
	}))
```

### linux
```shell
kill -9 $(lsof -t -i:8000)
```


### 统计访客人数
方案对比

┌────────────────────────────┬────────────────────────┬──────────────────────────────┬────────────────────┬──────────┐                                                    
│            方案            │          原理          │            准确性            │      实现成本      │ 隐私影响 │                                                    
├────────────────────────────┼────────────────────────┼──────────────────────────────┼────────────────────┼──────────┤                                                    
│ IP                         │ 记录访客 IP            │ 低（NAT/代理导致多人同IP）   │ 低                 │ 低       │                                                    
├────────────────────────────┼────────────────────────┼──────────────────────────────┼────────────────────┼──────────┤                                                    
│ IP + User-Agent            │ IP + UA 组合指纹       │ 中                           │ 低                 │ 低       │                                                    
├────────────────────────────┼────────────────────────┼──────────────────────────────┼────────────────────┼──────────┤                                                    
│ Cookie/ Session            │ 服务端分配唯一标识     │ 高（用户清cookie会重新计数） │ 低                 │ 中       │                                                    
├────────────────────────────┼────────────────────────┼──────────────────────────────┼────────────────────┼──────────┤                                                    
│ 浏览器指纹(Canvas/WebGL等) │ 采集浏览器特征生成哈希 │ 高                           │ 高（需前端JS）     │ 中       │                                                    
├────────────────────────────┼────────────────────────┼──────────────────────────────┼────────────────────┼──────────┤                                                    
│ 用户登录态                 │ 基于 UserID            │ 最高                         │ 低（已有用户体系） │ 低       │                                                    
└────────────────────────────┴────────────────────────┴──────────────────────────────┴────────────────────┴──────────┘

ai 推荐：Cookie/Session + uuid去重

┌───────────────────────┬────────────────────────────────────────────────────────┐                                                                                        
│         组件          │                          职责                          │                                                                                        
├───────────────────────┼────────────────────────────────────────────────────────┤                                                                                        
│ Redis (HLL + Counter) │ 实时计数、UV 去重、本周/今日统计                       │                                                                                        
├───────────────────────┼────────────────────────────────────────────────────────┤                                                                                        
│ MySQL                 │ 持久化存储，每天一条 (date, uv, pv)                    │                                                                                        
├───────────────────────┼────────────────────────────────────────────────────────┤                                                                                        
│ Redis (Cache)         │ 缓存 total_uv / total_pv 10 分钟，免去每次重复 PFCount │                                                                                        
└───────────────────────┴────────────────────────────────────────────────────────┘
- 新访客 → 生成 UUID 设置 cookie，UV +1（Redis HLL），总访客数 +1
- 老访客 → 不增加总 UV，仅记录今日 UV/PV
- MySQL → 每次访问同步更新当天 (date, uv, pv) 记录
- 缓存 → 刷新 total_uv / total_pv（10 分钟过期）
 

### docker
改动了后端代码，但数据库不显示：如果只想重建后端（不重启MySQL/Redis），也无需关闭之前的后端容器程序
```sh
docker compose up -d --build backend 
```
查看log
```shell
docker compose logs --tail=50 backend
```

**如果拉取go-1.26.4失败**
1. 使用 Docker Desktop旧版本为4.43.0, 不要使用最新版
2. 关闭代理：Setting - Resources - Proxies - 确保关闭 "Manual proxy configuration"
3. Setting - Docker Engine 新增
```json
{
  "registry-mirrors": [
    "https://docker.m.daocloud.io",
    "https://docker.1panel.live",
    "https://docker.xuanyuan.me",
    "https://docker.unsee.tech"
  ]
}
```

**查看build构建错误**
```shell
docker build --progress=plain --no-cache -t your-image-name .
```
4. 然后使用docker compose up -d

### cloudflare tunnel
```shell
$ cloudflared tunnel create api-test
# Created tunnel alpha-api with id b53f33e1-5f51-4597-9cc1-51b6538a4455
$ ls -la ~/.cloudflared   
#-r--------    1 coulsonzero  staff   175 Jul  4 23:02 b53f33e1-5f51-4597-9cc1-51b6538a4455.json
$ vim ~/.cloudflared/config.yml
#tunnel: api-test
#credentials-file: /Users/coulsonzero/.cloudflared/57eac48d-5987-4fbf-835f-32171c972429.json
#protocol: http2  
#
#ingress:
#  - hostname: api.coulsonzero.shop
#    service: http://localhost:8080
#  - service: http_status:404
$ cloudflared tunnel route dns api-test api.coulsonzero.shop
$ go run main.go
$ cloudflared tunnel run api-test
```
react修改env后重新push部署
```env
VITE_API_URL=https://api.coulsonzero.shop/api/v1
```
也要更改cloudflare pages项目-setting-变量为上述值
然后重新运行`cloudflared tunnel run alpha-api`
`curl https://api.coulsonzero.shop/api/v1/user`查看是否成功


* 查看tunnel
```shell
cloudflared tunnel list
#ID                                   NAME          CREATED              CONNECTIONS 
#b53f33e1-5f51-4597-9cc1-51b6538a4455 alpha-api     2026-07-04T15:02:43Z             
#57eac48d-5987-4fbf-835f-32171c972429 alpha-backend 2026-07-04T08:41:40Z             
#6d87878a-4baf-48d7-9cf1-2d360faeae06 gin-api       2026-07-04T15:02:15Z
cloudflared tunnel delete alpha-backend
cloudflared tunnel --loglevel debug run api-test
cloudflared --version
```

* ERR Failed to dial a quic connection error="failed to dial to edge with quic: timeout: no recent network activity"
```shell
# http2
cloudflared tunnel --protocol http2 run api-test
cloudflared tunnel --protocol http2 --loglevel debug run api-test
# ipv4
cloudflared tunnel --protocol http2 --edge-ip-version 4 run api-test
```

## git
### git commit 规范
| Type       | 用途         | 示例                                          |
| ---------- | ---------- | ------------------------------------------- |
| `feat`     | 新功能        | `feat(chat): add private messaging`         |
| `fix`      | 修复 Bug     | `fix(auth): resolve token expiration issue` |
| `refactor` | 重构（不改功能）   | `refactor(user): simplify login logic`      |
| `perf`     | 性能优化       | `perf(chat): optimize message query`        |
| `style`    | 格式调整（不改逻辑） | `style(ui): format sidebar styles`          |
| `docs`     | 文档更新       | `docs: update README`                       |
| `test`     | 测试         | `test(chat): add websocket unit tests`      |
| `build`    | 构建配置       | `build: update Dockerfile`                  |
| `ci`       | CI/CD 配置   | `ci: add GitHub Actions workflow`           |
| `chore`    | 杂项维护       | `chore: upgrade dependencies`               |
| `revert`   | 回滚提交       | `revert: revert JWT authentication changes` |

### 版本回滚
```shell
git log --oneline
:4b98d3f
git reset --hard 4b98d3f
```