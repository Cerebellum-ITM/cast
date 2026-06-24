## build: Compile the production binary [tags=build,go] [sc=b]
build:
	@echo "→ go build -o bin/app ./cmd/app"
	@sleep 1
	@echo "✓ built bin/app (4.2 MB)"

## test: Run the unit test suite [tags=test] [sc=t]
test:
	@echo "→ go test ./..."
	@sleep 1
	@echo "ok   all packages   0.42s"

## lint: Run golangci-lint over the codebase [tags=lint] [sc=l]
lint:
	@echo "→ golangci-lint run"
	@sleep 1
	@echo "✓ 0 issues"

## docker-up: Start the local stack [tags=docker,dev]
docker-up:
	@echo "→ docker compose up -d"
	@echo "✓ 3 containers running"

## logs: Follow the app logs [stream] [tags=docker]
logs:
	@echo "streaming app logs… (ctrl+c to stop)"

## deploy: Ship to production [confirm] [tags=deploy] [sc=D]
deploy:
	@echo "→ ./scripts/deploy.sh prod"
	@echo "✓ deployed"

## clean: Remove build artifacts [tags=clean]
clean:
	@echo "✓ cleaned bin/"
