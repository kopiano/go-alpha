# shell: make push
push:
	@bash push.sh

run:
	@go run main.go

# go-alpha, mysql, redis
docker-build:
	@docker compose up -d	# error先执行: docker builder prune -f

# 只重建后端代码，不重建mysql, redis
docker-rebuild:
	@docker compose up -d --build backend

# 重建backend，清空mysql，redis数据库重建
docker-restart:
	@docker compose down -v && docker compose up -d --build

