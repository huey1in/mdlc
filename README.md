# 麦当劳 Android APK 登录工具（Go）

项目包含可复用的 `mcd` 客户端和 `mcdsign` 签名包，SDK 源码版本为 0.1.0。集成方式、参数、错误处理及本地模块引用见 [SDK.md](SDK.md)，版本记录见 [CHANGELOG.md](CHANGELOG.md)。

`cmd/mcd-login` 使用 APK 7.0.36.0 的手机号短信登录流程。

发送验证码并交互登录（会实际发送短信）：

```powershell
go run -buildvcs=false ./cmd/mcd-login -phone 你的手机号 -device-id 你的APK设备ID
```

使用已收到的验证码，不重复发送：

```powershell
go run -buildvcs=false ./cmd/mcd-login -phone 你的手机号 -device-id 同一设备ID -code 123456
```

`-send-only` 仅发送短信。设备 ID 也可通过 `MCD_DEVICE_ID` 设置；分开执行发送和登录时应使用一致的设备上下文。

请求使用 `ct=102`、`language=cn`、`v=7.0.36.0` 和 Android User-Agent。手机号、验证码通过 AES/ECB/PKCS5Padding 加密后编码为小写十六进制。

- 发送：`POST /bff/passport/verifyCode/sms/send`，业务头 `biz_from=1011`。
- 登录：`POST /bff/passport/login/mobile`，业务头 `biz_from=1004`。
- 两者均使用 `biz_scenario=500`。
- APK HMAC 配置包含 `p` 签名头，使用 APK 自身的签名配置。

支持通过 `-device-d` 提供 APK SmSDK 的 d 请求头，通过 `-sec-verify` 提供已完成安全验证的结果。其他上下文包括 `-partner`、`-token`、`-sid`、`-meddy-id`、`-mcd-token`、`-tid`、`-routing-id`。mcdToken 默认使用 device-id，可单独覆盖。工具不生成 SmSDK 设备证明，也不自动完成服务端要求的安全验证。

```powershell
go run -buildvcs=false ./cmd/mcd-login -help
go test -buildvcs=false ./...
```

已通过本地 HTTP 测试验证 APK 路由、请求体、请求头、认证签名和请求体摘要。尚未验证真实短信发送或成功获取 sid。输出包含服务端响应及登录凭证，请自行妥善保管；工具不会保存凭证文件。

签名库使用 `mcdsign.NewAndroid7036Signer()`，支持 APK 的 GET 查询与 POST 请求体签名。

签名规则见 [ALGORITHM.md](ALGORITHM.md)。APK 分析资料和原始接口响应仅在本地 research 目录保留，不提交到仓库。

## 项目目录

- `cmd/mcd-login`：APK 短信登录。
- `cmd/mcd-browse`：携带本地会话查询门店、菜单和预约时段。
- `mcdsign`：Android HMAC 签名与测试。
- `mcd`：统一 HTTP 客户端、短信登录、门店、预约时段及菜单查询 API。
- `tools`：APK Dex 导出工具。
- `research`：APK 解包、反编译、接口响应及诊断记录。
- `麦当劳.apk`：原始 APK；`mcd-apk-session.json`：本地私有会话。

门店查询示例：

```powershell
go run -buildvcs=false ./cmd/mcd-browse
```

休息门店需先查询 `reservationTimeOption`，再携带有效 `date`、`time` 查询菜单；仅传餐段可能返回无 data 的成功响应。
