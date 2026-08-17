# BENZHI_README

## 项目说明
- 项目：benzhi-project-27bbaef6-08fb-4e01-a962-02ba6550efda
- 项目用途：PanelNest is a standard-library Go CLI for deterministic kerf-aware rectangular sheet-goods cutting plans with draft and commit workflows, immutable receipts, offcuts, and atomic JSON ledger persistence.
- Go 工具链：`golang:1.22.0`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/panelnest stock-list
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-27bbaef6-08fb-4e01-a962-02ba6550efda-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-27bbaef6-08fb-4e01-a962-02ba6550efda-arm64 linux/arm64
docker run -it benzhi-project-27bbaef6-08fb-4e01-a962-02ba6550efda-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/panelnest smoke`
