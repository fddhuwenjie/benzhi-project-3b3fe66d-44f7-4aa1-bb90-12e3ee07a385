# 口述史脱敏发布资格工作台

这是一个面向口述史档案整理员、受访者确认联络员和独立档案复核员的发布治理工作台。它把授权草稿修订、增量转写、批量遮蔽、结构化确认、逐项独立复核和发布包封存串成可追溯闭环。工作台提供不含转写正文的待办队列、整改轮次历史和只读发布证明校验。

## 构建、运行与测试

```text
go build ./...
go run ./cmd/oralarchive -addr=127.0.0.1:19091
go test ./...
go run ./cmd/oralarchive -selfcheck -addr=127.0.0.1:19091
```

服务默认监听 `127.0.0.1:19091`，可通过 `-addr=127.0.0.1:<port>` 或 `PORT=<port>` 配置。浏览器打开 `/workbench`，所有写请求都携带 `request_id` 和 `expected_revision`。

队列继续使用 `GET /api/dossiers`，支持 `status`、`editor_id`、`subject_code`、`keyword`、`updated_from`、`updated_to`、`page_size` 和 `cursor`。封存后的分项证明由 `GET /api/dossiers/{id}/release/verification` 只读返回。
