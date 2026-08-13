# gracefulshutdown

[English](README.md)

`gracefulshutdown` 是一個輕量的 Go 服務生命週期協調套件，可同時管理 HTTP
server、worker pool、背景工作與自訂元件。以下情況發生時會開始 graceful
shutdown：

- 上層 context 被取消；
- 收到 `SIGINT` 或 `SIGTERM`；
- 任一元件停止或回傳錯誤。

核心套件僅使用 Go 標準函式庫，最低需求為 Go 1.23。

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
