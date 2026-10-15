// Package huopan 货盘平台 API 签名客户端（HPS1-HMAC-SHA256）。
// 协议规范：docs/api-signature-design.md 第 6 章；仅依赖 Go 标准库。
//
// 用法：
//
//	c, err := huopan.New("HPAKID...", "SecretKey...", "https://www.kingnewe.com")
//	resp, err := c.Post("/api/products", map[string]any{"name": "测试商品"})
//	var out map[string]any
//	_ = resp.JSON(&out)
package huopan

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// Algorithm 签名算法标识
	Algorithm = "HPS1-HMAC-SHA256"
	// signedHeaders 参与签名的固定头集合（小写、有序、分号分隔）
	signedHeaders = "host;x-hp-algorithm;x-hp-nonce;x-hp-secretid;x-hp-service;x-hp-timestamp"
)

// Client API 签名客户端（可并发使用）
type Client struct {
	SecretID  string
	SecretKey string
	BaseURL   string // 如 https://www.kingnewe.com（不带尾斜杠）
	HTTP      *http.Client
}

// New 创建签名客户端；baseURL 不带尾斜杠，如 https://www.kingnewe.com
func New(secretID, secretKey, baseURL string) (*Client, error) {
	if secretID == "" || secretKey == "" || baseURL == "" {
		return nil, errors.New("secret_id / secret_key / base_url 均不能为空")
	}
	return &Client{
		SecretID:  secretID,
		SecretKey: secretKey,
		BaseURL:   strings.TrimRight(baseURL, "/"),
		HTTP:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Response 响应体
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// JSON 将响应体解析到 v
func (r *Response) JSON(v interface{}) error {
	return json.Unmarshal(r.Body, v)
}

// Get 发起签名 GET 请求；query 传 url.Values（可为 nil）
func (c *Client) Get(path string, query url.Values) (*Response, error) {
	return c.DoRequest(http.MethodGet, path, query, nil)
}

// Post 发起签名 POST 请求；body 为可 JSON 序列化对象（可为 nil）
func (c *Client) Post(path string, body interface{}) (*Response, error) {
	return c.doWithJSON(http.MethodPost, path, body)
}

// Put 发起签名 PUT 请求
func (c *Client) Put(path string, body interface{}) (*Response, error) {
	return c.doWithJSON(http.MethodPut, path, body)
}

// Delete 发起签名 DELETE 请求
func (c *Client) Delete(path string) (*Response, error) {
	return c.DoRequest(http.MethodDelete, path, nil, nil)
}

// doWithJSON 序列化 body 后走 DoRequest（签名与发送使用同一原始字节）
func (c *Client) doWithJSON(method, path string, body interface{}) (*Response, error) {
	var raw []byte
	if body != nil {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(body); err != nil {
			return nil, fmt.Errorf("序列化请求体失败: %w", err)
		}
		raw = bytes.TrimRight(buf.Bytes(), "\n")
	}
	return c.DoRequest(method, path, nil, raw)
}

// DoRequest 核心方法：生成时间戳/Nonce → 签名 → 发送
// service 按路径自动推断：/buyer-api→buyer、/seller-api→seller，其余为 admin
func (c *Client) DoRequest(method, path string, query url.Values, body []byte) (*Response, error) {
	rawURL := c.BaseURL + path
	if len(query) > 0 {
		rawURL += "?" + encodeQueryRaw(query)
	}
	nonce, err := randHex(32)
	if err != nil {
		return nil, err
	}
	header, err := SignRequest(c.SecretID, c.SecretKey, method, rawURL, body, nonce,
		strconv.FormatInt(time.Now().Unix(), 10))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(strings.ToUpper(method), rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		req.Header.Set(k, v[0])
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: data}, nil
}

// SignRequest 计算 HPS1 签名并返回需附加的请求头（含 Authorization）。
// rawURL 为完整 URL；nonce/ts 由调用方生成（测试时固定以对齐黄金向量）。
func SignRequest(secretID, secretKey, method, rawURL string, body []byte, nonce, ts string) (
	http.Header, error,
) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("URL 解析失败: %w", err)
	}
	service := "admin"
	switch {
	case strings.HasPrefix(u.Path, "/buyer-api"):
		service = "buyer"
	case strings.HasPrefix(u.Path, "/seller-api"):
		service = "seller"
	}
	canonicalHeaders := map[string]string{
		"host":           signHost(u),
		"x-hp-algorithm": Algorithm,
		"x-hp-nonce":     nonce,
		"x-hp-secretid":  secretID,
		"x-hp-service":   service,
		"x-hp-timestamp": ts,
	}
	canonical := strings.Join([]string{
		strings.ToUpper(method),
		sigURIEncode(u.EscapedPath()),
		canonicalQuery(u.Query()),
		buildCanonicalHeaders(canonicalHeaders),
		signedHeaders,
		hashPayload(body),
	}, "\n")
	date := time.Unix(mustAtoi(ts), 0).UTC().Format("2006-01-02")
	sts := strings.Join([]string{Algorithm, ts, date, hashPayload([]byte(canonical))}, "\n")
	kDate := hmacSHA256([]byte("HPS1"+secretKey), []byte(date))
	kService := hmacSHA256(kDate, []byte(service))
	sig := hex.EncodeToString(hmacSHA256(hmacSHA256(kService, []byte("hps1_request")), []byte(sts)))
	authorization := fmt.Sprintf("%s Credential=%s/%s/%s/hps1_request, SignedHeaders=%s, Signature=%s",
		Algorithm, secretID, date, service, signedHeaders, sig)
	header := http.Header{}
	for k, v := range canonicalHeaders {
		header.Set(k, v)
	}
	header.Set("Authorization", authorization)
	return header, nil
}

// ==================== 规范化构造（与服务端 utils/signature.go 同源语义） ====================

// sigURIEncode RFC3986 编码，保留路径分隔符 /；unreserved 集合不编码
func sigURIEncode(s string) string {
	return rfc3986Encode(s, true)
}

// sigParamEncode RFC3986 编码（/ 也编码），用于 query 的 k/v
func sigParamEncode(s string) string {
	return rfc3986Encode(s, false)
}

func rfc3986Encode(s string, keepSlash bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteByte(c)
		case c == '-' || c == '_' || c == '.' || c == '~':
			b.WriteByte(c)
		case c == '/' && keepSlash:
			b.WriteByte(c)
		default:
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

// encodeQueryRaw 将 url.Values 编码为可直接拼接的 query 串（与签名侧排序语义一致）
func encodeQueryRaw(q url.Values) string {
	type kv struct{ k, v string }
	pairs := make([]kv, 0, len(q))
	for k, vs := range q {
		ek := sigParamEncode(k)
		for _, v := range vs {
			pairs = append(pairs, kv{ek, sigParamEncode(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.k + "=" + p.v
	}
	return strings.Join(parts, "&")
}

// canonicalQuery 规范化 query：先编码每对 k/v，再按 (k, v) 字节序排序
func canonicalQuery(q url.Values) string {
	type kv struct{ k, v string }
	pairs := make([]kv, 0, len(q))
	for k, vs := range q {
		ek := sigParamEncode(k)
		for _, v := range vs {
			pairs = append(pairs, kv{ek, sigParamEncode(v)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].k != pairs[j].k {
			return pairs[i].k < pairs[j].k
		}
		return pairs[i].v < pairs[j].v
	})
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = p.k + "=" + p.v
	}
	return strings.Join(parts, "&")
}

// buildCanonicalHeaders 规范化头（按名称小写字典序，value 压缩空白）
func buildCanonicalHeaders(h map[string]string) string {
	names := make([]string, 0, len(h))
	for k := range h {
		names = append(names, k)
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(strings.Join(strings.Fields(h[name]), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

// signHost 提取签名用 Host（默认端口 80/443 省略，小写）
func signHost(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	if p := u.Port(); p != "" && p != "80" && p != "443" {
		return host + ":" + p
	}
	return host
}

// hashPayload body 原始字节 SHA256 小写 hex（nil 等价空串）
func hashPayload(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key, msg []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(msg)
	return m.Sum(nil)
}

func mustAtoi(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// randHex 生成 n 个字符的随机 hex（n/2 字节）
func randHex(n int) (string, error) {
	b := make([]byte, n/2)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("生成 nonce 失败: %w", err)
	}
	return hex.EncodeToString(b), nil
}
