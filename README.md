# mdlc

[![CI](https://github.com/huey1in/mdlc/actions/workflows/ci.yml/badge.svg)](https://github.com/huey1in/mdlc/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Go Reference](https://img.shields.io/badge/Go_Reference-pkg.go.dev-007d9c)](https://pkg.go.dev/github.com/huey1in/mdlc/mcdsign)
[![Dependencies](https://img.shields.io/badge/dependencies-standard_library-4c1)](go.mod)

Go 请求签名库，仅依赖标准库，支持 Go 1.23+。

库根据调用方提供的请求 URL、方法、请求头和原始正文计算签名，也可以将签名写入已有的 `http.Request`。设备 ID 生成工具用于创建调用方自行保存的逻辑设备标识。

## 能力

| 接口 | 职责 |
|---|---|
| `mcdsign.NewSigner` | 校验签名凭证并创建签名器 |
| `Signer.Sign` | 计算签名，返回请求头和诊断数据 |
| `Signer.SignHTTPRequest` | 为已有 HTTP 请求设置签名头并保留正文 |
| `mcdsign.NewAndroid7036Signer` | 创建版本绑定的兼容签名配置 |
| `mcdsign.NewDeviceID` | 生成 32 位小写十六进制设备 ID |

请求发送、账号登录、短信验证、业务 API、会话存储和重试由使用方负责。库本身没有网络请求和文件持久化，也不会读取环境变量。

## 使用

导入路径为 `github.com/huey1in/mdlc/mcdsign`。以下示例仅在内存中计算签名：

```go
signer, err := mcdsign.NewSigner(mcdsign.Credentials{
    AccessKey: "example-access-key",
    SecretKey: "example-secret-key",
})
if err != nil {
    return err
}

request := &mcdsign.Request{
    Method:  "POST",
    URL:     "https://example.test/resource",
    Body:    []byte(`{"example":true}`),
    Headers: map[string]string{"ct": "102", "language": "cn"},
}
signature, err := signer.Sign(request, mcdsign.SignOptions{})
if err != nil {
    return err
}
_ = signature.Authorization
_ = signature.Digest
```

真实签名凭证由调用方配置。正文必须是最终发送的字节；签名后重新序列化正文或修改参与签名的字段，可能导致签名失效。

设备 ID 使用方式：

```go
deviceID, err := mcdsign.NewDeviceID()
if err != nil {
    return err
}
// 调用方按账号保存 deviceID，后续请求复用同一个值。
```

签名器可以共享，但每个并发操作应使用独立请求对象和请求头。`Signature` 中的正文、规范化请求和授权头可能包含敏感信息，不宜直接写入日志。

## 工程结构

```text
mcdsign/                 签名库、HTTP 请求适配、设备 ID 与测试
docs/design.md           接口约定和设计边界
scripts/check.ps1        本地格式、静态检查、测试与构建
.github/workflows/ci.yml Windows/Linux 自动检查
CONTRIBUTING.md          开发与提交规范
```

本地研究资料位于被 Git 忽略的 `research/`，不属于库的交付范围。

## 验证

```powershell
./scripts/check.ps1
```

也可以直接执行：

```text
go vet ./...
go test -buildvcs=false ./...
go build ./...
```

测试仅构造内存请求，不访问业务服务。CI 在 Windows/Linux 上验证最低 Go 版本及稳定版本，并在 Linux 上运行竞态检查。

仓库尚未声明分发许可证。
