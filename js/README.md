# huopan-sdk-js

货盘平台 API 签名客户端（HPS1-HMAC-SHA256），ESM 单文件，零第三方依赖（Node 18+ 原生 fetch 与 node:crypto）。

协议规范：`docs/api-signature-design.md`

## 获取凭证

登录超管端/卖家端/买家端后台 → 「API 凭证」→ 创建，获得 `SecretId` 与 `SecretKey`（明文仅创建时显示一次）。

## 快速开始

```js
import { HuopanClient } from "./huopan.mjs";

const client = new HuopanClient({
  secretId: "HPAKID8XK3MPQZT2WVRYJL4NUC",
  secretKey: "你的SecretKey",
  baseUrl: "https://www.kingnewe.com",
});

// GET /api/products?page=1&page_size=20&keyword=货 盘
const resp = await client.get("/api/products", { page: 1, page_size: 20, keyword: "货 盘" });
console.log(resp.statusCode, resp.json());

// POST /api/products
const resp2 = await client.post("/api/products", { name: "测试商品", price: 9.9 });

// PUT / DELETE
await client.put("/api/products/123", { price: 19.9 });
await client.delete("/api/products/123");
```

## 接口说明

| 方法 | 说明 |
|------|------|
| `new HuopanClient({ secretId, secretKey, baseUrl, timeout })` | 创建客户端 |
| `get(path, query)` | 签名 GET |
| `post(path, body)` / `put(path, body)` | 签名 POST/PUT（body 自动 JSON 序列化，中文原样 UTF-8） |
| `delete(path)` | 签名 DELETE |
| `request(method, path, { query, body })` | 通用方法 |
| `sign(secretId, secretKey, method, urlStr, body, opts)` | 仅计算签名头（纯函数，可测） |

`service` 按路径自动推断：`/buyer-api` 前缀 → `buyer`，`/seller-api` 前缀 → `seller`，其余 → `admin`。凭证类型与访问端不匹配会返回 `E1007`。

## 错误处理

```js
const resp = await client.get("/api/me");
const body = resp.json();
if (body.code !== 0) {
  // 业务失败：body.code 为数字错误码或字符串签名错误码（E1001-E1008，见 docs 7.4）
  console.log(body.msg);
}
```

| code | HTTP | 含义 |
|------|------|------|
| E1001 | 401 | 凭证不存在 |
| E1002 | 401 | 签名校验失败 |
| E1003 | 403 | 凭证已禁用 |
| E1004 | 403 | 重复请求（Nonce 已使用） |
| E1005 | 401 | 请求时间戳过期 |
| E1006 | 400 | 签名头缺失或格式错误 |
| E1007 | 403 | 凭证类型与访问端不匹配 |
| E1008 | 403 | 账号状态异常 |

## 测试

```bash
node huopan_test.mjs   # 含黄金向量测试（与服务端跨语言一致性基准）
```

## 完整示例

`sdk/js/example.mjs`：初始化 → 按 `HUOPAN_SCOPE` 演示三端（admin：身份与商品列表；seller：概览与商品列表；buyer：商品列表与商品详情）→ POST 写操作（注释）→ 错误码处理。买家端示例会打印 `skus[].price` 与 `skus[].stock`。

```bash
export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE="admin"
# 卖家端凭证: HUOPAN_SCOPE="seller"
# 买家端凭证: HUOPAN_SCOPE="buyer"
node example.mjs
```

## 常见问题

- **E1005**：客户端机器时间偏差需在 ±300 秒内
- **E1002**：确认未对 body 重新序列化/编码；query 值空格应为 `%20`（SDK 已处理）
- **SecretKey 丢失**：密钥仅创建时返回一次，丢失后删除凭证重新创建
