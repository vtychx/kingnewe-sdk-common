import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.net.URI;
import java.net.URLDecoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.SecureRandom;
import java.time.Instant;
import java.time.ZoneOffset;
import java.time.format.DateTimeFormatter;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

/**
 * 货盘平台 API 签名客户端（HPS1-HMAC-SHA256）。
 * 协议规范：docs/api-signature-design.md 第 6 章；仅依赖 JDK 11+ 标准库。
 *
 * 用法：
 *   HuopanClient client = new HuopanClient("HPAKID...", "SecretKey", "https://www.kingnewe.com");
 *   String resp = client.get("/api/products", Map.of("page", "1"));
 *   String resp2 = client.post("/api/products", "{\"name\":\"测试商品\"}");
 *
 * 自检（黄金向量）：java HuopanClient selftest
 */
public class HuopanClient {

    public static final String ALGORITHM = "HPS1-HMAC-SHA256";
    public static final String SIGNED_HEADERS =
            "host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp";

    private final String secretId;
    private final String secretKey;
    private final String baseUrl;
    private final HttpClient http;
    private final SecureRandom random = new SecureRandom();

    public HuopanClient(String secretId, String secretKey, String baseUrl) {
        if (secretId == null || secretId.isEmpty() || secretKey == null || secretKey.isEmpty()
                || baseUrl == null || baseUrl.isEmpty()) {
            throw new IllegalArgumentException("secret_id / secret_key / base_url 均不能为空");
        }
        this.secretId = secretId;
        this.secretKey = secretKey;
        this.baseUrl = baseUrl.replaceAll("/+$", "");
        this.http = HttpClient.newHttpClient();
    }

    /** 签名 GET；query 为参数键值对（可为 null） */
    public String get(String path, Map<String, String> query) throws Exception {
        return doRequest("GET", path, query, null);
    }

    /** 签名 POST；jsonBody 为调用方序列化好的 JSON 字符串（可为 null） */
    public String post(String path, String jsonBody) throws Exception {
        return doRequest("POST", path, null, jsonBody);
    }

    /** 签名 PUT */
    public String put(String path, String jsonBody) throws Exception {
        return doRequest("PUT", path, null, jsonBody);
    }

    /** 签名 DELETE */
    public String delete(String path) throws Exception {
        return doRequest("DELETE", path, null, null);
    }

    private String doRequest(String method, String path, Map<String, String> query,
                             String jsonBody) throws Exception {
        StringBuilder urlBuilder = new StringBuilder(baseUrl).append(path);
        if (query != null && !query.isEmpty()) {
            List<String> pairs = new ArrayList<>();
            for (Map.Entry<String, String> e : query.entrySet()) {
                pairs.add(sigParamEncode(e.getKey()) + "=" + sigParamEncode(e.getValue()));
            }
            pairs.sort(String::compareTo);
            urlBuilder.append('?').append(String.join("&", pairs));
        }
        byte[] body = jsonBody == null ? new byte[0] : jsonBody.getBytes(StandardCharsets.UTF_8);
        Map<String, String> headers = sign(secretId, secretKey, method, urlBuilder.toString(),
                body, newNonce(), String.valueOf(System.currentTimeMillis() / 1000L));
        HttpRequest.Builder builder = HttpRequest.newBuilder()
                .uri(URI.create(urlBuilder.toString()))
                .method(method, body.length == 0 ? HttpRequest.BodyPublishers.noBody()
                        : HttpRequest.BodyPublishers.ofByteArray(body));
        for (Map.Entry<String, String> e : headers.entrySet()) {
            builder.header(e.getKey(), e.getValue());
        }
        if (jsonBody != null && !jsonBody.isEmpty()) {
            builder.header("Content-Type", "application/json");
        }
        HttpResponse<byte[]> resp = http.send(builder.build(),
                HttpResponse.BodyHandlers.ofByteArray());
        return new String(resp.body(), StandardCharsets.UTF_8);
    }

    /** 生成随机 Nonce（32 位 hex） */
    public String newNonce() {
        byte[] b = new byte[16];
        random.nextBytes(b);
        return hex(b);
    }

    /**
     * 计算 HPS1 签名，返回需附加的请求头（含 Authorization）。
     * service 按路径自动推断：/seller-api 前缀为 seller，其余为 admin。
     * nonce/ts 由调用方生成（测试时固定以对齐黄金向量）。
     */
    public static Map<String, String> sign(String secretId, String secretKey, String method,
                                           String rawUrl, byte[] body, String nonce, String ts)
            throws Exception {
        // 手动宽松解析（java.net.URI 会拒绝含未编码字符的 URL，与 Go/Python 行为不一致）
        int schemeEnd = rawUrl.indexOf("://") + 3;
        int slash = rawUrl.indexOf('/', schemeEnd);
        int qmark = rawUrl.indexOf('?', slash == -1 ? schemeEnd : slash);
        String hostPart = rawUrl.substring(schemeEnd,
                slash == -1 ? (qmark == -1 ? rawUrl.length() : qmark) : slash);
        String path = slash == -1 ? "/"
                : rawUrl.substring(slash, qmark == -1 ? rawUrl.length() : qmark);
        String rawQuery = qmark == -1 ? null : rawUrl.substring(qmark + 1);
        String service = path.startsWith("/seller-api") ? "seller" : "admin";
        Map<String, String> headers = new TreeMap<>();
        headers.put("host", signHost(hostPart));
        headers.put("x-hp-algorithm", ALGORITHM);
        headers.put("x-hp-nonce", nonce);
        headers.put("x-hp-secretid", secretId);
        headers.put("x-hp-service", service);
        headers.put("x-hp-timestamp", ts);

        StringBuilder canonical = new StringBuilder()
                .append(method.toUpperCase()).append('\n')
                .append(sigURIEncode(path))
                .append('\n')
                .append(canonicalQuery(rawQuery)).append('\n')
                .append(canonicalHeaders(headers)).append('\n')
                .append(SIGNED_HEADERS).append('\n')
                .append(hashPayload(body));
        String date = DateTimeFormatter.ofPattern("yyyy-MM-dd")
                .withZone(ZoneOffset.UTC).format(Instant.ofEpochSecond(Long.parseLong(ts)));
        String sts = String.join("\n", ALGORITHM, ts, date,
                hashPayload(canonical.toString().getBytes(StandardCharsets.UTF_8)));
        byte[] kDate = hmac(("HPS1" + secretKey).getBytes(StandardCharsets.UTF_8),
                date.getBytes(StandardCharsets.UTF_8));
        byte[] kService = hmac(kDate, service.getBytes(StandardCharsets.UTF_8));
        byte[] kSigning = hmac(kService, "hps1_request".getBytes(StandardCharsets.UTF_8));
        String sig = hex(hmac(kSigning, sts.getBytes(StandardCharsets.UTF_8)));
        Map<String, String> out = new TreeMap<>(headers);
        // JDK HttpClient 禁止手动设置受限头 Host：由其按 URL 自动发送，
        // 值与签名使用的 signHost 规则一致（默认端口省略），故从附加头中移除
        out.remove("host");
        out.put("Authorization", ALGORITHM + " Credential=" + secretId + "/" + date + "/"
                + service + "/hps1_request, SignedHeaders=" + SIGNED_HEADERS
                + ", Signature=" + sig);
        return out;
    }

    // ==================== 规范化构造（与服务端同源语义） ====================

    /** 签名用 Host：默认端口 80/443 省略，小写 */
    static String signHost(String hostPart) {
        int colon = hostPart.indexOf(':');
        String host = (colon >= 0 ? hostPart.substring(0, colon) : hostPart).toLowerCase();
        int port = colon >= 0 ? Integer.parseInt(hostPart.substring(colon + 1)) : -1;
        if (port != -1 && port != 80 && port != 443) {
            return host + ":" + port;
        }
        return host;
    }

    /** 规范 query：解析 rawQuery（+ 解码为空格）→ 逐对编码 → (k, v) 字节序排序 */
    static String canonicalQuery(String rawQuery) {
        if (rawQuery == null || rawQuery.isEmpty()) {
            return "";
        }
        List<String[]> pairs = new ArrayList<>();
        for (String part : rawQuery.split("&")) {
            if (part.isEmpty()) {
                continue;
            }
            int eq = part.indexOf('=');
            String k = eq < 0 ? part : part.substring(0, eq);
            String v = eq < 0 ? "" : part.substring(eq + 1);
            pairs.add(new String[]{sigParamEncode(urlDecode(k)), sigParamEncode(urlDecode(v))});
        }
        pairs.sort((a, b) -> {
            int byKey = a[0].compareTo(b[0]);
            return byKey != 0 ? byKey : a[1].compareTo(b[1]);
        });
        StringBuilder sb = new StringBuilder();
        for (String[] p : pairs) {
            if (sb.length() > 0) {
                sb.append('&');
            }
            sb.append(p[0]).append('=').append(p[1]);
        }
        return sb.toString();
    }

    /** 规范头：按名称小写字典序（TreeMap），value 压缩连续空白 */
    static String canonicalHeaders(Map<String, String> headers) {
        StringBuilder sb = new StringBuilder();
        for (Map.Entry<String, String> e : headers.entrySet()) {
            sb.append(e.getKey()).append(':')
                    .append(String.join(" ", e.getValue().trim().split("\\s+")))
                    .append('\n');
        }
        return sb.toString();
    }

    /** RFC3986 编码：unreserved 集合不编码；keepSlash 时保留路径分隔符 */
    static String rfc3986Encode(String s, boolean keepSlash) {
        StringBuilder sb = new StringBuilder();
        for (byte b : s.getBytes(StandardCharsets.UTF_8)) {
            int c = b & 0xFF;
            if ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')
                    || c == '-' || c == '_' || c == '.' || c == '~' || (c == '/' && keepSlash)) {
                sb.append((char) c);
            } else {
                sb.append('%').append(String.format("%02X", c));
            }
        }
        return sb.toString();
    }

    static String sigURIEncode(String s) {
        return rfc3986Encode(s, true);
    }

    static String sigParamEncode(String s) {
        return rfc3986Encode(s, false);
    }

    static String urlDecode(String s) {
        return URLDecoder.decode(s, StandardCharsets.UTF_8);
    }

    static String hashPayload(byte[] body) {
        return hex(sha256(body == null ? new byte[0] : body));
    }

    static byte[] hmac(byte[] key, byte[] msg) throws Exception {
        Mac mac = Mac.getInstance("HmacSHA256");
        mac.init(new SecretKeySpec(key, "HmacSHA256"));
        return mac.doFinal(msg);
    }

    static byte[] sha256(byte[] data) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(data);
        } catch (Exception e) {
            throw new IllegalStateException("SHA-256 算法不可用", e);
        }
    }

    static String hex(byte[] data) {
        StringBuilder sb = new StringBuilder();
        for (byte b : data) {
            sb.append(String.format("%02x", b));
        }
        return sb.toString();
    }

    // ==================== 自检（黄金向量） ====================

    /** java HuopanClient selftest：验证与服务端的跨语言一致性 */
    public static void main(String[] args) throws Exception {
        if (args.length > 0 && "selftest".equals(args[0])) {
            Map<String, String> headers = sign(
                    "HPAKIDTESTSECRETID00000001",
                    "TestSecretKey0123456789abcdefABCDEF",
                    "POST",
                    "https://www.kingnewe.com/api/products?page=1&page_size=20&keyword=货 盘",
                    "{\"name\":\"测试\"}".getBytes(StandardCharsets.UTF_8),
                    "5f8a1c2e9b4d4f6a8e0c7a3b5d9e1f2c",
                    "1726646400");
            String want = ALGORITHM + " Credential=HPAKIDTESTSECRETID00000001/2024-09-18/admin/"
                    + "hps1_request, SignedHeaders=" + SIGNED_HEADERS
                    + ", Signature=c4a39375035c1d8d0eeb16e5b922dcac5d9eba27e3f6ed8b3047599e3cec6079";
            if (want.equals(headers.get("Authorization"))) {
                System.out.println("[PASS] 黄金向量签名与服务端一致");
            } else {
                System.out.println("[FAIL] Authorization=\n" + headers.get("Authorization")
                        + "\nwant\n" + want);
                System.exit(1);
            }
            return;
        }
        System.out.println("用法：java HuopanClient selftest");
    }
}
