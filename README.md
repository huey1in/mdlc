# mdlc

[![CI](https://github.com/huey1in/mdlc/actions/workflows/ci.yml/badge.svg)](https://github.com/huey1in/mdlc/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?logo=go&logoColor=white)](https://go.dev/dl/)
[![Go Reference](https://img.shields.io/badge/Go_Reference-pkg.go.dev-007d9c)](https://pkg.go.dev/github.com/huey1in/mdlc/mcdsign)
[![Dependencies](https://img.shields.io/badge/dependencies-standard_library-4c1)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![LINUX DO 社区](https://img.shields.io/badge/Community-LINUX%20DO-FFB003)](https://linux.do/)

麦当劳中国 Android App Go 签名库，生成 `Authorization`、`X-HMAC-DIGEST` 请求头，并提供设备 ID 生成。

[安装](#安装) · [快速开始](#快速开始) · [设备 ID](#设备-id) · [接口参考](#接口参考) · [开发与贡献](#开发与贡献) · [免责声明](#免责声明)

## 功能与范围

- 根据请求方法、URL、请求头和原始正文计算签名。
- 为已有的 `http.Request` 写入签名头，并保留可读取的请求正文。
- 提供麦当劳 Android App `7.0.36.0` 的内置签名配置，也支持调用方传入签名凭证。
- 生成 32 位小写十六进制设备 ID，供调用方按账号保存和复用。

## 安装

需要 **Go 1.23 或更新版本**，仅依赖 Go 标准库。

```sh
go get github.com/huey1in/mdlc/mcdsign
```

```go
import "github.com/huey1in/mdlc/mcdsign"
```

## 快速开始

下面的完整示例为已有的 HTTP 请求添加签名头。示例地址和凭证均为占位值，程序只在内存中签名，不发送请求。

```go
package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/huey1in/mdlc/mcdsign"
)

func main() {
	signer, err := mcdsign.NewSigner(mcdsign.Credentials{
		AccessKey: "example-access-key",
		SecretKey: "example-secret-key",
	})
	if err != nil {
		log.Fatal(err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		"https://example.test/resource?source=app",
		strings.NewReader(`{"example":true}`),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer req.Body.Close()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ct", "102")
	req.Header.Set("language", "cn")

	if _, err := signer.SignHTTPRequest(req, nil, mcdsign.SignOptions{}); err != nil {
		log.Fatal(err)
	}

	fmt.Println("Authorization 已设置:", req.Header.Get("Authorization") != "")
	fmt.Println("X-HMAC-DIGEST 已设置:", req.Header.Get("X-HMAC-DIGEST") != "")
}
```

`SignHTTPRequest` 会写入 `Authorization`、`X-HMAC-DIGEST`、`sv` 和 `x-mcd-gw-v`。签名成功后，请求正文仍可供调用方读取或发送。

### 选择签名配置

使用内置的 Android App `7.0.36.0` 配置时，可将上例的签名器初始化替换为：

```go
signer := mcdsign.NewAndroid7036Signer()
```

该配置与指定 App 版本绑定。需要使用自己的签名凭证时，通过 `NewSigner` 传入 `AccessKey` 和 `SecretKey`；两者均不能为空。签名凭证与账号的会话凭证分别由调用方配置。

### 直接计算签名

如果已有自己的请求结构，可以使用 `Signer.Sign`，无需先创建 `http.Request`：

```go
func signRequest(signer *mcdsign.Signer) (mcdsign.Signature, error) {
	req := &mcdsign.Request{
		Method: "POST",
		URL:    "https://example.test/resource?source=app",
		Body:   []byte(`{"example":true}`),
		Headers: map[string]string{
			"ct":       "102",
			"language": "cn",
		},
	}
	return signer.Sign(req, mcdsign.SignOptions{})
}
```

返回值中的 `Signature.Authorization` 和 `Signature.Digest` 分别对应两个签名头。`Sign` 同时会把签名头及版本字段写入 `Request.Headers`，调用方可将这些请求头应用到实际请求。

## 设备 ID

`NewDeviceID` 使用 `crypto/rand` 生成 16 个随机字节，再编码为 32 位小写十六进制字符串。这是随机生成的逻辑设备标识，不读取硬件信息。

```go
func createAccountDeviceID() (string, error) {
	deviceID, err := mcdsign.NewDeviceID()
	if err != nil {
		return "", err
	}
	// 调用方将 deviceID 与账号关联保存，后续请求读取并复用。
	return deviceID, nil
}
```

每个账号首次建立设备身份时生成一次，由调用方保存。后续请求复用该账号的设备 ID，避免每次请求重新生成，也避免所有账号共用一个值。签名器不会自动生成、保存或注入设备 ID。

## 使用约定

- **URL 与查询参数**：传入绝对 URL，建议直接将查询参数放入实际请求 URL。URL 中已有的查询串优先于 `Request.Query` 和 `SignHTTPRequest` 的 `query` 参数；后两者仅在 URL 没有查询串时用于签名，不会自动补到实际 URL 中。
- **正文**：签名使用最终发送的原始字节，JSON 等序列化应在签名前完成。`GET` 请求按空正文计算摘要；其他方法使用提供的正文。签名后修改正文或参与签名的字段，需要重新签名。
- **请求头**：在签名前设置业务接口所需的请求头和会话字段。库生成签名相关字段，业务参数的完整性由调用方保证。
- **时间**：`SignOptions.Time` 为零时使用当前时间；可通过 `Time` 固定签名时间，通过 `TimeOffset` 应用时间偏移。
- **并发**：构造完成的 `Signer` 可以共享；每次并发签名应使用独立的请求对象和请求头。默认签名头配置应在初始化阶段调整，单次请求可通过 `SignOptions.SignedHeaders` 覆盖。
- **错误与日志**：处理构造、URL 解析和正文读取等操作返回的错误。`Signature` 中的授权头、正文与规范化请求可能含有敏感信息，记录日志前应脱敏。

更详细的参数语义、正文读取行为和并发约定见 [设计与接口约定](docs/design.md)。

## 接口参考

| 接口 | 用途 |
|---|---|
| `mcdsign.NewAndroid7036Signer()` | 使用 Android App 7.0.36.0 的内置签名配置 |
| `mcdsign.NewSigner(credentials)` | 校验调用方提供的凭证并创建签名器 |
| `Signer.Sign(req, options)` | 签名 `mcdsign.Request`，返回结果并更新请求头 |
| `Signer.SignHTTPRequest(req, query, options)` | 签名已有 HTTP 请求，保留正文并更新请求头 |
| `mcdsign.NewDeviceID()` | 生成设备 ID，随机数生成失败时返回错误 |

完整类型和方法文档见 [Go Reference](https://pkg.go.dev/github.com/huey1in/mdlc/mcdsign)。

## 开发与贡献

```text
mcdsign/                  签名实现、HTTP 请求适配、设备 ID 与测试
docs/design.md            设计与接口约定
scripts/check.ps1         本地工程检查
.github/workflows/ci.yml  Windows/Linux CI
CONTRIBUTING.md           开发与提交规范
LICENSE                   MIT 开源协议
```

在仓库根目录运行 PowerShell 检查脚本，检查格式、静态分析、测试与构建：

```powershell
./scripts/check.ps1
```

其他环境可直接执行：

```sh
go fmt ./...
go vet ./...
go test -buildvcs=false ./...
go build ./...
```

测试使用内存请求，不访问业务服务。CI 在 Windows/Linux 上检查 Go 1.23 和稳定版本，并在 Linux 上执行竞态检查。已配置 C 编译器的本地环境也可运行 `./scripts/check.ps1 -Race`。

提交问题或改动前，请阅读 [贡献指南](CONTRIBUTING.md)。涉及签名行为或公开接口的变更，应同步更新测试和设计文档。

## 免责声明

- 本项目为非官方实现，与麦当劳及其关联公司无隶属或合作关系。
- 请在合法授权范围内使用，遵守适用法律与相关服务条款，不得用于未授权访问或接口滥用。
- 软件按“现状”提供，不保证持续兼容服务端；许可与责任限制见 [MIT 协议](LICENSE)。

## 许可证

本项目采用 [MIT 开源协议](LICENSE)。
