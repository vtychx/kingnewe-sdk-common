using System;
using System.Collections.Generic;
using System.Globalization;
using System.Linq;
using System.Net;
using System.Net.Http;
using System.Security.Cryptography;
using System.Text;
using System.Text.Json;
using System.Text.RegularExpressions;

namespace HuopanSdk
{
    /// <summary>HPS1 协议常量。</summary>
    public static class HuopanConstants
    {
        public const string Algorithm = "HPS1-HMAC-SHA256";
        public const string SignedHeaders =
            "host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp";
    }

    /// <summary>HTTP / 网络层错误。</summary>
    public sealed class HuopanException : Exception
    {
        public HttpStatusCode StatusCode { get; }
        public string Body { get; }

        public HuopanException(HttpStatusCode statusCode, string body)
            : base($"HTTP {(int)statusCode}: {body}")
        {
            StatusCode = statusCode;
            Body = body;
        }
    }

    /// <summary>响应体。</summary>
    public sealed class HuopanResponse
    {
        public int StatusCode { get; set; }
        public string Body { get; set; }

        public JsonDocument Json() =>
            JsonDocument.Parse(string.IsNullOrEmpty(Body) ? "{}" : Body);
    }

    /// <summary>
    /// 货盘平台 API 签名客户端（HPS1-HMAC-SHA256），仅依赖 .NET 标准库。
    /// 协议规范：docs/api-signature-design.md 第 6 章。
    /// </summary>
    public sealed class HuopanClient
    {
        private readonly string _secretId;
        private readonly string _secretKey;
        private readonly string _baseUrl;
        private readonly HttpClient _http;

        public HuopanClient(string secretId, string secretKey, string baseUrl)
            : this(secretId, secretKey, baseUrl, new HttpClient())
        {
        }

        public HuopanClient(string secretId, string secretKey, string baseUrl, HttpClient http)
        {
            if (string.IsNullOrEmpty(secretId) || string.IsNullOrEmpty(secretKey) ||
                string.IsNullOrEmpty(baseUrl))
                throw new ArgumentException("secretId / secretKey / baseUrl 均不能为空");

            _secretId = secretId;
            _secretKey = secretKey;
            _baseUrl = baseUrl.TrimEnd('/');
            _http = http ?? new HttpClient();
        }

        // ---------- 便捷方法 ----------

        public HuopanResponse Get(string path, IDictionary<string, string> query = null) =>
            DoRequest("GET", path, query, null);

        public HuopanResponse Post(string path, string jsonBody) =>
            DoRequest("POST", path, null, jsonBody);

        public HuopanResponse Put(string path, string jsonBody) =>
            DoRequest("PUT", path, null, jsonBody);

        public HuopanResponse Delete(string path) =>
            DoRequest("DELETE", path, null, null);

        // ---------- 核心请求 ----------

        /// <summary>发起签名请求。query 的 k/v 按 RFC3986 编码后排序；body 使用同一原始字节签名并发送。</summary>
        public HuopanResponse DoRequest(string method, string path,
            IDictionary<string, string> query, string jsonBody)
        {
            string url = _baseUrl + path;
            if (query != null && query.Count > 0)
            {
                var pairs = query
                    .Select(kv => SigParamEncode(kv.Key) + "=" + SigParamEncode(kv.Value))
                    .OrderBy(s => s, StringComparer.Ordinal);
                url += "?" + string.Join("&", pairs);
            }

            byte[] body = string.IsNullOrEmpty(jsonBody)
                ? Array.Empty<byte>()
                : Encoding.UTF8.GetBytes(jsonBody);

            string nonce = NewNonce();
            string ts = DateTimeOffset.UtcNow.ToUnixTimeSeconds()
                .ToString(CultureInfo.InvariantCulture);
            Dictionary<string, string> headers =
                Sign(_secretId, _secretKey, method, url, body, nonce, ts);

            using var request = new HttpRequestMessage(new HttpMethod(method.ToUpperInvariant()), url);
            foreach (var kv in headers)
            {
                if (kv.Key.Equals("host", StringComparison.OrdinalIgnoreCase))
                    request.Headers.Host = kv.Value;
                else
                    request.Headers.TryAddWithoutValidation(kv.Key, kv.Value);
            }
            if (body.Length > 0)
            {
                var content = new ByteArrayContent(body);
                content.Headers.ContentType =
                    new System.Net.Http.Headers.MediaTypeHeaderValue("application/json");
                request.Content = content;
            }

            HttpResponseMessage response = _http
                .SendAsync(request, HttpCompletionOption.ResponseContentRead)
                .GetAwaiter().GetResult();
            using (response)
            {
                string text = response.Content.ReadAsStringAsync().GetAwaiter().GetResult();
                if (!response.IsSuccessStatusCode)
                    throw new HuopanException(response.StatusCode, text);
                return new HuopanResponse { StatusCode = (int)response.StatusCode, Body = text };
            }
        }

        /// <summary>
        /// 计算 HPS1 签名，返回需附加的请求头（含 Authorization）。
        /// service 按路径前缀推断：/buyer-api→buyer、/seller-api→seller，其余→admin。
        /// nonce/ts 由调用方生成（测试时固定以对齐黄金向量）。
        /// </summary>
        public static Dictionary<string, string> Sign(string secretId, string secretKey,
            string method, string rawUrl, byte[] body, string nonce, string ts)
        {
            // 手动宽松解析：golden 向量 URL 含未编码的中文与空格，避免 Uri 归一化影响签名
            int schemeEnd = rawUrl.IndexOf("://", StringComparison.Ordinal);
            int start = schemeEnd < 0 ? 0 : schemeEnd + 3;
            int slash = rawUrl.IndexOf('/', start);
            int qmark = rawUrl.IndexOf('?', slash < 0 ? start : slash);
            int authorityEnd = slash < 0 ? (qmark < 0 ? rawUrl.Length : qmark) : slash;
            string hostPart = rawUrl.Substring(start, authorityEnd - start);
            string path = slash < 0
                ? "/"
                : rawUrl.Substring(slash, (qmark < 0 ? rawUrl.Length : qmark) - slash);
            string rawQuery = qmark < 0 ? null : rawUrl.Substring(qmark + 1);

            string service = path.StartsWith("/buyer-api", StringComparison.Ordinal) ? "buyer"
                : path.StartsWith("/seller-api", StringComparison.Ordinal) ? "seller"
                : "admin";

            var headers = new Dictionary<string, string>(StringComparer.Ordinal)
            {
                ["host"] = SignHost(hostPart),
                ["x-hp-algorithm"] = HuopanConstants.Algorithm,
                ["x-hp-nonce"] = nonce,
                ["x-hp-secretid"] = secretId,
                ["x-hp-service"] = service,
                ["x-hp-timestamp"] = ts,
            };

            string canonical = string.Join("\n", new[]
            {
                method.ToUpperInvariant(),
                SigUriEncode(path),
                CanonicalQuery(rawQuery),
                CanonicalHeaders(headers),
                HuopanConstants.SignedHeaders,
                HashPayload(body),
            });

            long unix = long.Parse(ts, CultureInfo.InvariantCulture);
            string date = DateTimeOffset.FromUnixTimeSeconds(unix).UtcDateTime
                .ToString("yyyy-MM-dd", CultureInfo.InvariantCulture);
            string sts = string.Join("\n", new[]
            {
                HuopanConstants.Algorithm,
                ts,
                date,
                Sha256Hex(Encoding.UTF8.GetBytes(canonical)),
            });

            byte[] kDate = Hmac(Encoding.UTF8.GetBytes("HPS1" + secretKey),
                Encoding.UTF8.GetBytes(date));
            byte[] kService = Hmac(kDate, Encoding.UTF8.GetBytes(service));
            byte[] kSigning = Hmac(kService, Encoding.UTF8.GetBytes("hps1_request"));
            string sig = ToHex(Hmac(kSigning, Encoding.UTF8.GetBytes(sts)));

            var result = new Dictionary<string, string>(headers, StringComparer.Ordinal)
            {
                ["Authorization"] =
                    $"{HuopanConstants.Algorithm} Credential={secretId}/{date}/{service}/hps1_request, " +
                    $"SignedHeaders={HuopanConstants.SignedHeaders}, Signature={sig}",
            };
            return result;
        }

        // ==================== 规范化构造（与服务端同源语义） ====================

        /// <summary>签名用 Host：默认端口 80/443 省略，小写。</summary>
        public static string SignHost(string hostPart)
        {
            int colon = hostPart.IndexOf(':');
            string host = (colon >= 0 ? hostPart.Substring(0, colon) : hostPart).ToLowerInvariant();
            int port = -1;
            if (colon >= 0 && int.TryParse(hostPart.Substring(colon + 1),
                    NumberStyles.Integer, CultureInfo.InvariantCulture, out int p))
                port = p;
            return port != -1 && port != 80 && port != 443 ? host + ":" + port : host;
        }

        /// <summary>规范 query：先解码每对 k/v，再 RFC3986 编码，按 (k, v) 字节序排序。</summary>
        public static string CanonicalQuery(string rawQuery)
        {
            if (string.IsNullOrEmpty(rawQuery)) return "";
            var pairs = new List<string[]>();
            foreach (string part in rawQuery.Split('&'))
            {
                if (part.Length == 0) continue;
                int eq = part.IndexOf('=');
                string k = eq < 0 ? part : part.Substring(0, eq);
                string v = eq < 0 ? "" : part.Substring(eq + 1);
                pairs.Add(new[] { SigParamEncode(UrlDecode(k)), SigParamEncode(UrlDecode(v)) });
            }
            pairs.Sort((a, b) =>
            {
                int byKey = string.CompareOrdinal(a[0], b[0]);
                return byKey != 0 ? byKey : string.CompareOrdinal(a[1], b[1]);
            });
            return string.Join("&", pairs.Select(p => p[0] + "=" + p[1]));
        }

        /// <summary>规范头：按名称小写字典序，value 压缩连续空白。</summary>
        private static string CanonicalHeaders(IDictionary<string, string> headers)
        {
            var sb = new StringBuilder();
            foreach (var kv in headers.OrderBy(k => k.Key, StringComparer.Ordinal))
                sb.Append(kv.Key).Append(':')
                  .Append(Regex.Replace(kv.Value.Trim(), @"\s+", " "))
                  .Append('\n');
            return sb.ToString();
        }

        /// <summary>RFC3986 编码：unreserved 集合不编码；keepSlash 时保留路径分隔符。</summary>
        private static string Rfc3986Encode(string s, bool keepSlash)
        {
            var sb = new StringBuilder();
            foreach (byte b in Encoding.UTF8.GetBytes(s))
            {
                int c = b;
                if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
                    (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' ||
                    (c == '/' && keepSlash))
                    sb.Append((char)c);
                else
                    sb.Append('%').Append(c.ToString("X2", CultureInfo.InvariantCulture));
            }
            return sb.ToString();
        }

        public static string SigUriEncode(string s) => Rfc3986Encode(s, true);
        public static string SigParamEncode(string s) => Rfc3986Encode(s, false);

        /// <summary>URL 解码：+ 转空格，%XX 按 UTF-8 字节解码。</summary>
        private static string UrlDecode(string s)
        {
            var output = new List<byte>();
            var literal = new StringBuilder();

            void Flush()
            {
                if (literal.Length == 0) return;
                output.AddRange(Encoding.UTF8.GetBytes(literal.ToString()));
                literal.Clear();
            }

            for (int i = 0; i < s.Length; i++)
            {
                char ch = s[i];
                if (ch == '+')
                {
                    Flush();
                    output.Add(0x20);
                }
                else if (ch == '%' && i + 2 < s.Length &&
                         TryHex(s[i + 1], out int h) && TryHex(s[i + 2], out int l))
                {
                    Flush();
                    output.Add((byte)((h << 4) | l));
                    i += 2;
                }
                else
                {
                    literal.Append(ch);
                }
            }
            Flush();
            return Encoding.UTF8.GetString(output.ToArray());
        }

        private static bool TryHex(char c, out int v)
        {
            v = c >= '0' && c <= '9' ? c - '0'
                : c >= 'a' && c <= 'f' ? c - 'a' + 10
                : c >= 'A' && c <= 'F' ? c - 'A' + 10
                : -1;
            return v >= 0;
        }

        /// <summary>body 原始字节 SHA256 小写 hex（null 等价空串）。</summary>
        public static string HashPayload(byte[] body) => Sha256Hex(body ?? Array.Empty<byte>());

        private static string Sha256Hex(byte[] data)
        {
            using var sha = SHA256.Create();
            return ToHex(sha.ComputeHash(data));
        }

        private static byte[] Hmac(byte[] key, byte[] msg)
        {
            using var mac = new HMACSHA256(key);
            return mac.ComputeHash(msg);
        }

        private static string ToHex(byte[] data) =>
            string.Concat(data.Select(b => b.ToString("x2", CultureInfo.InvariantCulture)));

        /// <summary>生成 32 位 hex 随机 Nonce。</summary>
        public static string NewNonce() => ToHex(RandomNumberGenerator.GetBytes(16));
    }
}
