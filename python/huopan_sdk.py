"""货盘平台 API 签名客户端（HPS1-HMAC-SHA256）。

协议规范：docs/api-signature-design.md 第 6 章。
零第三方依赖（仅 Python 3.8+ 标准库）。

用法：
    from huopan_sdk import HuopanClient

    client = HuopanClient(
        secret_id="HPAKID...",
        secret_key="你的SecretKey",
        base_url="https://www.kingnewe.com",
    )
    resp = client.post("/api/products", body={"name": "测试商品"})
    print(resp.status_code, resp.json())
"""
import hashlib
import hmac
import json
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid

ALGORITHM = "HPS1-HMAC-SHA256"
SIGNED_HEADERS = "host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp"


class HuopanError(Exception):
    """SDK 本地错误（网络/协议层）"""

    def __init__(self, status_code, payload):
        super().__init__(f"HTTP {status_code}: {payload}")
        self.status_code = status_code
        self.payload = payload


class Response:
    """响应体"""

    def __init__(self, status_code, body):
        self.status_code = status_code
        self.body = body

    def json(self):
        return json.loads(self.body or "{}")


def _rfc3986(value, keep_slash=False):
    """RFC3986 编码：unreserved 集合不编码；keep_slash 时保留路径分隔符"""
    safe = "-_.~" + ("/" if keep_slash else "")
    return urllib.parse.quote(str(value), safe=safe)


def _canonical_query(qseq):
    """规范 query：先编码每对 k/v，再按 (k, v) 字节序排序"""
    pairs = sorted((_rfc3986(k), _rfc3986(v)) for k, v in qseq)
    return "&".join(f"{k}={v}" for k, v in pairs)


def _canonical_headers(headers):
    """规范头：按名称小写字典序，value 压缩连续空白"""
    return "".join(
        f"{name}:{' '.join(str(headers[name]).split())}\n" for name in sorted(headers)
    )


def _sign_host(parsed):
    """签名用 Host：默认端口 80/443 省略，小写"""
    host = (parsed.hostname or "").lower()
    port = parsed.port
    if port is not None and port not in (80, 443):
        host = f"{host}:{port}"
    return host


def _hash_payload(body):
    """body 原始字节 SHA256 小写 hex（None 等价空串）"""
    return hashlib.sha256(body or b"").hexdigest()


def _hmac(key, msg):
    return hmac.new(key, msg, hashlib.sha256).digest()


class HuopanClient:
    """API 签名客户端（线程安全，可复用）"""

    def __init__(self, secret_id, secret_key, base_url, timeout=30, http=None):
        if not secret_id or not secret_key or not base_url:
            raise ValueError("secret_id / secret_key / base_url 均不能为空")
        self.secret_id = secret_id
        self.secret_key = secret_key
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self._opener = http or urllib.request.build_opener()

    # ---------- 便捷方法 ----------

    def get(self, path, query=None):
        return self.request("GET", path, query=query)

    def post(self, path, body=None):
        return self.request("POST", path, body=body)

    def put(self, path, body=None):
        return self.request("PUT", path, body=body)

    def delete(self, path):
        return self.request("DELETE", path)

    # ---------- 核心请求 ----------

    def request(self, method, path, query=None, body=None, nonce=None, ts=None):
        """发起签名请求。

        body 传 dict/list 时自动 JSON 序列化（ensure_ascii=False，中文原样 UTF-8），
        也可直接传 bytes（签名与发送使用同一原始字节）。
        nonce/ts 仅测试时覆盖。
        """
        url_str = self.base_url + path
        if query:
            url_str += "?" + _rfc3986_query_encode(query)
        if isinstance(body, (dict, list)):
            raw = json.dumps(body, ensure_ascii=False).encode("utf-8")
        else:
            raw = body
        headers = self.sign(
            self.secret_id, self.secret_key, method, url_str, raw, nonce=nonce, ts=ts
        )
        req = urllib.request.Request(url_str, data=raw, method=method.upper(), headers=headers)
        try:
            resp = self._opener.open(req, timeout=self.timeout)
            return Response(resp.status, resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            raw = e.read().decode("utf-8", "replace")
            try:
                payload = json.loads(raw)  # 业务错误响应为 JSON（含 code/msg）
            except Exception:
                payload = raw
            raise HuopanError(e.code, payload) from None

    # ---------- 签名（纯函数，可独立测试） ----------

    @staticmethod
    def sign(secret_id, secret_key, method, url_str, body=b"", nonce=None, ts=None):
        """计算 HPS1 签名，返回完整请求头 dict（含 Authorization）。

        service 按路径自动推断：/seller-api 前缀为 seller，其余为 admin。
        """
        parsed = urllib.parse.urlparse(url_str)
        qseq = urllib.parse.parse_qsl(parsed.query, keep_blank_values=True)
        service = "seller" if parsed.path.startswith("/seller-api") else "admin"
        ts = ts or str(int(time.time()))
        nonce = nonce or uuid.uuid4().hex
        headers = {
            "host": _sign_host(parsed),
            "x-hp-algorithm": ALGORITHM,
            "x-hp-nonce": nonce,
            "x-hp-secretid": secret_id,
            "x-hp-service": service,
            "x-hp-timestamp": ts,
        }
        canonical = "\n".join([
            method.upper(),
            _rfc3986(parsed.path, keep_slash=True),
            _canonical_query(qseq),
            _canonical_headers(headers),
            SIGNED_HEADERS,
            _hash_payload(body),
        ])
        date = time.strftime("%Y-%m-%d", time.gmtime(int(ts)))
        sts = "\n".join([
            ALGORITHM, ts, date,
            hashlib.sha256(canonical.encode("utf-8")).hexdigest(),
        ])
        k_date = _hmac(("HPS1" + secret_key).encode("utf-8"), date.encode("utf-8"))
        k_service = _hmac(k_date, service.encode("utf-8"))
        k_signing = _hmac(k_service, b"hps1_request")
        sig = hmac.new(k_signing, sts.encode("utf-8"), hashlib.sha256).hexdigest()
        authorization = (
            f"{ALGORITHM} Credential={secret_id}/{date}/{service}/hps1_request, "
            f"SignedHeaders={SIGNED_HEADERS}, Signature={sig}"
        )
        out = dict(headers)
        out["Authorization"] = authorization
        return out


def _rfc3986_query_encode(query):
    """将 dict 编码为可直接拼接的 query 串（与签名侧排序语义一致）"""
    qseq = []
    for k, v in query.items():
        values = v if isinstance(v, (list, tuple)) else [v]
        for item in values:
            qseq.append((k, item))
    return _canonical_query(qseq)


# 兼容别名：便于脚本式调用
Client = HuopanClient

__all__ = ["HuopanClient", "Client", "Response", "HuopanError", "ALGORITHM", "SIGNED_HEADERS"]
