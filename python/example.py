"""huopan-sdk-python 使用示例：初始化客户端 → 签名调用 → 解析响应 → 错误处理。

============================ 凭证与访问端匹配规则 ============================

平台有两类 API 凭证（在对应后台的「API凭证」页面创建），与访问路径一一对应，
不能交叉访问（混用返回 E1007，提示信息会指明该凭证应使用的路径前缀）：

    超管端凭证（在超管后台创建） → 调用 /api/**        路径，service=admin
    卖家端凭证（在卖家后台创建） → 调用 /seller-api/** 路径，service=seller

SDK 会根据请求路径前缀自动推断 service，无需手动指定。

============================ 运行 ============================

运行前准备：
1. 登录超管端或卖家端后台 →「API凭证」→ 创建凭证，保存 SecretId 与 SecretKey
   （明文仅创建时展示一次）
2. 设置环境变量（或直接修改下面的默认值）：
   export HUOPAN_SECRET_ID="HPAKIDxxxx"
   export HUOPAN_SECRET_KEY="你的SecretKey"
   export HUOPAN_BASE_URL="https://www.kingnewe.com"  # 测试环境可用 http://101.42.20.80:8080
   export HUOPAN_SCOPE="admin"    # admin=演示超管端（默认）；seller=演示卖家端
3. 运行：python3 example.py

============================ 响应结构 ============================

平台统一响应外层：{"code":0,"msg":"ok","data":{...}}
  - code==0 表示业务成功
  - 签名/凭证类错误 code 为字符串 "E1001"-"E1008"（E1001 凭证不存在、
    E1002 签名失败、E1003 凭证已禁用、E1004 Nonce 重复、E1005 时间戳过期、
    E1006 签名头格式错误、E1007 端不匹配、E1008 账号状态异常，详见 README）
"""
import json
import os

from huopan_sdk import HuopanClient, HuopanError


def env_or(key: str, fallback: str) -> str:
    return os.environ.get(key) or fallback


def data_excerpt(data) -> str:
    """截断展示 data 便于控制台查看"""
    s = json.dumps(data, ensure_ascii=False) if not isinstance(data, str) else data
    if len(s) > 200:
        s = s[:200] + "..."
    return s or "<无数据>"


def main() -> None:
    # ---------- 1. 初始化客户端 ----------
    client = HuopanClient(
        secret_id=env_or("HUOPAN_SECRET_ID", "HPAKID你的SecretId"),
        secret_key=env_or("HUOPAN_SECRET_KEY", "你的SecretKey"),
        base_url=env_or("HUOPAN_BASE_URL", "https://www.kingnewe.com"),
    )

    # ---------- 2. 按凭证类型演示对应端的调用 ----------
    # 超管端凭证 → HUOPAN_SCOPE=admin（默认）；卖家端凭证 → HUOPAN_SCOPE=seller
    scope = env_or("HUOPAN_SCOPE", "admin").lower()
    if scope == "seller":
        run_seller_examples(client)
    else:
        run_admin_examples(client)
    print("示例执行完成")


def run_admin_examples(client: HuopanClient) -> None:
    """超管端凭证调用 /api/** 接口"""
    # ---- 2.1 GET /api/me：获取当前凭证对应的超管账号身份 ----
    try:
        resp = client.get("/api/me")
    except HuopanError as e:
        # 网络/签名/凭证类失败：e.status_code 为 HTTP 状态码，
        # e.payload 为 {"code": "E1001", "msg": "..."}
        print(f"获取身份失败: HTTP {e.status_code} {e.payload}")
        return
    body = resp.json()
    if body.get("code") != 0:
        print(f"业务失败: {body.get('msg')}")
        return
    admin = (body.get("data") or {}).get("admin") or {}
    if admin:
        print(f"当前身份(超管端): id={admin.get('id')} username={admin.get('username')} role={admin.get('role')}")

    # ---- 2.2 GET 带查询参数（含中文与空格，SDK 自动 RFC3986 编码） ----
    # data 的具体字段以实际接口文档为准，此处演示通用解析方式
    resp = client.get("/api/products", query={"page": 1, "page_size": 5, "keyword": "货 盘"})
    body = resp.json()
    if body.get("code") == 0:
        print(f"商品列表(超管端): {data_excerpt(body.get('data'))}")
    else:
        print(f"商品列表失败: {body.get('msg')}")

    # ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
    # resp = client.post("/api/products", body={"name": "示例商品", "price": 9.9})
    # print("创建商品:", resp.status_code, resp.json())


def run_seller_examples(client: HuopanClient) -> None:
    """卖家端凭证调用 /seller-api/** 接口"""
    # ---- 2.1 GET /seller-api/dashboard：卖家端概览（含店铺与账号信息） ----
    try:
        resp = client.get("/seller-api/dashboard")
    except HuopanError as e:
        print(f"获取卖家概览失败: HTTP {e.status_code} {e.payload}")
        return
    body = resp.json()
    if body.get("code") != 0:
        print(f"业务失败: {body.get('msg')}")
        return
    print(f"卖家概览: {data_excerpt(body.get('data'))}")

    # ---- 2.2 GET /seller-api/products：当前卖家的商品列表（数据按凭证卖家隔离） ----
    resp = client.get("/seller-api/products", query={"page": 1, "page_size": 5})
    body = resp.json()
    if body.get("code") == 0:
        print(f"商品列表(卖家端): {data_excerpt(body.get('data'))}")
    else:
        print(f"商品列表失败: {body.get('msg')}")

    # ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
    # resp = client.post("/seller-api/products", body={"name": "卖家示例商品"})
    # print("创建商品:", resp.status_code, resp.json())


if __name__ == "__main__":
    main()
