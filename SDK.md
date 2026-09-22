# Go SDK v0.1.0

非官方 Android API 客户端，仅依赖 Go 标准库。支持 Go 1.23+。

## 本地引用

模块名为 `github.com/huey1in/mdlc`。推送后，其他 Go 项目可引用主分支：

```powershell
go get github.com/huey1in/mdlc@main
```

当前 v0.1.0 是源码版本，不表示已存在远程标签。未指定分发许可证。本地开发可用 `go mod edit -replace=github.com/huey1in/mdlc=C:/Users/29269/Downloads/mdlc`。

## 创建客户端

```go
client, err := mcd.NewClient(mcd.Config{
    Credentials: mcdsign.Credentials{
        AccessKey: os.Getenv("MCD_ACCESS_KEY"),
        SecretKey: os.Getenv("MCD_SECRET_KEY"),
    },
    AESKey: os.Getenv("MCD_AES_KEY"),
    DeviceID: os.Getenv("MCD_DEVICE_ID"),
    SID: os.Getenv("MCD_SID"),
})
if err != nil { return err }
```

导入 `github.com/huey1in/mdlc/mcd`、`github.com/huey1in/mdlc/mcdsign`。SDK 不读环境变量、不自动发送短信、不保存凭证。配置可注入 BaseURL、HTTPClient、Signer 和 APK 设备请求头；客户端创建后可并发使用，不提供可变会话设置器。自定义 Transport/Jar 的并发安全由调用方负责。

版本绑定的 `mcdsign.NewAndroid7036Signer()` 保留供现有命令行兼容；生产集成推荐显式注入 Credentials。SDK 没有默认 AES 密钥，只有短信与登录操作需要它。不要把签名密钥和登录 sid 混淆。

## 接口

| 方法 | 行为 |
|---|---|
| SendSMS(ctx, phone, region, securityVerification) | 发送一次真实短信请求 |
| LoginSMS(ctx, phone, region, code) | 加密手机号、验证码并返回 Session |
| NearbyStores(ctx, latitude, longitude) | 查询附近门店 |
| ReservationTimes(ctx, storeCode) | 查询到店取餐预约时段 |
| Menu(ctx, MenuOptions) | 查询到店菜单；可指定餐段、预约日期和时间 |
| Do(ctx, method, path, query, payload, headers) | 扩展 GET/POST，路径限定同源 /bff/ |

登录成功后创建携带 Session.SID、MeddyID、DeviceID 的新客户端。日期、时间应取自 ReservationTimes，SDK 不自动为用户选择预约时段，也不创建订单。当前便捷查询方法固定到店模式 orderType=1、beType=1；其他模式使用 Do。

## 错误处理

```go
r, err := client.Menu(ctx, mcd.MenuOptions{StoreCode: "1450586", DayPartCode: "8", Date: "2026-09-23", Time: "10:45"})
if errors.Is(err, mcd.ErrNoData) { /* 查询预约时段或提示当前无数据 */ }
var apiErr *mcd.APIError
if errors.As(err, &apiErr) { /* 检查 StatusCode、Code、TraceID */ }
if err != nil { return err }
var data struct { Menu []json.RawMessage `json:"menu"` }
if err := r.Decode(&data); err != nil { return err }
```

示例日期仅演示格式，实际从预约接口获取。API 响应通过 Response.Data 保留未知字段；使用 Decode 解码自己的模型。API 错误保留结构化响应但不会在 Error() 中输出响应正文。成功却无 data 使用 ErrNoData；登录二次验证使用 ErrVerificationRequired。网络与取消错误支持 errors.Is。

默认超时 25 秒、响应上限 16 MiB、禁止自动重定向、无自动重试，避免短信与其他 POST 重复执行。Context 用于取消和截止时间。APIError.Message 为服务端原文，记录日志时仍应脱敏。

## 验证与版本

```powershell
go test -buildvcs=false ./...
go vet ./...
```

测试使用本地 HTTP 服务，不触发真实短信。覆盖 APK 签名和加密、查询规范化、预约菜单、空数据、服务端错误、取消、参数校验与重定向。SDK 0.x 阶段可能调整 API；APK 配置版本独立于 SDK 版本。
