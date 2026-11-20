using System;
using System.Collections.Generic;
using System.Text;
using System.Text.Json;
using HuopanSdk;

/// <summary>
/// huopan-sdk-csharp 使用示例：初始化客户端 → 签名调用 → 解析响应 → 错误处理。
///
/// 凭证类型与访问路径一一对应，混用返回 E1007：
///   超管端凭证 → /api/**          service=admin   （HUOPAN_SCOPE=admin，默认）
///   卖家端凭证 → /seller-api/**   service=seller  （HUOPAN_SCOPE=seller）
///   买家端凭证 → /buyer-api/**    service=buyer   （HUOPAN_SCOPE=buyer）
///
/// 运行：
///   dotnet run -- \
///      --secret-id HPAKID... --secret-key ... --base-url https://www.kingnewe.com --scope buyer
///   或使用环境变量 HUOPAN_SECRET_ID / HUOPAN_SECRET_KEY / HUOPAN_BASE_URL / HUOPAN_SCOPE
///   dotnet run -- --selftest     # 黄金向量自检（无需网络、无需凭证）
/// </summary>
internal static class Example
{
    private static int Main(string[] args)
    {
        if (Array.IndexOf(args, "--selftest") >= 0) return SelfTest();

        string secretId = Arg(args, "--secret-id") ?? Env("HUOPAN_SECRET_ID", "HPAKID你的SecretId");
        string secretKey = Arg(args, "--secret-key") ?? Env("HUOPAN_SECRET_KEY", "你的SecretKey");
        string baseUrl = Arg(args, "--base-url") ?? Env("HUOPAN_BASE_URL", "https://www.kingnewe.com");
        string scope = (Arg(args, "--scope") ?? Env("HUOPAN_SCOPE", "admin")).ToLowerInvariant();

        var client = new HuopanClient(secretId, secretKey, baseUrl);
        Console.WriteLine($"演示端: {scope} | BaseURL: {baseUrl}");

        try
        {
            switch (scope)
            {
                case "seller": RunSeller(client); break;
                case "buyer": RunBuyer(client); break;
                default: RunAdmin(client); break;
            }
            Console.WriteLine("示例执行完成");
            return 0;
        }
        catch (HuopanException ex)
        {
            Console.WriteLine($"请求失败 HTTP {(int)ex.StatusCode}: {ex.Body}");
            Console.WriteLine("错误码含义：E1001 凭证不存在 / E1002 签名失败 / E1003 凭证禁用 / " +
                              "E1004 Nonce 重复 / E1005 时间戳过期 / E1006 签名头错误 / " +
                              "E1007 端不匹配 / E1008 账号异常");
            return 1;
        }
        catch (Exception ex)
        {
            Console.WriteLine($"错误: {ex.GetType().Name}: {ex.Message}");
            return 1;
        }
    }

    // ---------- 超管端 ----------

    private static void RunAdmin(HuopanClient client)
    {
        Show("身份 /api/me", client.Get("/api/me"));

        Show("商品列表 /api/products", client.Get("/api/products", new Dictionary<string, string>
        {
            ["page"] = "1", ["page_size"] = "5", ["keyword"] = "货 盘",
        }));

        // 写操作示例（默认注释，取消注释后真实生效）：
        // Show("创建商品", client.Post("/api/products", "{\"name\":\"示例商品\",\"price\":9.9}"));
    }

    // ---------- 卖家端 ----------

    private static void RunSeller(HuopanClient client)
    {
        Show("卖家概览 /seller-api/dashboard", client.Get("/seller-api/dashboard"));

        Show("商品列表 /seller-api/products", client.Get("/seller-api/products",
            new Dictionary<string, string> { ["page"] = "1", ["page_size"] = "5" }));

        // Show("创建商品", client.Post("/seller-api/products", "{\"name\":\"卖家示例商品\"}"));
    }

    // ---------- 买家端 ----------

    private static void RunBuyer(HuopanClient client)
    {
        Show("买家信息 /buyer-api/profile", client.Get("/buyer-api/profile"));

        Show("订单列表 /buyer-api/orders", client.Get("/buyer-api/orders",
            new Dictionary<string, string> { ["page"] = "1", ["page_size"] = "5" }));

        var products = client.Get("/buyer-api/products", new Dictionary<string, string>
        {
            ["page"] = "1", ["page_size"] = "5",
        });
        Show("商品列表 /buyer-api/products", products);
        PrintSkuPrices(products);

        var detail = client.Get("/buyer-api/products/63");
        Show("商品详情 /buyer-api/products/63", detail);
        PrintSkuPrices(detail);

        // Show("创建地址", client.Post("/buyer-api/addresses",
        //     "{\"receiver\":\"张三\",\"phone\":\"13800138000\"}"));
    }

    private static void PrintSkuPrices(HuopanResponse resp)
    {
        try
        {
            using var doc = resp.Json();
            if (!doc.RootElement.TryGetProperty("data", out var data) ||
                data.ValueKind != JsonValueKind.Object) return;

            if (data.TryGetProperty("skus", out var skus) && skus.ValueKind == JsonValueKind.Array)
                PrintSkus(skus);
            else if (data.TryGetProperty("list", out var list) && list.ValueKind == JsonValueKind.Array)
                foreach (var item in list.EnumerateArray())
                    if (item.TryGetProperty("skus", out var itemSkus))
                        PrintSkus(itemSkus);
        }
        catch (JsonException)
        {
        }
    }

    private static void PrintSkus(JsonElement skus)
    {
        foreach (var sku in skus.EnumerateArray())
        {
            string spec = sku.TryGetProperty("spec_name", out var s) ? s.GetString() : "";
            decimal price = sku.TryGetProperty("price", out var p) && p.TryGetDecimal(out var d) ? d : 0;
            int stock = sku.TryGetProperty("stock", out var st) && st.TryGetInt32(out var n) ? n : 0;
            Console.WriteLine($"    sku: 规格={spec} price={price} stock={stock}");
        }
    }

    // ---------- 工具 ----------

    private static void Show(string tag, HuopanResponse resp)
    {
        Console.WriteLine($"[{tag}] HTTP {resp.StatusCode}");
        try
        {
            using var doc = resp.Json();
            var root = doc.RootElement;
            string code = "";
            if (root.TryGetProperty("code", out var c))
                code = c.ValueKind == JsonValueKind.String ? c.GetString() ?? "" : c.ToString();
            string msg = root.TryGetProperty("msg", out var m) ? m.GetString() ?? "" : "";
            Console.WriteLine($"    code={code} msg={msg}");
            if (root.TryGetProperty("data", out var d))
            {
                string raw = d.GetRawText();
                if (raw.Length > 300) raw = raw.Substring(0, 300) + "...";
                Console.WriteLine($"    data={raw}");
            }
        }
        catch (JsonException)
        {
            string raw = resp.Body ?? "";
            if (raw.Length > 300) raw = raw.Substring(0, 300) + "...";
            Console.WriteLine($"    非JSON响应: {raw}");
        }
    }

    private static string Arg(string[] args, string name)
    {
        int i = Array.IndexOf(args, name);
        return i >= 0 && i + 1 < args.Length ? args[i + 1] : null;
    }

    private static string Env(string key, string fallback)
    {
        string v = Environment.GetEnvironmentVariable(key);
        return string.IsNullOrEmpty(v) ? fallback : v;
    }

    /// <summary>黄金向量自检：验证签名与服务端跨语言一致，无需网络。</summary>
    private static int SelfTest()
    {
        const string sid = "HPAKIDTESTSECRETID00000001";
        const string skey = "TestSecretKey0123456789abcdefABCDEF";
        const string url = "https://www.kingnewe.com/api/products?page=1&page_size=20&keyword=货 盘";
        byte[] body = Encoding.UTF8.GetBytes("{\"name\":\"测试\"}");

        var headers = HuopanClient.Sign(sid, skey, "POST", url, body,
            "5f8a1c2e9b4d4f6a8e0c7a3b5d9e1f2c", "1726646400");

        string want = HuopanConstants.Algorithm +
            " Credential=HPAKIDTESTSECRETID00000001/2024-09-18/admin/hps1_request, " +
            "SignedHeaders=host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp, " +
            "Signature=c4a39375035c1d8d0eeb16e5b922dcac5d9eba27e3f6ed8b3047599e3cec6079";
        string got = headers["Authorization"];

        var checks = new (string Name, bool Ok)[]
        {
            ("黄金向量 Authorization", got == want),
            ("canonical query 编码", HuopanClient.CanonicalQuery("page=1&page_size=20&keyword=货 盘")
                == "keyword=%E8%B4%A7%20%E7%9B%98&page=1&page_size=20"),
            ("query 中 / 编码为 %2F", HuopanClient.SigParamEncode("a/b") == "a%2Fb"),
            ("path 编码保留 /", HuopanClient.SigUriEncode("a/b") == "a/b"),
            ("Host 默认端口省略", HuopanClient.SignHost("www.kingnewe.com:443") == "www.kingnewe.com"),
            ("Host 非默认端口保留", HuopanClient.SignHost("127.0.0.1:8080") == "127.0.0.1:8080"),
            ("空 body 哈希", HuopanClient.HashPayload(Array.Empty<byte>())
                == "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"),
        };

        bool allOk = true;
        foreach (var check in checks)
        {
            Console.WriteLine($"{(check.Ok ? "[PASS]" : "[FAIL]")} {check.Name}");
            allOk &= check.Ok;
        }
        if (!allOk)
        {
            Console.WriteLine($"got authorization:\n{got}\nwant:\n{want}");
            return 1;
        }
        return 0;
    }
}
