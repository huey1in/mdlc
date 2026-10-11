# 设计与接口约定

## 范围与依赖

公开 Go 包为 `github.com/huey1in/mdlc/mcdsign`。模块只有请求签名能力与设备 ID 生成工具，依赖 Go 标准库。

`net/http` 用于描述和修改请求对象。库不创建 HTTP 客户端，不发送请求，不获取或保存账号会话，不读取环境变量，也不处理业务接口。

## 公开接口

```go
func NewSigner(credentials Credentials) (*Signer, error)
func NewAndroid7036Signer() *Signer
func NewDeviceID() (string, error)

func (s *Signer) Sign(req *Request, options SignOptions) (Signature, error)
func (s *Signer) SignHTTPRequest(req *http.Request, query map[string]string, options SignOptions) (Signature, error)
```

`NewSigner` 接收调用方的签名凭证，复制构造时的默认签名头列表。版本绑定的构造函数是既有兼容配置；库使用方应明确选择所需配置，签名凭证与账号会话凭证是不同概念。

## 请求与签名

- `Request.URL` 必须是绝对 URL；URL 中已有的查询字符串优先于 `Request.Query`。
- `Request.Body` 保存最终请求正文的原始字节。库不负责 JSON 序列化。
- `Sign` 会创建缺失的 `Headers` map，并写入签名相关字段。
- `SignOptions.Time` 可固定签名时间；零值使用当前时间。时间偏移及自定义签名头由调用方选择。
- `Signature` 返回授权头、正文摘要和诊断数据。诊断数据可能包含请求正文与会话字段，调用方负责脱敏。

`SignHTTPRequest` 会读取并保留请求正文。请求支持 `GetBody` 时使用独立正文副本；其他情况读取原正文、关闭原 reader，再设置可读取的正文。正文读取失败会返回错误，调用方应终止该请求。

该适配接口中的 `query` 仅用于签名计算，不会自动修改请求 URL。实际请求 URL 与签名所用查询参数的一致性由调用方保证。

## 设备身份

`NewDeviceID` 使用 `crypto/rand` 生成 16 个随机字节，再编码为 32 位小写十六进制字符串。随机数生成错误返回给调用方。

这是逻辑设备标识。调用方应在首次建立账号设备身份时生成并保存，后续复用；签名器不负责账号映射，也不会在每次请求时自动生成设备 ID。

## 并发与错误

构造完成后的 `Signer` 可供多个 goroutine 共享。每次调用应使用独立的 `Request`、`http.Request` 和请求头，因为签名操作会修改请求。

`DefaultSignedHeaders` 是保留的公开配置变量。应在初始化阶段调整，避免与签名器构造并发修改。每个签名器保存自己的副本。

参数、URL 和正文读取错误通过返回值报告。签名器不会自动重试、刷新凭证或发送请求。

## 验证策略

测试检查签名输出、查询处理、请求正文保留和共享签名器的并发使用。所有请求只在内存中构造，不发送到线上服务。

工程检查包含 `gofmt`、`go vet`、`go test` 和 `go build`；CI 另运行竞态检查。模块没有第三方依赖，因此无需生成空的 `go.sum`。
