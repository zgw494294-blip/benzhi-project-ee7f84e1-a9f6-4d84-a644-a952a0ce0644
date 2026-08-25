# buoy-calibration-gate

`buoy-calibration-gate` 是面向海洋观测站的浮标部署前校准放行服务。它通过 JSON HTTP API 把浮标传感器配置基线、不可变校准证据、自动容差判定、偏差整改、技术复核、冻结清单和部署许可约束在一条可追溯流程内。

服务使用 SQLite 持久化业务实体、幂等结果和哈希链审计事件。写操作在事务中提交，并通过 `X-Expected-Version` 实现乐观并发控制；创建档案和初次校准运行通过 `Idempotency-Key` 防止重复。冻结后禁止修改传感器、校准运行和复核结论，许可提供可离线复算的 SHA-256 校验材料及分层核验结果。

## 构建与测试

```text
go build ./...
go test ./...
```

## 运行

默认只监听高位回环地址 `127.0.0.1:19081`，并在当前目录创建 `calibration.db`：

```text
go run ./cmd/server
```

可以用 `-addr` 指定监听地址，也可以设置 `PORT` 端口号；使用 `PORT` 时始终绑定 `127.0.0.1:<PORT>`。SQLite 路径可以用 `-db` 指定。

```text
go run ./cmd/server -addr=127.0.0.1:19120 -db=station.db
PORT=19121 go run ./cmd/server
```

执行真实 HTTP 全流程自检并在限定时间内自行退出：

```text
go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck -selfcheck-timeout=8s
```

## API 约定

写请求使用 `X-Actor` 和 `X-Role` 标识操作者及角色。角色可取 `engineer`、`reviewer`、`deployer`。除创建档案外，修改档案的请求还必须携带 `X-Expected-Version`。创建档案和提交初次运行必须携带 `Idempotency-Key`；重放响应带 `Idempotent-Replayed: true`。普通请求体限制为 1 MiB，批量整改限制为 256 KiB 和最多 20 项，未知 JSON 字段会被拒绝。

主要资源位于 `/api/v1/calibration-dossiers`，流程依次为：创建档案、登记温度/盐度/溶解氧三类基线、逐传感器提交运行、整改偏差、复核批准冻结、签发部署许可。证据摘要统一使用 `sha256:<64 位十六进制>` 格式。

档案详情 `GET /api/v1/calibration-dossiers/{dossierID}` 除原始实体外还返回实时 `progress`、`nextActions` 和按时间排序的 `remediationAttempts`。整改既可逐项提交，也可通过 `POST /api/v1/calibration-dossiers/{dossierID}/deviations/remediations:batch` 原子批量提交。复核员应先调用 `GET /api/v1/calibration-dossiers/{dossierID}/reviews/preflight` 获取阻塞项和 `previewDigest`，再把摘要随批准请求提交。

`GET /api/v1/calibration-dossiers/{dossierID}/timeline` 返回经验证的审计时间线。`GET /api/v1/deployment-permits/{permitNumber}/verify` 返回许可哈希、冻结版本、清单摘要、审计链、关联关系和签发事件的逐项结果；可读取但核验失败的许可仍返回 HTTP 200，并通过 `valid` 与 `reasonCodes` 表明异常。
