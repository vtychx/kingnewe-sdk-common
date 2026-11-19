import java.util.HashMap;
import java.util.Map;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * huopan-sdk-java 使用示例：初始化客户端 → 签名调用 → 解析响应 → 错误处理。
 *
 * ============================ 凭证与访问端匹配规则 ============================
 *
 * 平台有三类 API 凭证（在对应后台的「API凭证」页面创建），与访问路径一一对应，
 * 不能交叉访问（混用返回 E1007，提示信息会指明该凭证应使用的路径前缀）：
 *
 *   超管端凭证（在超管后台创建） → 调用 /api/**        路径，service=admin
 *   卖家端凭证（在卖家后台创建） → 调用 /seller-api/** 路径，service=seller
 *   买家端凭证（在买家后台创建） → 调用 /buyer-api/**  路径，service=buyer
 *
 * SDK 会根据请求路径前缀自动推断 service，无需手动指定。
 *
 * ============================ 运行 ============================
 *
 * 运行前准备：
 * 1. 登录超管端/卖家端/买家端后台 →「API凭证」→ 创建凭证，保存 SecretId 与 SecretKey
 *    （明文仅创建时展示一次）
 * 2. 修改下方 SECRET_ID / SECRET_KEY / BASE_URL（或改用环境变量 System.getenv）
 * 3. 编译运行（HUOPAN_SCOPE=admin 演示超管端 / seller 演示卖家端 / buyer 演示买家端）：
 *    javac HuopanClient.java Example.java
 *    HUOPAN_SCOPE=admin java Example
 *
 * ============================ 响应结构 ============================
 *
 * 平台统一响应外层：{"code":0,"msg":"ok","data":{...}}
 *   code==0 业务成功；签名/凭证类错误 code 为字符串 "E1001"-"E1008"
 *   （E1001 凭证不存在、E1002 签名失败、E1003 凭证已禁用、E1004 Nonce 重复、
 *   E1005 时间戳过期、E1006 签名头格式错误、E1007 端不匹配、E1008 账号状态异常）。
 * Java 标准库无 JSON 解析器，示例演示原始文本处理；生产建议引入 Jackson/Gson。
 *
 * ============================ 游客视图（重要） ============================
 * 未携带签名头（直接用浏览器/curl 打开接口 URL）时，/buyer-api/** 查询接口按游客
 * 身份返回：code 仍为 0，但 skus[].price 与 skus[].stock 均为 0。这是正常行为，
 * 不代表接口异常或无数据；必须经本 SDK 签名调用才能看到真实价格与库存。
 */
public class Example {

    static String SECRET_ID = "HPAKID你的SecretId";
    static String SECRET_KEY = "你的SecretKey";
    static String BASE_URL = "https://www.kingnewe.com"; // 测试环境可用 http://101.42.20.80:8080

    public static void main(String[] args) throws Exception {
        // 环境变量优先
        String env = System.getenv("HUOPAN_SECRET_ID");
        if (env != null && !env.isEmpty()) SECRET_ID = env;
        env = System.getenv("HUOPAN_SECRET_KEY");
        if (env != null && !env.isEmpty()) SECRET_KEY = env;
        env = System.getenv("HUOPAN_BASE_URL");
        if (env != null && !env.isEmpty()) BASE_URL = env;
        String scope = System.getenv("HUOPAN_SCOPE") == null
                ? "admin" : System.getenv("HUOPAN_SCOPE").toLowerCase();

        // ---------- 1. 初始化客户端 ----------
        HuopanClient client = new HuopanClient(SECRET_ID, SECRET_KEY, BASE_URL);

        // ---------- 2. 按凭证类型演示对应端的调用 ----------
        if ("seller".equals(scope)) {
            runSellerExamples(client);
        } else if ("buyer".equals(scope)) {
            runBuyerExamples(client);
        } else {
            runAdminExamples(client);
        }
        System.out.println("示例执行完成");
    }

    /** 超管端凭证调用 /api/** 接口 */
    static void runAdminExamples(HuopanClient client) throws Exception {
        // ---- 2.1 GET /api/me：获取当前凭证对应的超管账号身份 ----
        try {
            String resp = client.get("/api/me", null);
            System.out.println("当前身份响应: " + excerpt(resp));
            // 响应结构：{"code":0,"data":{"admin":{"id":1,"username":"admin","role":"super"},"menus":[...]}}
            if (resp.contains("\"username\"")) {
                System.out.println("身份解析示例: 含 username 字段，签名与认证均通过");
            }
        } catch (Exception e) {
            // 平台失败响应（JSON）：{"code":"E1002","msg":"签名校验失败"}
            System.out.println("获取身份失败: " + e.getMessage());
            return;
        }

        // ---- 2.2 GET 带查询参数（含中文与空格，SDK 自动 RFC3986 编码） ----
        Map<String, String> query = new HashMap<>();
        query.put("page", "1");
        query.put("page_size", "5");
        query.put("keyword", "货 盘");
        String resp = client.get("/api/products", query);
        System.out.println("商品列表响应: " + excerpt(resp));

        // ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
        // String resp2 = client.post("/api/products",
        //         "{\"name\":\"示例商品\",\"price\":9.9}");
        // System.out.println("创建商品: " + resp2);
    }

    /** 卖家端凭证调用 /seller-api/** 接口 */
    static void runSellerExamples(HuopanClient client) throws Exception {
        // ---- 2.1 GET /seller-api/dashboard：卖家端概览（含店铺与账号信息） ----
        try {
            String resp = client.get("/seller-api/dashboard", null);
            System.out.println("卖家概览响应: " + excerpt(resp));
        } catch (Exception e) {
            System.out.println("获取卖家概览失败: " + e.getMessage());
            return;
        }

        // ---- 2.2 GET /seller-api/products：当前卖家的商品列表（数据按凭证卖家隔离） ----
        Map<String, String> query = new HashMap<>();
        query.put("page", "1");
        query.put("page_size", "5");
        String resp = client.get("/seller-api/products", query);
        System.out.println("商品列表响应: " + excerpt(resp));

        // ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
        // String resp2 = client.post("/seller-api/products", "{\"name\":\"卖家示例商品\"}");
        // System.out.println("创建商品: " + resp2);
    }

    /** 买家端凭证调用 /buyer-api/** 接口 */
    static void runBuyerExamples(HuopanClient client) throws Exception {
        // ---- 2.1 GET /buyer-api/profile：当前买家个人信息 ----
        try {
            String resp = client.get("/buyer-api/profile", null);
            System.out.println("买家信息响应: " + excerpt(resp));
        } catch (Exception e) {
            System.out.println("获取买家信息失败: " + e.getMessage());
            return;
        }

        // ---- 2.2 GET /buyer-api/orders：当前买家的订单列表（数据按凭证买家隔离） ----
        Map<String, String> query = new HashMap<>();
        query.put("page", "1");
        query.put("page_size", "5");
        String resp = client.get("/buyer-api/orders", query);
        System.out.println("订单列表响应: " + excerpt(resp));

        // ---- 2.3 GET /buyer-api/products：商品列表（签名后价格库存为真实值） ----
        Map<String, String> listQuery = new HashMap<>();
        listQuery.put("page", "1");
        listQuery.put("page_size", "5");
        String listResp = client.get("/buyer-api/products", listQuery);
        System.out.println("商品列表响应: " + excerpt(listResp));
        System.out.println("  price=" + extract(listResp, "price") + " stock=" + extract(listResp, "stock"));

        // ---- 2.4 GET /buyer-api/products/{id}：商品详情（示例 id=63） ----
        String detailResp = client.get("/buyer-api/products/63", null);
        System.out.println("商品详情响应: " + excerpt(detailResp));
        System.out.println("  price=" + extract(detailResp, "price") + " stock=" + extract(detailResp, "stock"));

        // ---- 2.5 POST 写操作示例（默认注释，取消注释后真实生效） ----
        // String resp2 = client.post("/buyer-api/addresses",
        //         "{\"receiver\":\"张三\",\"phone\":\"13800138000\"}");
        // System.out.println("创建地址: " + resp2);
    }

    /** 截断长响应便于控制台查看 */
    static String excerpt(String s) {
        return s.length() <= 200 ? s : s.substring(0, 200) + "...";
    }

    /** 从响应 JSON 文本中提取某数值字段的全部取值（无 JSON 库时便于查看价格/库存） */
    static String extract(String json, String field) {
        Matcher m = Pattern.compile("\"" + field + "\"\\s*:\\s*(-?[0-9.]+)").matcher(json);
        StringBuilder sb = new StringBuilder();
        while (m.find()) {
            if (sb.length() > 0) {
                sb.append(", ");
            }
            sb.append(m.group(1));
        }
        return "[" + sb + "]";
    }
}
