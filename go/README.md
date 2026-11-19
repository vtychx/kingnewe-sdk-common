# huopan-sdk-go

货盘平台 API 签名客户端（HPS1-HMAC-SHA256），仅依赖 Go 标准库（1.21+）。

协议规范：`docs/api-signature-design.md`

## 获取凭证

登录超管端/卖家端/买家端后台 → 「API 凭证」→ 创建，获得 `SecretId` 与 `SecretKey`（明文仅创建时显示一次）。

## 快速开始

```go
package main

import (
	"fmt"
	"net/url"

	"huopan-sdk-go/huopan"
)

func main() {
	client, err := huopan.New(
		"HPAKID8XK3MPQZT2WVRYJL4NUC",
		"你的SecretKey",
		"https://www.kingnewe.com",
	)
	if err != nil {
		panic(err)
	}

	// GET /api/products?page=1&page_size=20&keyword=货 盘
	resp, err := client.Get("/api/products", url.Values{
		"page": {"1"}, "page_size": {"20"}, "keyword": {"货 盘"},
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(resp.StatusCode, string(resp.Body))

	// POST /api/products
	resp, err = client.Post("/api/products", map[string]any{
		"name": "测试商品", "price": 9.9,
	})
	if err != nil {
		panic(err)
	}
	var out map[string]any
	_ = resp.JSON(&out)
	fmt.Println(out)
}
```

## 接口说明

| 方法 | 说明 |
|------|------|
| `New(secretID, secretKey, baseURL)` | 创建客户端；baseURL 不带尾斜杠 |
| `Get(path, query url.Values)` | 签名 GET |
| `Post(path, body)` / `Put(path, body)` | 签名 POST/PUT（body 自动 JSON 序列化，中文原样 UTF-8） |
| `Delete(path)` | 签名 DELETE |
| `DoRequest(method, path, query, body []byte)` | 通用方法（body 为原始字节，签名与发送同源） |

`service` 按路径自动推断：`/buyer-api` 前缀 → `buyer`，`/seller-api` 前缀 → `seller`，其余 → `admin`。**凭证类型与路径前缀必须匹配**：买家端凭证只能调用 `/buyer-api/**`，卖家端凭证只能调用 `/seller-api/**`，超管端凭证只能调用 `/api/**`，混用返回 `E1007`。

## 错误码

| code | HTTP | 含义 |
|------|------|------|
| E1001 | 401 | 凭证不存在 |
| E1002 | 401 | 签名校验失败 |
| E1003 | 403 | 凭证已禁用 |
| E1004 | 403 | 重复请求（Nonce 已使用） |
| E1005 | 401 | 请求时间戳过期（校准系统时间） |
| E1006 | 400 | 签名头缺失或格式错误 |
| E1007 | 403 | 凭证类型与访问端不匹配 |
| E1008 | 403 | 账号状态异常 |

## 测试

```bash
go test ./... -v   # 含黄金向量测试（与服务端跨语言一致性基准）
```

## 完整示例

`sdk/go/example/main.go`：初始化 → 按 `HUOPAN_SCOPE` 演示三端（admin：身份与商品列表；seller：概览与商品列表；buyer：商品列表与商品详情）→ POST 写操作（注释）→ 错误码处理。买家端示例会打印 `skus[].price` 与 `skus[].stock`。

```bash
export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE="admin"
# 卖家端凭证: HUOPAN_SCOPE="seller"
# 买家端凭证: HUOPAN_SCOPE="buyer"
go run ./example
```

## 常见问题

- **E1005 时间戳过期**：客户端机器时间与标准时间偏差需在 ±300 秒内
- **E1002 签名校验失败**：确认未对 body 重新序列化/编码；query 值中的空格应为 `%20` 而非 `+`（SDK 已处理）
- **SecretKey 丢失**：密钥仅创建时返回一次，丢失后删除凭证重新创建
