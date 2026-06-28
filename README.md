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
go get -u "github.com/redis/go-redis/v9"
go ger -u "github.com/gin-contrib/cors"

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