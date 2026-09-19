# huopan-sdk-ts

货盘平台 API 签名客户端（HPS1-HMAC-SHA256）TypeScript 版本，完整类型定义，零第三方依赖（Node 18+）。

协议规范：`docs/api-signature-design.md`

## 获取凭证

登录超管端/卖家端后台 → 「API 凭证」→ 创建，获得 `SecretId` 与 `SecretKey`（明文仅创建时显示一次）。

## 快速开始

```ts
import { HuopanClient } from "./huopan.ts";

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

## 类型定义

```ts
interface ClientOptions { secretId: string; secretKey: string; baseUrl: string; timeout?: number }
interface ApiResponse { statusCode: number; body: string; json<T = unknown>(): T }
type Query = Record<string, QueryValue | QueryValue[]>;
```

## 接口说明

| 方法 | 说明 |
|------|------|
| `new HuopanClient(opts: ClientOptions)` | 创建客户端 |
| `get(path, query?)` / `post(path, body?)` / `put(path, body?)` / `delete(path)` | 便捷方法 |
| `request(method, path, opts?)` | 通用方法 |
| `sign(...): SignHeaders`（导出函数） | 仅计算签名头（纯函数，可测） |

`service` 按路径自动推断：`/seller-api` 前缀 → `seller`，其余 → `admin`。凭证类型与访问端不匹配会返回 `E1007`。

## 错误处理

响应体 `body.code !== 0` 即业务失败；签名/凭证类错误 `code` 为字符串 `E1001`-`E1008`（含义见 `docs/api-signature-design.md` 7.4）。

## 测试

```bash
node --experimental-strip-types huopan_test.ts   # 含黄金向量测试（与服务端跨语言一致性基准）
```

## 完整示例

`sdk/ts/example.ts`：初始化 → GET /api/me（解析身份）→ GET 带中文 query 的列表 → POST 写操作（注释）→ 错误码处理。

```bash
export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE="admin"
# 卖家端凭证: HUOPAN_SCOPE="seller"
node --experimental-strip-types example.ts
```

## 常见问题

- **E1005**：客户端机器时间偏差需在 ±300 秒内
- **E1002**：确认未对 body 重新序列化/编码；query 值空格应为 `%20`（SDK 已处理）
- **SecretKey 丢失**：密钥仅创建时返回一次，丢失后删除凭证重新创建
