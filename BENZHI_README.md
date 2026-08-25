# BENZHI_README

基于 Go 实现的buoy-calibration-gate HTTP API 项目，一款后端服务，已完整实现面向海洋观测站的浮标部署前校准放行服务，提供 SQLite 持久化、乐观并发、幂等创建、自动容差判定、偏差整改复测、复核冻结、不可变部署许可、哈希审计时间线和真实 HTTP 全流程自检。

## 项目说明
- 项目：benzhi-project-ee7f84e1-a9f6-4d84-a644-a952a0ce0644
- 项目用途：已完整实现面向海洋观测站的浮标部署前校准放行服务，提供 SQLite 持久化、乐观并发、幂等创建、自动容差判定、偏差整改复测、复核冻结、不可变部署许可、哈希审计时间线和真实 HTTP 全流程自检。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=8s
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-ee7f84e1-a9f6-4d84-a644-a952a0ce0644-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-ee7f84e1-a9f6-4d84-a644-a952a0ce0644-arm64 linux/arm64
docker run -it benzhi-project-ee7f84e1-a9f6-4d84-a644-a952a0ce0644-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=8s`
