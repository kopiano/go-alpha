# Cloudflare
**部署思路**
```sh
react(vite): cloudflare pages
go(gin).   : cloudflare tunnel 
```
**react(vite)**
```sh
1. https://dash.cloudflare.com
2. 左侧搜索 Pages 进入 "Works and Pages"
3. 点击右上角 "Create application"
4. 点击中央正下方 "Get started"
5. 点击 Import an existing Git repository右侧的 "Get started"
3. Select a repository eg "web-alpha", 点击"Begin setup"
5. Set:
  - Project name: web-alpha-cloudflare
  - Framework: React(vite)
  - Build command: pnpm run build
  - Build output directory: dist
6. 点击 "Save and Deploy"
```
**自定义域名**
```
1. 搜索 "Works and Pages", 筛选Show all -> "Pages", 点击项目卡片进入
2. 点击正上方 "Custom domains", 点击"set up a custom domains"按钮
3. 输入最终的访问网址：如"alpha.coulsonzero.shop"
4. 搜索"DNS", 点击"Add record"
5. 设置 
Type: CNAME
Name: alpha
IPv4 address: web-alpha-cloudflare.pages.dev
6. 点击"Save"
7. 等待一段时间即可(几分钟至十几分钟)
```


### deploy a go project
- 后端: 临时可用ngrok测试

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
