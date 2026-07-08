push:
	@git checkout dev
	@git add .
	@bash push.sh dev
merge:
	@bash push.sh main

run:
	@docker compose up -d --build backend
	@cloudflared tunnel run api-test

stop:
	@docker compose stop backend

# (首次使用)启动服务器
docker-build:
	@docker compose up -d	# error先执行: docker builder prune -f

# (常用)只重建后端代码，不删除mysql表、字段数据和redis缓存数据
docker-rebuild:
	@docker compose up -d --build backend

# (慎用)重启服务器(数据全部清空！)
docker-restart:
	@docker compose down -v && docker compose up -d --build

cloudflare:
	@cloudflared tunnel run api-test	# 如果error: 关闭vpn

#log-50:
#	@docker compose logs --tail=50 backend
#
#log:
#	@docker compose logs backend


