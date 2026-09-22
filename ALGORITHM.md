# Android APK HMAC v4

配置来自 APK 7.0.36.0 的 SecBox 和 MCDSignatureInterceptor。

规范串依次为大写 HTTP 方法、解码后的路径（移除花括号）、排序的 URL 编码查询串、access key、GMT 时间、请求头块；各段以 LF 分隔，末尾保留 LF。

查询串按 & 拆分并排序，再还原 %28、%29、%27、%21、%7E；不二次编码。实际 URL 是签名数据来源，保留重复参数。

签名头按顺序为 ct、language、p、sid、sv、token、v、x-mcd-gw-v，缺失值为空。APK 使用 ct=102、language=cn、v=7.0.36.0，强制 sv=v4。

authorization 格式：

```text
hmac-auth-v1#accessKey#Base64(HMAC-SHA256(secretKey,规范串))#hmac-sha256#GMT时间#ct;language;p;sid;sv;token;v;x-mcd-gw-v
```

x-hmac-digest 单独计算请求体原始字节的 Base64(HMAC-SHA256)。当前 GET 查询没有请求体。

使用 mcdsign.NewAndroid7036Signer()；密钥随客户端版本变化。
