"""Python SDK 黄金向量与编码规则测试。

运行：python3 test_huopan_sdk.py 或 pytest test_huopan_sdk.py
黄金向量与 docs/api-signature-design.md 6.4 及服务端/Go SDK 一致。
"""
import sys

from huopan_sdk import (
    ALGORITHM,
    SIGNED_HEADERS,
    HuopanClient,
    _canonical_query,
    _hash_payload,
    _rfc3986,
)

GOLDEN_SECRET_ID = "HPAKIDTESTSECRETID00000001"
GOLDEN_SECRET_KEY = "TestSecretKey0123456789abcdefABCDEF"
GOLDEN_NONCE = "5f8a1c2e9b4d4f6a8e0c7a3b5d9e1f2c"
GOLDEN_TS = "1726646400"  # UTC 2024-09-18
GOLDEN_SIGNATURE = "c4a39375035c1d8d0eeb16e5b922dcac5d9eba27e3f6ed8b3047599e3cec6079"
GOLDEN_URL = "https://www.kingnewe.com/api/products?page=1&page_size=20&keyword=货 盘"


def test_golden_vector():
    """黄金向量：签名必须与服务端/Go SDK 逐字节一致"""
    headers = HuopanClient.sign(
        GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "POST", GOLDEN_URL,
        body='{"name":"测试"}'.encode("utf-8"), nonce=GOLDEN_NONCE, ts=GOLDEN_TS,
    )
    want = (
        "HPS1-HMAC-SHA256 Credential=HPAKIDTESTSECRETID00000001/2024-09-18/admin/hps1_request, "
        f"SignedHeaders={SIGNED_HEADERS}, "
        f"Signature={GOLDEN_SIGNATURE}"
    )
    assert headers["Authorization"] == want
    assert headers["x-hp-service"] == "admin"


def test_canonical_query_encoding():
    qseq = [("keyword", "货 盘"), ("page", "1"), ("page_size", "20")]
    assert _canonical_query(qseq) == "keyword=%E8%B4%A7%20%E7%9B%98&page=1&page_size=20"
    # / 在 query 值中必须编码为 %2F
    assert _rfc3986("a/b") == "a%2Fb"
    # 路径编码保留 /
    assert _rfc3986("a/b", keep_slash=True) == "a/b"


def test_hash_payload_empty():
    want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
    assert _hash_payload(None) == want
    assert _hash_payload(b"") == want


def test_seller_service_inference():
    """/seller-api 前缀自动推断 service=seller"""
    headers = HuopanClient.sign(
        GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
        "https://www.kingnewe.com/seller-api/dashboard?x=1",
        nonce=GOLDEN_NONCE, ts=GOLDEN_TS,
    )
    assert headers["x-hp-service"] == "seller"
    assert "/seller" in headers["Authorization"]


def test_host_rules():
    """默认端口省略，非默认端口保留"""
    headers = HuopanClient.sign(
        GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
        "http://127.0.0.1:8080/api/me", nonce=GOLDEN_NONCE, ts=GOLDEN_TS,
    )
    assert headers["host"] == "127.0.0.1:8080"
    headers = HuopanClient.sign(
        GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
        "https://www.kingnewe.com:443/api/me", nonce=GOLDEN_NONCE, ts=GOLDEN_TS,
    )
    assert headers["host"] == "www.kingnewe.com"


def test_client_validation():
    try:
        HuopanClient("", "key", "https://x.com")
        raise AssertionError("空 secret_id 应报 ValueError")
    except ValueError:
        pass
    client = HuopanClient("id", "key", "https://x.com/")
    assert client.base_url == "https://x.com"


if __name__ == "__main__":
    fns = [v for k, v in sorted(globals().items()) if k.startswith("test_")]
    failed = 0
    for fn in fns:
        try:
            fn()
            print(f"[PASS] {fn.__name__}")
        except Exception as exc:  # noqa: BLE001
            failed += 1
            print(f"[FAIL] {fn.__name__}: {exc}")
    print(f"===== 结果: {len(fns) - failed}/{len(fns)} 通过 =====")
    sys.exit(1 if failed else 0)
