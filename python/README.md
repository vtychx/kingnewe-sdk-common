# huopan-sdk-python

货盘平台 API 签名客户端（HPS1-HMAC-SHA256），**零第三方依赖**（Python 3.8+ 标准库）。

协议规范：`docs/api-signature-design.md`

## 获取凭证

登录超管端/卖家端后台 → 「API 凭证」→ 创建，获得 `SecretId` 与 `SecretKey`（明文仅创建时显示一次）。

## 快速开始

```python
from huopan_sdk import HuopanClient

client = HuopanClient(
    secret_id="HPAKID8XK3MPQZT2WVRYJL4NUC",
    secret_key="你的SecretKey",
    base_url="https://www.kingnewe.com",
)

# GET /api/products?page=1&page_size=20&keyword=货 盘
resp = client.get("/api/products", query={"page": 1, "page_size": 20, "keyword": "货 盘"})
print(resp.status_code, resp.json())

# POST /api/products
resp = client.post("/api/products", body={"name": "测试商品", "price": 9.9})
print(resp.status_code, resp.json())

# PUT / DELETE
resp = client.put("/api/products/123", body={"price": 19.9})
resp = client.delete("/api/products/123")
```

将 `huopan_sdk.py` 放入项目或 `sys.path` 即可使用；也可以复制源码内嵌。

## 接口说明

| 方法 | 说明 |
|------|------|
| `HuopanClient(secret_id, secret_key, base_url, timeout=30)` | 创建客户端 |
| `get(path, query=None)` | 签名 GET（query 支持标量/列表值） |
| `post(path, body=None)` / `put(path, body=None)` | 签名 POST/PUT（body 自动 JSON 序列化，中文原样 UTF-8） |
| `delete(path)` | 签名 DELETE |
| `request(method, path, query=None, body=None)` | 通用方法（body 传 bytes 时签名与发送同源） |
| `HuopanClient.sign(...)`（静态方法） | 仅计算签名头，供测试或自定义传输 |

`service` 按路径自动推断：`/seller-api` 前缀 → `seller`，其余 → `admin`。凭证类型与访问端不匹配会返回 `E1007`。

## 错误处理

```python
from huopan_sdk import HuopanError

try:
    resp = client.get("/api/products")
except HuopanError as e:
    print(e.status_code, e.payload)  # 如 403 / {"code": "E1003", "msg": "凭证已禁用..."}
```

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
python3 test_huopan_sdk.py        # 零依赖运行（含黄金向量测试）
# 或 pytest test_huopan_sdk.py -v
```

## 完整示例

`sdk/python/example.py`：初始化 → GET /api/me（解析身份）→ GET 带中文 query 的列表 → POST 写操作（注释）→ 错误码处理。

```bash
export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE="admin"
# 卖家端凭证: HUOPAN_SCOPE="seller"
python3 example.py
```

## 常见问题

- **E1005 时间戳过期**：客户端机器时间与标准时间偏差需在 ±300 秒内
- **E1002 签名校验失败**：确认未对 body 重新序列化/编码；query 值中的空格应为 `%20` 而非 `+`（SDK 已处理）
- **SecretKey 丢失**：密钥仅创建时返回一次，丢失后删除凭证重新创建
