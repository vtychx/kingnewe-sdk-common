// huopan-sdk-go 使用示例：初始化客户端 → 签名调用 → 解析响应 → 错误处理。
//
// ============================ 凭证与访问端匹配规则 ============================
//
// 平台有三类 API 凭证（在对应后台的「API凭证」页面创建），与访问路径一一对应，
// 不能交叉访问（混用返回 E1007，提示信息会指明该凭证应使用的路径前缀）：
//
//	超管端凭证（在超管后台创建） → 调用 /api/**        路径，service=admin
//	卖家端凭证（在卖家后台创建） → 调用 /seller-api/** 路径，service=seller
//	买家端凭证（在买家后台创建） → 调用 /buyer-api/**  路径，service=buyer
//
// SDK 会根据请求路径前缀自动推断 service，无需手动指定。
//
// ============================ 运行 ============================
//
// 运行前准备：
//  1. 登录超管端/卖家端/买家端后台 →「API凭证」→ 创建凭证，保存 SecretId 与 SecretKey
//     （明文仅创建时展示一次）
//  2. 设置环境变量（或直接修改下面的默认值）：
//     export HUOPAN_SECRET_ID="HPAKIDxxxx"
//     export HUOPAN_SECRET_KEY="你的SecretKey"
//     export HUOPAN_BASE_URL="https://www.kingnewe.com"  # 测试环境可用 http://101.42.20.80:8080
//     export HUOPAN_SCOPE="admin"   # admin=演示超管端（默认）；seller=演示卖家端；buyer=演示买家端
//  3. 运行：go run ./example
//
// ============================ 响应结构 ============================
//
// 平台统一响应外层：{"code":0,"msg":"ok","data":{...}}
//   - code==0 表示业务成功
//   - 签名/凭证类错误 code 为字符串 "E1001"-"E1008"（E1001 凭证不存在、
//     E1002 签名失败、E1003 凭证已禁用、E1004 Nonce 重复、E1005 时间戳过期、
//     E1006 签名头格式错误、E1007 端不匹配、E1008 账号状态异常，详见 README）
package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"huopan-sdk-go/huopan"
)

// platformResponse 平台统一响应外层
type platformResponse struct {
	Code json.RawMessage `json:"code"` // 成功为数字 0，失败可能为字符串 "E1002"
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// ok 判断业务是否成功
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

func main() {
	// ---------- 1. 初始化客户端 ----------
	client, err := huopan.New(
		envOr("HUOPAN_SECRET_ID", "HPAKID你的SecretId"),
		envOr("HUOPAN_SECRET_KEY", "你的SecretKey"),
		envOr("HUOPAN_BASE_URL", "https://www.kingnewe.com"),
	)
	if err != nil {
		panic(err)
	}

	// ---------- 2. 按凭证类型演示对应端的调用 ----------
	// 超管端凭证 → HUOPAN_SCOPE=admin（默认）
	// 卖家端凭证 → HUOPAN_SCOPE=seller
	// 买家端凭证 → HUOPAN_SCOPE=buyer
	switch strings.ToLower(envOr("HUOPAN_SCOPE", "admin")) {
	case "seller":
		runSellerExamples(client)
	case "buyer":
		runBuyerExamples(client)
	default:
		runAdminExamples(client)
	}
	fmt.Println("示例执行完成")
}

// runAdminExamples 超管端凭证调用 /api/** 接口
func runAdminExamples(client *huopan.Client) {
	// ---- 2.1 GET /api/me：获取当前凭证对应的超管账号身份 ----
	resp, err := client.Get("/api/me", nil)
	if err != nil {
		panic(fmt.Errorf("网络请求失败: %w", err))
	}
	var base platformResponse
	_ = json.Unmarshal(resp.Body, &base)
	if !base.ok() {
		fmt.Printf("获取身份失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
		return
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
		fmt.Printf("当前身份(超管端): id=%d username=%s role=%s\n",
			me.Admin.ID, me.Admin.Username, me.Admin.Role)
	}

	// ---- 2.2 GET 带查询参数（含中文与空格，SDK 自动 RFC3986 编码） ----
	// data 的具体字段以实际接口文档为准，此处演示通用解析方式
	resp, err = client.Get("/api/products", url.Values{
		"page": {"1"}, "page_size": {"5"}, "keyword": {"货 盘"},
	})
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(resp.Body, &base)
	if base.ok() {
		fmt.Printf("商品列表(超管端): %s\n", dataExcerpt(base.Data))
	} else {
		fmt.Printf("商品列表失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
	}

	// ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
	// resp, err = client.Post("/api/products", map[string]interface{}{
	// 	"name":  "示例商品",
	// 	"price": 9.9,
	// })
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("创建商品:", resp.StatusCode, string(resp.Body))
}

// runSellerExamples 卖家端凭证调用 /seller-api/** 接口
func runSellerExamples(client *huopan.Client) {
	// ---- 2.1 GET /seller-api/dashboard：卖家端概览（含店铺与账号信息） ----
	resp, err := client.Get("/seller-api/dashboard", nil)
	if err != nil {
		panic(fmt.Errorf("网络请求失败: %w", err))
	}
	var base platformResponse
	_ = json.Unmarshal(resp.Body, &base)
	if !base.ok() {
		fmt.Printf("获取卖家概览失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
		return
	}
	fmt.Printf("卖家概览: %s\n", dataExcerpt(base.Data))

	// ---- 2.2 GET /seller-api/products：当前卖家的商品列表（数据按凭证卖家隔离） ----
	resp, err = client.Get("/seller-api/products", url.Values{
		"page": {"1"}, "page_size": {"5"},
	})
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(resp.Body, &base)
	if base.ok() {
		fmt.Printf("商品列表(卖家端): %s\n", dataExcerpt(base.Data))
	} else {
		fmt.Printf("商品列表失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
	}

	// ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
	// resp, err = client.Post("/seller-api/products", map[string]interface{}{
	// 	"name": "卖家示例商品",
	// })
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("创建商品:", resp.StatusCode, string(resp.Body))
}

// runBuyerExamples 买家端凭证调用 /buyer-api/** 接口
func runBuyerExamples(client *huopan.Client) {
	// ---- 2.1 GET /buyer-api/profile：当前买家个人信息 ----
	resp, err := client.Get("/buyer-api/profile", nil)
	if err != nil {
		panic(fmt.Errorf("网络请求失败: %w", err))
	}
	var base platformResponse
	_ = json.Unmarshal(resp.Body, &base)
	if !base.ok() {
		fmt.Printf("获取买家信息失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
		return
	}
	fmt.Printf("买家信息: %s\n", dataExcerpt(base.Data))

	// ---- 2.2 GET /buyer-api/orders：当前买家的订单列表（数据按凭证买家隔离） ----
	resp, err = client.Get("/buyer-api/orders", url.Values{
		"page": {"1"}, "page_size": {"5"},
	})
	if err != nil {
		panic(err)
	}
	_ = json.Unmarshal(resp.Body, &base)
	if base.ok() {
		fmt.Printf("订单列表(买家端): %s\n", dataExcerpt(base.Data))
	} else {
		fmt.Printf("订单列表失败: code=%s msg=%s\n", stringsTrim(base.Code), base.Msg)
	}

	// ---- 2.3 POST 写操作示例（默认注释，取消注释后真实生效） ----
	// resp, err = client.Post("/buyer-api/addresses", map[string]interface{}{
	// 	"receiver": "张三",
	// 	"phone":    "13800138000",
	// })
	// if err != nil {
	// 	panic(err)
	// }
	// fmt.Println("创建地址:", resp.StatusCode, string(resp.Body))
}

// dataExcerpt 截断展示 data 便于控制台查看
func dataExcerpt(data json.RawMessage) string {
	s := string(data)
	if len(s) > 200 {
		return s[:200] + "..."
	}
	if s == "" {
		return "<无数据>"
	}
	return s
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
