# huopan-sdk-csharp

货盘平台 API 签名客户端（HPS1-HMAC-SHA256），仅依赖 .NET 标准库（.NET 8+，已在 .NET 11 验证）。
协议规范：`docs/api-signature-design.md`

## 文件

| 文件 | 说明 |
|------|------|
| `HuopanClient.cs` | SDK 本体：签名 + HTTP 客户端 + 响应/异常类型 |
| `Example.cs` | 完整示例（三端）+ 命令行入口 + `--selftest` 黄金向量自检 |
| `HuopanExample.csproj` | 项目文件 |

## 快速开始

```bash
cd csharp
dotnet run -- --secret-id HPAKID... --secret-key 你的SecretKey \
  --base-url https://www.kingnewe.com --scope buyer
```

也可使用环境变量（与其余语言 SDK 一致）：

```bash
export HUOPAN_SECRET_ID="HPAKID..."
export HUOPAN_SECRET_KEY="你的SecretKey"
export HUOPAN_BASE_URL="https://www.kingnewe.com"
export HUOPAN_SCOPE="buyer"   # admin(默认) / seller / buyer
dotnet run
```

> 请勿把真实 SecretKey 硬编码进仓库；示例同时支持命令行参数与环境变量，优先命令行。

## 自检（无需网络/凭证）

```bash
dotnet run -- --selftest
```

校验黄金向量签名、canonical query 编码、Host 规则、空 body 哈希，结果须全部 `[PASS]`。

## API

| 方法 | 说明 |
|------|------|
| `new HuopanClient(secretId, secretKey, baseUrl)` | 创建客户端；baseUrl 不带尾斜杠 |
| `Get(path, query)` / `Post(path, json)` / `Put(path, json)` / `Delete(path)` | 签名请求 |
| `DoRequest(method, path, query, jsonBody)` | 通用方法 |
| `HuopanClient.Sign(...)` | 纯签名函数，返回含 `Authorization` 的请求头字典 |
| `HuopanResponse.Json()` | `System.Text.Json.JsonDocument` 解析响应 |

`service` 按路径前缀自动推断：`/buyer-api` → `buyer`，`/seller-api` → `seller`，其余 → `admin`。
**凭证类型与路径前缀必须匹配**，混用返回 `E1007`。

## 错误码

| code | HTTP | 含义 |
|------|------|------|
| E1001 | 401 | 凭证不存在 |
| E1002 | 401 | 签名校验失败 |
| E1003 | 403 | 凭证已禁用 |
| E1004 | 403 | 重复请求（Nonce 已使用） |
| E1005 | 401 | 时间戳过期（校准系统时间，±300 秒） |
| E1006 | 400 | 签名头缺失或格式错误 |
| E1007 | 403 | 凭证类型与访问端不匹配 |
| E1008 | 403 | 账号状态异常 |

## 注意事项

- 未携带签名头访问 `/buyer-api/**` 查询接口时返回游客视图（`code` 仍为 0，但 `skus[].price`、`skus[].stock` 为 0）；签名调用才返回真实值。
- body 签名与发送使用同一份 UTF-8 字节，禁止二次序列化；中文保持原样。
- query 值中空格编码为 `%20` 而非 `+`（SDK 已处理）。
