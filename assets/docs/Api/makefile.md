# makefile

```Makefile
# shell: make push
push:
	@bash push.sh

docker-build:
	@docker compose up -d	# error先执行: docker builder prune -f

docker-rebuild:
	@docker compose up -d --build backend
```