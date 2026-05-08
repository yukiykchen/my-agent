# 网络侵权证据智能系统（NIEIS）

> 本科毕业设计《网络侵权证据智能系统设计与实现》配套工程。
>
> 以 **证据链条五阶段流转**（生成 → 固化 → 传输 → 存储 → 验证）为主线，
> 结合 **ReAct 智能体 + MCP 工具总线 + 法律三段论 Prompt Engineering**，
> 面向淘宝 / 抖音 / 拼多多 / 知乎等主流平台的网络侵权取证场景。

---

## ✨ 主要能力

### 1. 证据链条全流程闭环（论文 4.1-4.5）

| 阶段 | 技术实现 | 对应接口 |
|------|---------|----------|
| **生成** | 无头浏览器网页取证 / MediaRecorder 5 秒分段录屏 / yt-dlp 短视频抓取 | `mcp_web_screenshot` / 前端直播取证面板 |
| **固化**（**1 秒级原子化**） | Web Crypto API 即时哈希 + RFC 3161 TSA 时间戳 + `chmod 444` 只读保护 + `fsnotify` 监控 | `POST /api/notarize/fixate` |
| **传输** | HTTPS 上传 + 服务端哈希二次校验 | 上传流水线内置 |
| **存储** | 本地分层存储 + 腾讯文档云端备份 + **蚂蚁链 Merkle Tree 批量上链** | `POST /api/chain/{caseId}/anchor` |
| **验证** | **客户端 / 服务端 / TSA / 区块链 四值哈希链一致性校验** | `POST /api/notarize/{caseId}/{evidenceId}/verify` |

### 2. AI 分析与法律规范化报告

- 自研 Go ReAct Agent 引擎（最多 20 轮迭代 + 阶段性总结机制）
- 通过 **MCP 协议** 接入 8+ 工具服务器
- **法律三段论提示工程**：7 部法律的规范结构编码至系统提示词（见 `server/prompts/infringement-analyst.md`）
- 七段式法律规范化报告（封面 / 案件概述 / 证据清单 / 真实性校验 / 侵权事实 / 法律三段论 / 结论建议）

### 3. 监督链（Chain of Custody）

所有证据的采集 / 复制 / 传输 / 校验 / 上链操作均追加至 `custody.log.jsonl`，
采用链式哈希（`LinkHash = sha256(prev || body)`），
支持通过 `GET /api/custody/{caseId}/verify` 独立验证日志本身未被篡改。

---

## 📦 目录结构

```
my-agent/
├── server/                           Go 后端
│   ├── cmd/server/main.go            入口
│   ├── internal/
│   │   ├── agent/                    ReAct Agent 引擎
│   │   ├── notary/                   证据固化 / TSA / Merkle / 区块链 / 监督链
│   │   ├── evidence/                 证据存储 / 固化服务 / 链上锚定
│   │   ├── report/                   法律规范化报告生成器（七段式）
│   │   ├── tools/                    Agent 工具注册（含 evidence_* / asr_/ocr_/report_generate）
│   │   ├── mcp/                      MCP Client & Bridge
│   │   ├── providers/                多模型适配（Moonshot / DeepSeek / ...）
│   │   └── prompt/                   Prompt 管理
│   ├── mcp-servers/                  自研 MCP 工具服务（screenshot / webcrawl / textcompare 等）
│   └── prompts/infringement-analyst.md   法律三段论提示词
├── client/                           React 18 + Vite + TS 前端
│   └── src/
│       ├── services/
│       │   ├── evidenceCrypto.ts     Web Crypto 即时哈希 + 固化调用
│       │   └── liveRecorder.ts       MediaRecorder 5 秒分段录屏
│       └── components/
│           ├── ChatPanel.tsx         对话式交互主界面
│           ├── LiveCapturePanel.tsx  直播取证面板（5 秒分段即时固化）
│           ├── WebCapturePanel.tsx   网页取证面板（截图/正文抓取 + 即时固化）
│           └── EvidenceBrowser.tsx   证据列表浏览器（校验/上链/预览）
└── 毕业论文 / 开题报告 / 参考文献 等
```

---

## 🚀 快速开始

### 配置环境变量

```bash
# server/.env
MOONSHOT_API_KEY=sk-xxxxxxx
DEEPSEEK_API_KEY=sk-xxxxxxx     # 可选
# 其他可选模型 Key 见 server/internal/config/config.go
```

### 启动

```bash
# 同时启动前后端
npm run dev

# 或分别启动
cd server && go run ./cmd/server     # 后端 http://localhost:3001
cd client && npm run dev             # 前端 http://localhost:5173

# 构建
npm run build:server                 # 编译 Go 二进制
cd client && npm run build           # 前端生产构建
```

启动后后端日志会显示：
```
  ✅ MCP 工具注册完成，共 166 个工具
  ✅ 证据即时固化服务已启动（Mock TSA + fsnotify + 监督链）
  ✅ Mock 蚂蚁链存证服务已启动（Merkle Tree 批量聚合）
  ✅ 证据链条工具集已注册（evidence_list / evidence_verify / evidence_anchor / ...）
```

---

## 🧪 端到端测试

项目提供了两个测试脚本：

### 1. 证据链全流程 smoke test

```bash
python3 /tmp/smoketest.py
```

涵盖：固化 → 四值哈希链校验 → Merkle 批量上链 → 独立链上 Merkle Proof 验证 → 监督链审计。

### 2. 篡改检测测试

```bash
python3 /tmp/tamper_test.py
```

模拟攻击者强制修改已固化文件，验证四值哈希链能否识别。

---

## 📡 核心 HTTP API

### 证据即时固化

```http
POST /api/notarize/fixate
Content-Type: multipart/form-data

file:         <binary>
caseId:       case_xxx
sourceType:   web | live_segment | short_video | document
clientHash:   <必填：客户端 Web Crypto API 计算的 SHA-256 hex>
collector:    <采集者>
```

返回：`NotarizedEvidence`（含 hashChain、tsaToken、fixationLatencyMs 等）

### 四值哈希链校验

```http
POST /api/notarize/{caseId}/{evidenceId}/verify
```

返回 `{ consistent: true|false, reason: "...", evidence: {...} }`

### Merkle Tree 批量上链

```http
POST /api/chain/{caseId}/anchor
Content-Type: application/json

{ "evidenceIds": ["ev_1", "ev_2"] }   # 可选，不填则上链全部未上链证据
```

### 独立链上验证

```http
GET /api/chain/{caseId}/{evidenceId}/verify
```

### 监督链

```http
GET  /api/custody/{caseId}              # 列出所有事件
GET  /api/custody/{caseId}/verify       # 校验监督链自身完整性
```

---

## 🛠 Agent 工具列表（证据链条相关）

| 工具 | 描述 |
|------|------|
| `evidence_list` | 列出案件所有已固化证据 |
| `evidence_verify` | 校验四值哈希链一致性 |
| `evidence_anchor` | Merkle Tree 批量上链 |
| `custody_list` | 查看监督链审计日志 |
| `asr_transcribe` | 对直播片段做 ASR 转写（Mock Whisper，可替换为真实服务） |
| `ocr_recognize` | 对关键帧做 OCR 识别（Mock PaddleOCR） |
| `report_generate` | 生成符合法律文书规范的七段式《网络侵权行为分析报告》 |

---

## 📝 重要说明

本工程出于毕业设计场景需要，对 **TSA 可信时间戳** 和 **蚂蚁链司法存证**
采用了 **Mock 实现**（符合 RFC 3161 数据结构 + 本地 ECDSA-P256 签名 + 本地 append-only 账本），
接口设计与真实 API 对齐，可直接替换为：
- 联合信任时间戳服务中心真实 SDK
- 蚂蚁链 / 保全网 / 易保全 真实 SDK

Mock 版本已能完整验证论文提出的所有方法论（四值哈希链、Merkle Proof、监督链链式哈希等）。

---

## License

MIT
