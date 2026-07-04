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

### cloudflare tunnel
```shell
$ cloudflared tunnel create alpha-api
# Created tunnel alpha-api with id b53f33e1-5f51-4597-9cc1-51b6538a4455
$ ls -la ~/.cloudflared   
#-r--------    1 coulsonzero  staff   175 Jul  4 23:02 b53f33e1-5f51-4597-9cc1-51b6538a4455.json
$ vim ~/.cloudflared/config.yml
#tunnel: alpha-api
#credentials-file: /Users/coulsonzero/.cloudflared/57eac48d-5987-4fbf-835f-32171c972429.json
#protocol: http2  
#
#ingress:
#  - hostname: api.coulsonzero.shop
#    service: http://localhost:8080
#  - service: http_status:404
$ cloudflared tunnel route dns alpha-api api.coulsonzero.shop
$ go run main.go
$ cloudflared tunnel run alpha-api
```
react修改env后重新push部署
```env
VITE_API_URL=https://api.coulsonzero.shop/api/v1
```
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
cloudflared tunnel --loglevel debug run alpha-api
cloudflared --version
```

* ERR Failed to dial a quic connection error="failed to dial to edge with quic: timeout: no recent network activity"
```shell
# http2
cloudflared tunnel --protocol http2 run alpha-api
cloudflared tunnel --protocol http2 --loglevel debug run alpha-api
# ipv4
cloudflared tunnel --protocol http2 --edge-ip-version 4 run alpha-api
```

