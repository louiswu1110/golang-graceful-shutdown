# gracefulshutdown

[English](README.md)

**優雅地停止整個 Go 服務，而不只是 HTTP server。**

`gracefulshutdown` 是零第三方依賴的 Go 服務生命週期協調套件。只用一個
manager，即可同時啟動及停止 HTTP server、Gin、worker pool、背景工作與資源
清理程序。

- 統一管理 server、worker、背景工作與資源清理
- 支援反向循序或平行 shutdown
- 整個 shutdown 流程共用一個可設定的 deadline
- 原生處理 `SIGINT`、`SIGTERM` 與上層 context
- 使用 `errors.Join` 彙整錯誤，可搭配 `errors.Is`、`errors.As`
- 核心 module 零第三方依賴

以下情況發生時會開始 graceful shutdown：

- 上層 context 被取消；
- 收到 `SIGINT` 或 `SIGTERM`；
- 任一元件停止或回傳錯誤。

核心套件僅使用 Go 標準函式庫，最低需求為 Go 1.23。

## 為什麼需要這個套件？

`http.Server.Shutdown` 能等待 HTTP 連線完成，但正式服務通常還包含 worker
pool、背景工作、queue、資料庫連線與 metrics exporter。它們也必須按照正確順序
停止。

你可以自行組合 signal channel、goroutine、timeout 與錯誤處理；這個套件則把
通用協調邏輯整理成小型 API，同時讓每個 component 自己決定如何完成 shutdown。

| 能力 | 自行處理 signal | Framework 專用工具 | `gracefulshutdown` |
| --- | :---: | :---: | :---: |
| HTTP graceful shutdown | ✓ | ✓ | ✓ |
| Gin 與任何 `http.Handler` | 手動 | 通常限定一種 | ✓ |
| Worker 與背景工作 | 手動 | — | ✓ |
| 依相依關係反向關閉 | 手動 | — | ✓ |
| 平行關閉元件 | 手動 | — | ✓ |
| 核心零第三方依賴 | ✓ | 視工具而定 | ✓ |

## 安裝

```bash
go get github.com/louiswu1110/golang-graceful-shutdown
```

## 快速開始

```go
server := &http.Server{
	Addr:              ":8080",
	Handler:           http.DefaultServeMux,
	ReadHeaderTimeout: 5 * time.Second,
}

manager := gracefulshutdown.New(
	gracefulshutdown.WithTimeout(30 * time.Second),
)
manager.Add(
	gracefulshutdown.HTTPServer(server),
	gracefulshutdown.WithName("api"), // 名稱選填，方便辨識錯誤來源
)

if err := manager.Run(context.Background()); err != nil {
	log.Printf("service stopped: %v", err)
}
```

## Gin 與其他 HTTP framework

Gin 實作了 `http.Handler`，可直接交給標準 `http.Server`，不需要讓核心套件依賴
Gin：

```go
router := gin.New()
server := &http.Server{Addr: ":8080", Handler: router}

manager := gracefulshutdown.New()
manager.Add(
	gracefulshutdown.HTTPServer(server),
	gracefulshutdown.WithName("gin-api"),
)
err := manager.Run(context.Background())
```

Chi、Echo、Gorilla Mux 或其他實作 `http.Handler` 的 framework 也使用相同方式。
完整程式請參考 [Gin example](examples/gin)。

## 自訂 server 與 worker pool

有自己生命週期的服務可實作 `Component`：

```go
type Component interface {
	Start(context.Context) error
	Shutdown(context.Context) error
}
```

`Start` 應在元件運行期間持續阻塞；`Shutdown` 應停止接收新工作、完成執行中的
工作，並遵守 context deadline。簡單情況也可使用 `ComponentFuncs`：

```go
manager.Add(gracefulshutdown.ComponentFuncs(
	func(ctx context.Context) error { return pool.Run(ctx) },
	func(ctx context.Context) error { return pool.Shutdown(ctx) },
), gracefulshutdown.WithName("workers"))
```

可執行範例包含 [HTTP server](examples/http/main.go)、[Gin](examples/gin) 與
[worker pool](examples/worker_pool/main.go)。

## Shutdown 順序

預設採循序關閉，順序與註冊順序相反：

```go
manager.Add(database) // 最後關閉
manager.Add(workers)
manager.Add(server)   // 最先關閉
```

彼此獨立的元件可改成平行關閉：

```go
manager := gracefulshutdown.New(
	gracefulshutdown.WithShutdownMode(gracefulshutdown.ShutdownParallel),
)
```

`WithTimeout` 是整個 shutdown 流程共用的 deadline。多個錯誤會以
`errors.Join` 彙整，因此可用 `errors.Is` 與 `errors.As` 判斷。

## 行為說明

- `Manager.Run` 只能呼叫一次。
- 元件即使正常結束也會觸發整體 shutdown；受管理元件應持續運行。
- OS signal 觸發正常關閉，本身不會被當成錯誤回傳；上層 context 取消則會回傳。
- Shutdown callback 應遵守 context。Manager 會按 deadline 返回，但 Go 無法強制
  終止完全忽略 context 的 callback。

## 參與貢獻與授權

歡迎提出 issue 與 pull request，請先閱讀 [CONTRIBUTING.md](CONTRIBUTING.md)。
本專案使用 [MIT License](LICENSE)。
