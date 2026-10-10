.PHONY: help run web test test-docker build

help:
	@echo make run         start the API on :8080
	@echo make web         start the React app on :5173
	@echo make test        run Go tests on this machine
	@echo make test-docker run Go tests in Docker (use this if Windows blocks go test)
	@echo make build       typecheck and build the web app

run:
	cd apps/api && go run ./cmd/api

web:
	npm --prefix apps/web run dev

test:
	cd apps/api && go test ./...

test-docker:
	docker run --rm -v "$(CURDIR)/apps/api:/app" -w /app golang:1.26 go test ./...

build:
	npm --prefix apps/web run build
