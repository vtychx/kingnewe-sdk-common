/**
 * JS SDK 黄金向量与编码规则测试。
 * 运行：node huopan_test.mjs
 * 黄金向量与 docs/api-signature-design.md 6.4 及服务端/其他语言 SDK 一致。
 */
import { strict as assert } from "node:assert";
import {
  ALGORITHM,
  SIGNED_HEADERS,
  HuopanClient,
  canonicalQuery,
  hashPayload,
  rfc3986,
  sign,
} from "./huopan.mjs";

const GOLDEN_SECRET_ID = "HPAKIDTESTSECRETID00000001";
const GOLDEN_SECRET_KEY = "TestSecretKey0123456789abcdefABCDEF";
const GOLDEN_NONCE = "5f8a1c2e9b4d4f6a8e0c7a3b5d9e1f2c";
const GOLDEN_TS = "1726646400"; // UTC 2024-09-18
const GOLDEN_SIGNATURE = "c4a39375035c1d8d0eeb16e5b922dcac5d9eba27e3f6ed8b3047599e3cec6079";
const GOLDEN_URL = "https://www.kingnewe.com/api/products?page=1&page_size=20&keyword=货 盘";

function testGoldenVector() {
  const headers = sign(
    GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "POST", GOLDEN_URL,
    '{"name":"测试"}', { nonce: GOLDEN_NONCE, ts: GOLDEN_TS },
  );
  const want =
    `${ALGORITHM} Credential=${GOLDEN_SECRET_ID}/2024-09-18/admin/hps1_request, ` +
    `SignedHeaders=${SIGNED_HEADERS}, Signature=${GOLDEN_SIGNATURE}`;
  assert.equal(headers.Authorization, want);
  assert.equal(headers["x-hp-service"], "admin");
}

function testCanonicalQueryEncoding() {
  const qseq = [["keyword", "货 盘"], ["page", "1"], ["page_size", "20"]];
  assert.equal(canonicalQuery(qseq), "keyword=%E8%B4%A7%20%E7%9B%98&page=1&page_size=20");
  assert.equal(rfc3986("a/b"), "a%2Fb"); // query 中 / 必须编码
  assert.equal(rfc3986("a/b", true), "a/b"); // 路径编码保留 /
}

function testHashPayloadEmpty() {
  const want = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855";
  assert.equal(hashPayload(undefined), want);
  assert.equal(hashPayload(""), want);
}

function testSellerServiceInference() {
  const headers = sign(
    GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
    "https://www.kingnewe.com/seller-api/dashboard?x=1",
    "", { nonce: GOLDEN_NONCE, ts: GOLDEN_TS },
  );
  assert.equal(headers["x-hp-service"], "seller");
}

function testHostRules() {
  const h1 = sign(GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
    "http://127.0.0.1:8080/api/me", "", { nonce: GOLDEN_NONCE, ts: GOLDEN_TS });
  assert.equal(h1.host, "127.0.0.1:8080");
  const h2 = sign(GOLDEN_SECRET_ID, GOLDEN_SECRET_KEY, "GET",
    "https://www.kingnewe.com:443/api/me", "", { nonce: GOLDEN_NONCE, ts: GOLDEN_TS });
  assert.equal(h2.host, "www.kingnewe.com");
}

function testClientValidation() {
  assert.throws(() => new HuopanClient({ secretId: "", secretKey: "k", baseUrl: "https://x.com" }));
  const c = new HuopanClient({ secretId: "id", secretKey: "key", baseUrl: "https://x.com/" });
  assert.equal(c.baseUrl, "https://x.com");
}

const tests = {
  testGoldenVector,
  testCanonicalQueryEncoding,
  testHashPayloadEmpty,
  testSellerServiceInference,
  testHostRules,
  testClientValidation,
};

let failed = 0;
for (const [name, fn] of Object.entries(tests)) {
  try {
    fn();
    console.log(`[PASS] ${name}`);
  } catch (e) {
    failed++;
    console.log(`[FAIL] ${name}: ${e.message}`);
  }
}
console.log(`===== 结果: ${Object.keys(tests).length - failed}/${Object.keys(tests).length} 通过 =====`);
process.exit(failed ? 1 : 0);
