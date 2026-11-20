# 货盘平台 API 签名 SDK（HPS1-HMAC-SHA256）

多语言签名客户端，供客户以程序化方式调用超管端（`/api`）、卖家端（`/seller-api`）与买家端（`/buyer-api`）接口。
协议规范与错误码：`docs/api-signature-design.md`。

| 语言 | 目录 | 依赖 | 测试 |
|------|------|------|------|
| Golang | `sdk/go/` | 仅标准库（1.21+） | `cd sdk/go && go test ./...` |
| Python | `sdk/python/` | 零依赖（3.8+） | `cd sdk/python && python3 test_huopan_sdk.py` |
| Java | `sdk/java/` | 仅 JDK 11+ | `javac HuopanClient.java && java HuopanClient selftest` |
| JavaScript | `sdk/js/` | 零依赖（Node 18+） | `cd sdk/js && node huopan_test.mjs` |
| TypeScript | `sdk/ts/` | 零依赖（Node 18+） | `cd sdk/ts && node --experimental-strip-types huopan_test.ts` |
| C# | `csharp/` | 仅 .NET 标准库（8+） | `cd csharp && dotnet run -- --selftest` |

所有语言的单元测试共用同一组**黄金测试向量**（文档 6.4），签名结果跨语言逐字节一致。

## 完整示例（example）

每种语言都附带可直接运行的完整示例：初始化 → 签名 GET（含中文 query）→ 解析响应 → POST 写操作（注释形式）→ 错误码处理。示例按 `HUOPAN_SCOPE` 演示三端：`admin`（默认）/ `seller` / `buyer`；买家端示例包含商品列表与商品详情（含 SKU 价格、库存解析）。

| 语言 | 示例文件 | 运行 |
|------|----------|------|
| Go | `sdk/go/example/main.go` | `export HUOPAN_SECRET_ID=... HUOPAN_SECRET_KEY=... HUOPAN_BASE_URL=...` 后 `go run ./example` |
| Python | `sdk/python/example.py` | `python3 example.py` |
| Java | `sdk/java/Example.java` | `javac Example.java && java Example` |
| JavaScript | `sdk/js/example.mjs` | `node example.mjs` |
| TypeScript | `sdk/ts/example.ts` | `node --experimental-strip-types example.ts` |
| C# | `csharp/Example.cs` | `cd csharp && dotnet run -- --secret-id ... --secret-key ... --scope buyer` |

示例默认读环境变量 `HUOPAN_SECRET_ID` / `HUOPAN_SECRET_KEY` / `HUOPAN_BASE_URL` / `HUOPAN_SCOPE`（`admin` 默认、`seller`、`buyer`）。

## 接入流程

1. 登录超管端/卖家端/买家端后台 → 「API 凭证」→ 创建凭证
2. 保存 `SecretId` 与 `SecretKey`（明文仅创建时显示一次）
3. 用对应语言 SDK 初始化客户端并调用业务接口

## 统一行为约定

- `service` 自动推断：路径以 `/buyer-api` 开头 → `buyer`，以 `/seller-api` 开头 → `seller`，其余 → `admin`（凭证类型与访问端不匹配返回 `E1007`，提示信息会指明该凭证应使用的路径前缀）
- **凭证类型与路径前缀必须匹配**：买家端凭证 → `/buyer-api/**`，卖家端凭证 → `/seller-api/**`，超管端凭证 → `/api/**`
- **未携带签名头 = 游客视图**：`/buyer-api/**` 查询接口以游客身份返回，`code` 仍为 `0`，但 `skus[].price` 与 `skus[].stock` 均为 `0`，这是正常行为，不代表接口异常或无数据；必须携带签名头才能看到真实价格与库存
- Host 参与签名：默认端口 80/443 省略；经反向代理访问时服务端优先取 `X-Forwarded-Host`
- body：签名与发送使用同一原始字节，禁止二次序列化/编码
- 时间戳容忍窗口 ±300 秒；每次请求必须生成新 Nonce（32 位 hex）
