# huopan-sdk-java

货盘平台 API 签名客户端（HPS1-HMAC-SHA256），单文件零依赖（JDK 11+）。

协议规范：`docs/api-signature-design.md`

## 快速开始

```java
import java.util.Map;

HuopanClient client = new HuopanClient(
        "HPAKID8XK3MPQZT2WVRYJL4NUC",
        "你的SecretKey",
        "https://www.kingnewe.com");

// GET（query 键值对）
String resp = client.get("/api/products",
        Map.of("page", "1", "page_size", "20", "keyword", "货 盘"));

// POST（body 为调用方序列化好的 JSON 字符串）
String resp2 = client.post("/api/products", "{\"name\":\"测试商品\",\"price\":9.9}");

// PUT / DELETE
client.put("/api/products/123", "{\"price\":19.9}");
client.delete("/api/products/123");
```

## 编译与自检

```bash
javac HuopanClient.java
java HuopanClient selftest   # 黄金向量自检（须输出 PASS）
```

## 完整示例

`sdk/java/Example.java`：初始化 → 按 `HUOPAN_SCOPE` 演示三端（admin：身份与商品列表；seller：概览与商品列表；buyer：商品列表与商品详情）→ POST 写操作（注释）→ 错误码处理。买家端示例会打印 `skus[].price` 与 `skus[].stock`。

```bash
export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE="admin"
# 卖家端凭证: HUOPAN_SCOPE="seller"
# 买家端凭证: HUOPAN_SCOPE="buyer"
javac Example.java && java Example
```

## 接口说明

| 方法 | 说明 |
|------|------|
| `HuopanClient(secretId, secretKey, baseUrl)` | 创建客户端 |
| `get(path, query)` | 签名 GET |
| `post(path, jsonBody)` / `put(path, jsonBody)` | 签名 POST/PUT（body 为 JSON 字符串） |
| `delete(path)` | 签名 DELETE |
| `sign(secretId, secretKey, method, rawUrl, body, nonce, ts)`（静态） | 仅计算签名头，供测试 |

`service` 按路径自动推断：`/buyer-api` 前缀 → `buyer`，`/seller-api` 前缀 → `seller`，其余 → `admin`。凭证类型与访问端不匹配会返回 `E1007`。

## 错误码

同 `docs/api-signature-design.md` 7.4：E1001 凭证不存在 / E1002 签名失败 / E1003 凭证已禁用 /
E1004 Nonce 重复 / E1005 时间戳过期 / E1006 签名头格式错误 / E1007 端不匹配 / E1008 账号状态异常。

## 常见问题

- **E1005**：客户端机器时间偏差需在 ±300 秒内
- **E1002**：确认 body 字节未被重新编码；query 值空格应为 `%20`（SDK 已处理）
- **路径含动态段**：传入的 path 若含非 ASCII/特殊字符，请先自行做 RFC3986 编码（`HuopanClient.rfc3986Encode(path, true)`）
