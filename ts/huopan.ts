/**
 * 货盘平台 API 签名客户端（HPS1-HMAC-SHA256）— TypeScript 版本。
 * 协议规范：docs/api-signature-design.md 第 6 章。
 * 运行环境：Node 18+（原生 fetch 与 node:crypto），零第三方依赖。
 * 可直接运行（node --experimental-strip-types）或经 tsc 编译后使用。
 *
 * 用法：
 *   import { HuopanClient } from "./huopan.ts";
 *   const client = new HuopanClient({
 *     secretId: "HPAKID...", secretKey: "你的SecretKey",
 *     baseUrl: "https://www.kingnewe.com",
 *   });
 *   const resp = await client.post("/api/products", { name: "测试商品" });
 *   console.log(resp.statusCode, resp.json());
 */
import { createHash, createHmac, randomBytes } from "node:crypto";

export const ALGORITHM = "HPS1-HMAC-SHA256";
export const SIGNED_HEADERS =
  "host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp";

export type QueryValue = string | number | boolean;
export type Query = Record<string, QueryValue | QueryValue[]>;
export type JsonLike = Record<string, unknown> | unknown[] | string | number | boolean | null;

export interface ClientOptions {
  secretId: string;
  secretKey: string;
  baseUrl: string;
  /** 请求超时毫秒数，默认 30000 */
  timeout?: number;
}

export interface ApiResponse {
  statusCode: number;
  body: string;
  /** 解析响应 JSON（非 JSON 响应会抛出异常） */
  json: <T = unknown>() => T;
}

export type SignHeaders = Record<string, string>;

export interface SignOptions {
  nonce?: string;
  ts?: string;
}

/** RFC3986 编码：unreserved 集合不编码；keepSlash 时保留路径分隔符 */
export function rfc3986(value: string | number | boolean, keepSlash = false): string {
  const bytes = Buffer.from(String(value), "utf8");
  let out = "";
  for (const b of bytes) {
    const c = String.fromCharCode(b);
    if (/[A-Za-z0-9\-_.~]/.test(c) || (keepSlash && c === "/")) {
      out += c;
    } else {
      out += "%" + b.toString(16).toUpperCase().padStart(2, "0");
    }
  }
  return out;
}

/** 规范 query：先编码每对 k/v，再按 (k, v) 字节序排序 */
export function canonicalQuery(qseq: Array<[string, string]>): string {
  const pairs = qseq
    .map(([k, v]): [string, string] => [rfc3986(k), rfc3986(v)])
    .sort((a, b) => (a[0] < b[0] ? -1 : a[0] > b[0] ? 1 : a[1] < b[1] ? -1 : a[1] > b[1] ? 1 : 0));
  return pairs.map(([k, v]) => `${k}=${v}`).join("&");
}

/** 规范头：按名称小写字典序，value 压缩连续空白 */
export function canonicalHeaders(headers: SignHeaders): string {
  return Object.keys(headers)
    .sort()
    .map((name) => `${name}:${String(headers[name]).trim().replace(/\s+/g, " ")}\n`)
    .join("");
}

/** body 原始字节 SHA256 小写 hex（null/undefined 等价空串） */
export function hashPayload(body?: string | Buffer): string {
  return createHash("sha256").update(body ?? "").digest("hex");
}

const signHost = (u: URL): string => {
  const host = u.hostname.toLowerCase();
  const port = u.port;
  return port && port !== "80" && port !== "443" ? `${host}:${port}` : host;
};

/**
 * 计算 HPS1 签名，返回完整请求头对象（含 Authorization）。
 * service 按路径自动推断：/seller-api 前缀为 seller，其余为 admin。
 * opts.nonce / opts.ts 仅测试时覆盖。
 */
export function sign(
  secretId: string,
  secretKey: string,
  method: string,
  urlStr: string,
  body: string | Buffer = "",
  opts: SignOptions = {},
): SignHeaders {
  const u = new URL(urlStr);
  const qseq: Array<[string, string]> = [...u.searchParams.entries()];
  const service = u.pathname.startsWith("/seller-api") ? "seller" : "admin";
  const ts = opts.ts ?? String(Math.floor(Date.now() / 1000));
  const nonce = opts.nonce ?? randomBytes(16).toString("hex");
  const headers: SignHeaders = {
    host: signHost(u),
    "x-hp-algorithm": ALGORITHM,
    "x-hp-nonce": nonce,
    "x-hp-secretid": secretId,
    "x-hp-service": service,
    "x-hp-timestamp": ts,
  };
  const canonical = [
    method.toUpperCase(),
    rfc3986(u.pathname, true),
    canonicalQuery(qseq),
    canonicalHeaders(headers),
    SIGNED_HEADERS,
    hashPayload(body),
  ].join("\n");
  const date = new Date(Number(ts) * 1000).toISOString().slice(0, 10);
  const sts = [
    ALGORITHM,
    ts,
    date,
    createHash("sha256").update(canonical).digest("hex"),
  ].join("\n");
  const kDate = createHmac("sha256", "HPS1" + secretKey).update(date).digest();
  const kService = createHmac("sha256", kDate).update(service).digest();
  const kSigning = createHmac("sha256", kService).update("hps1_request").digest();
  const sig = createHmac("sha256", kSigning).update(sts).digest("hex");
  const authorization =
    `${ALGORITHM} Credential=${secretId}/${date}/${service}/hps1_request, ` +
    `SignedHeaders=${SIGNED_HEADERS}, Signature=${sig}`;
  return { ...headers, Authorization: authorization };
}

/** API 签名客户端（可复用） */
export class HuopanClient {
  private readonly secretId: string;
  private readonly secretKey: string;
  private readonly baseUrl: string;
  private readonly timeout: number;

  constructor(opts: ClientOptions) {
    const { secretId, secretKey, baseUrl } = opts;
    if (!secretId || !secretKey || !baseUrl) {
      throw new Error("secretId / secretKey / baseUrl 均不能为空");
    }
    this.secretId = secretId;
    this.secretKey = secretKey;
    this.baseUrl = baseUrl.replace(/\/+$/, "");
    this.timeout = opts.timeout ?? 30000;
  }

  get(path: string, query?: Query): Promise<ApiResponse> {
    return this.request("GET", path, { query });
  }

  post(path: string, body?: JsonLike): Promise<ApiResponse> {
    return this.request("POST", path, { body });
  }

  put(path: string, body?: JsonLike): Promise<ApiResponse> {
    return this.request("PUT", path, { body });
  }

  delete(path: string): Promise<ApiResponse> {
    return this.request("DELETE", path);
  }

  /** 核心请求：body 传对象时自动 JSON 序列化（中文原样 UTF-8）；传 string 时视为原始文本 */
  async request(
    method: string,
    path: string,
    opts: { query?: Query; body?: JsonLike | Buffer; nonce?: string; ts?: string } = {},
  ): Promise<ApiResponse> {
    let urlStr: string = this.baseUrl + path;
    const { query, body, nonce, ts } = opts;
    if (query) {
      const qseq: Array<[string, string]> = [];
      for (const [k, v] of Object.entries(query)) {
        for (const item of Array.isArray(v) ? v : [v]) {
          qseq.push([k, String(item)]);
        }
      }
      urlStr += "?" + canonicalQuery(qseq);
    }
    const raw: string | Buffer =
      body === undefined || body === null
        ? ""
        : typeof body === "string" || Buffer.isBuffer(body)
          ? body
          : JSON.stringify(body);
    const headers = sign(this.secretId, this.secretKey, method, urlStr, raw, { nonce, ts });
    if (raw !== "" && typeof body === "object") {
      headers["Content-Type"] = "application/json";
    }
    const resp = await fetch(urlStr, {
      method: method.toUpperCase(),
      headers,
      body: raw === "" ? undefined : raw,
      signal: AbortSignal.timeout(this.timeout),
    });
    const text = await resp.text();
    const toJson = <T>(): T => JSON.parse(text) as T;
    return { statusCode: resp.status, body: text, json: toJson };
  }
}
