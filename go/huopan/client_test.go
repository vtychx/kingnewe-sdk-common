package huopan

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"testing"
)

// 黄金测试向量：与 docs/api-signature-design.md 6.4 及服务端
// internal/utils/signature_test.go 保持一致（跨语言一致性基准）。
// 任何偏差都意味着协议实现被破坏，禁止发布。
const (
	goldenSecretID  = "HPAKIDTESTSECRETID00000001"
	goldenSecretKey = "TestSecretKey0123456789abcdefABCDEF"
	goldenNonce     = "5f8a1c2e9b4d4f6a8e0c7a3b5d9e1f2c"
	goldenTS        = "1726646400" // UTC 2024-09-18
	goldenSignature = "c4a39375035c1d8d0eeb16e5b922dcac5d9eba27e3f6ed8b3047599e3cec6079"
)

func TestGoldenVector(t *testing.T) {
	rawURL := "https://www.kingnewe.com/api/products?page=1&page_size=20&keyword=货 盘"
	header, err := SignRequest(goldenSecretID, goldenSecretKey, "POST", rawURL,
		[]byte(`{"name":"测试"}`), goldenNonce, goldenTS)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	auth := header.Get("Authorization")
	want := "HPS1-HMAC-SHA256 Credential=HPAKIDTESTSECRETID00000001/2024-09-18/admin/hps1_request, " +
		"SignedHeaders=host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp, " +
		"Signature=" + goldenSignature
	if auth != want {
		t.Fatalf("Authorization =\n%s\nwant\n%s", auth, want)
	}
}

// 服务端按语义值重排 query，客户端发送顺序不影响验签，但编码必须一致
func TestCanonicalQueryEncoding(t *testing.T) {
	q := url.Values{"keyword": {"货 盘"}, "page": {"1"}, "page_size": {"20"}}
	want := "keyword=%E8%B4%A7%20%E7%9B%98&page=1&page_size=20"
	if got := canonicalQuery(q); got != want {
		t.Fatalf("canonicalQuery = %q, want %q", got, want)
	}
	// / 在 query 值中必须编码为 %2F
	if got := sigParamEncode("a/b"); got != "a%2Fb" {
		t.Fatalf("sigParamEncode = %q, want a%%2Fb", got)
	}
	// 路径编码保留 /
	if got := sigURIEncode("a/b"); got != "a/b" {
		t.Fatalf("sigURIEncode = %q, want a/b", got)
	}
}

// Host 规则：默认端口省略，非默认端口保留
func TestSignHost(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"https://www.kingnewe.com/p", "www.kingnewe.com"},
		{"http://127.0.0.1:8080/p", "127.0.0.1:8080"},
		{"https://www.kingnewe.com:443/p", "www.kingnewe.com"},
	}
	for _, c := range cases {
		u, err := url.Parse(c.raw)
		if err != nil {
			t.Fatalf("parse %s: %v", c.raw, err)
		}
		if got := signHost(u); got != c.want {
			t.Fatalf("signHost(%s) = %q, want %q", c.raw, got, c.want)
		}
	}
}

// 空请求体哈希为协议固定值
func TestHashPayloadEmpty(t *testing.T) {
	want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hashPayload(nil) != want || hashPayload([]byte("")) != want {
		t.Fatal("空请求体哈希与协议定义不符")
	}
}

// New 参数校验
func TestNewValidation(t *testing.T) {
	if _, err := New("", "key", "https://x.com"); err == nil {
		t.Fatal("空 secret_id 应报错")
	}
	c, err := New("id", "key", "https://x.com/")
	if err != nil {
		t.Fatalf("New 失败: %v", err)
	}
	if strings.HasSuffix(c.BaseURL, "/") {
		t.Fatal("BaseURL 应去除尾斜杠")
	}
}

// platformResponse 平台统一响应外层：{"code":0,"msg":"ok","data":{...}}
// code==0 表示业务成功；签名/凭证类错误为字符串码（E1001-E1008，见 README 错误码表）。
type platformResponse struct {
	Code json.RawMessage `json:"code"` // 成功为数字 0，失败可能为字符串 "E1002"
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// ok 判断业务是否成功（兼容数字 0 与字符串 "0"）
func (r *platformResponse) ok() bool {
	return stringsTrim(r.Code) == "0"
}

func stringsTrim(raw json.RawMessage) string {
	s := string(raw)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// TestMain 真实联调测试（需要有效凭证，未配置时自动跳过）。
//
// 凭证类型与路径前缀必须匹配（否则返回 E1007）：
//   - 卖家端凭证 → /seller-api/**（service=seller），环境变量 HUOPAN_SCOPE=seller（默认）
//   - 超管端凭证 → /api/**（service=admin），环境变量 HUOPAN_SCOPE=admin
//
// 运行：
//
//	export HUOPAN_SECRET_ID="HPAKID..." HUOPAN_SECRET_KEY="..." \
//	       HUOPAN_BASE_URL="https://www.kingnewe.com" HUOPAN_SCOPE=seller
//	go test -run TestMain -v
//
// 注意：请勿将真实 SecretKey 硬编码进代码仓库，一律使用环境变量。
func TestMain(t *testing.T) {
	secretID := envOr("HUOPAN_SECRET_ID","xxx")
	secretKey := envOr("HUOPAN_SECRET_KEY","xxx")
	if secretID == "" || secretKey == "" {
		t.Skip("未设置 HUOPAN_SECRET_ID/HUOPAN_SECRET_KEY，跳过真实联调测试")
	}
	baseURL := envOr("HUOPAN_BASE_URL", "https://www.kingnewe.com")
	scope := strings.ToLower(envOr("HUOPAN_SCOPE", "seller"))

	client, err := New(secretID, secretKey, baseURL)
	if err != nil {
		t.Fatalf("初始化客户端失败: %v", err)
	}

	dump := func(tag string, body []byte) {
		s := string(body)
		if len(s) > 200 {
			s = s[:200] + "..."
		}
		t.Logf("%s 响应: %s", tag, s)
	}

	switch scope {
	case "seller":
		// 卖家端凭证 → /seller-api/**（service=seller）
		resp, err := client.Get("/seller-api/dashboard", nil)
		if err != nil {
			t.Fatalf("网络请求失败: %v", err)
		}
		dump("卖家端身份", resp.Body)

		resp, err = client.Get("/seller-api/products", url.Values{
			"page": {"1"}, "page_size": {"5"},
		})
		if err != nil {
			t.Fatalf("网络请求失败: %v", err)
		}
		dump("卖家商品列表", resp.Body)
	default:
		// 超管端凭证 → /api/**（service=admin）
		resp, err := client.Get("/api/me", nil)
		if err != nil {
			t.Fatalf("网络请求失败: %v", err)
		}
		var base platformResponse
		_ = json.Unmarshal(resp.Body, &base)
		if !base.ok() {
			t.Fatalf("获取身份失败: code=%s msg=%s", stringsTrim(base.Code), base.Msg)
		}
		var me struct {
			Admin *struct {
				ID       uint   `json:"id"`
				Username string `json:"username"`
				Role     string `json:"role"`
			} `json:"admin"`
		}
		_ = json.Unmarshal(base.Data, &me)
		if me.Admin != nil {
			t.Logf("当前身份(超管端): id=%d username=%s role=%s",
				me.Admin.ID, me.Admin.Username, me.Admin.Role)
		}

		resp, err = client.Get("/api/products", url.Values{
			"page": {"1"}, "page_size": {"5"}, "keyword": {"货 盘"},
		})
		if err != nil {
			t.Fatalf("网络请求失败: %v", err)
		}
		dump("商品列表", resp.Body)
	}
}
