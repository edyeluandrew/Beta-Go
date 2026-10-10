@echo off
if "%1"=="" goto help
if "%1"=="help" goto help
if "%1"=="run" goto run
if "%1"=="web" goto web
if "%1"=="test" goto test
if "%1"=="test-docker" goto testdocker
if "%1"=="build" goto build
echo Unknown command: %1
goto help

:help
echo make run         start the API on :8080
echo make web         start the React app on :5173
echo make test        run Go tests on this machine
echo make test-docker run Go tests in Docker
echo make build       typecheck and build the web app
goto :eof

:run
cd /d "%~dp0apps\api" && go run ./cmd/api
goto :eof

:web
cd /d "%~dp0" && npm --prefix apps/web run dev
goto :eof

:test
cd /d "%~dp0apps\api" && go test ./...
goto :eof

:testdocker
cd /d "%~dp0" && docker run --rm -v "%cd%/apps/api:/app" -w /app golang:1.26 go test ./...
goto :eof

:build
cd /d "%~dp0" && npm --prefix apps/web run build
goto :eof
