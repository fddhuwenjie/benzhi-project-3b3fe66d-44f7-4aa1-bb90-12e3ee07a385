# BENZHI_README

## 项目说明
- 项目：benzhi-project-3b3fe66d-44f7-4aa1-bb90-12e3ee07a385
- 项目用途：口述史脱敏发布资格工作台以授权锁定、逐段转写、确定性检查、精确遮蔽、受访者确认和独立复核形成可追溯闭环，批准后生成不可变发布包并提供摘要校验。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：口述史脱敏发布资格工作台
- 项目介绍：面向档案整理人员的口述史录音发布治理应用，将授权范围、逐段转写、敏感信息遮蔽、受访者确认和独立复核串成一个可追溯的发布资格闭环，最终生成可验证且不可变的发布包。
- 项目概述：面向档案整理人员的口述史录音发布治理应用，将授权范围、逐段转写、敏感信息遮蔽、受访者确认和独立复核串成一个可追溯的发布资格闭环，最终生成可验证且不可变的发布包。
- 核心工作流：整理员建立访谈档案并登记授权证据，按时间码完成转写基线，运行规则检查后逐项遮蔽敏感内容，记录受访者确认，再由不同人员独立复核；驳回时仅重开指定问题，全部通过后封存发布包并进入只读终态。
- 对外接口：Go 服务直接提供一个原生 HTML、CSS 和 JavaScript 浏览器工作台及仅供该页面使用的同源 JSON 端点；工作台以状态栏、转写分段表、问题队列、确认区和发布包校验区完成全部操作，不引入 Node 构建链。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/oralarchive -selfcheck -addr=127.0.0.1:19091

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-3b3fe66d-44f7-4aa1-bb90-12e3ee07a385-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-3b3fe66d-44f7-4aa1-bb90-12e3ee07a385-arm64 linux/arm64

docker run -it benzhi-project-3b3fe66d-44f7-4aa1-bb90-12e3ee07a385-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/oralarchive -selfcheck -addr=127.0.0.1:19091`
