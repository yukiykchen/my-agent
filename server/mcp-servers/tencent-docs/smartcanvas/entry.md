# 文档（SmartCanvas）工具完整参考文档

腾讯文档（SmartCanvas）提供了一套完整的文档元素操作 API，支持对页面、文本、标题、待办事项等元素进行增删改查操作。

---

## 目录

- [概念说明](#概念说明)
- [创建智能文档 — create_smartcanvas_by_mdx](#创建智能文档--create_smartcanvas_by_mdx)
- [统一编辑工具（推荐）](#统一编辑工具推荐)
  - [smartcanvas.read - 读取页面全部内容](#smartcanvasread)
  - [smartcanvas.find - 搜索文档内容](#smartcanvasfind)
  - [smartcanvas.edit - 编辑文档内容](#smartcanvasedit)
- [已废弃工具](#已废弃工具)
- [典型工作流示例](#典型工作流示例)

---

## 概念说明

| 概念 | 说明 |
|------|------|
| `file_id` | 文档的唯一标识符，每个文档有唯一的 file_id |
| `page_id` | 页面 ID，Page 是文档的基本容器单元，可通过 `smartcanvas.read` 读取页面内容 |
| `Block ID` | 块 ID，`smartcanvas.read` / `smartcanvas.find` 返回的 MDX 中 `id` 属性值，用于 `smartcanvas.edit` 定位锚点 |

**文档结构**：

```
file_id（文档）
└── Page（页面）
    ├── Heading（标题，level 1-6）
    ├── Paragraph / Text（段落/文本）
    ├── BulletedList / NumberedList（列表）
    ├── Todo（待办事项）
    ├── Table（表格）
    ├── Callout（高亮块）
    ├── ColumnList（分栏布局）
    ├── Image（图片）
    └── ...（更多组件详见 mdx_references.md）
```

> ⚠️ **重要约束**：
> - 所有内容块（Block）必须挂载在 `Page` 下
> - `Page` 可以不指定父节点（挂载到根节点）
> - 完整的组件列表和规范详见 `mdx_references.md`

---

## 创建智能文档 — create_smartcanvas_by_mdx

通过 MDX 格式创建排版丰富的在线智能文档。MDX 支持分栏布局（ColumnList）、高亮块（Callout）、待办列表（Todo）、表格（Table）、带样式文本（Mark）等高级组件。MDX 内容必须严格遵循 `mdx_references.md` 规范生成。

**📖 MDX 规范详见：** `mdx_references.md`

### 工作流

```
1. 阅读 mdx_references.md 了解 MDX 组件规范（组件、属性、取值白名单、格式约束）
2. 按规范生成包含 Frontmatter 和 MDX 组件的内容
3. ⚠️【图片前置检查 - 必须执行】检查 MDX 内容中是否包含 <Image> 图片元素：
   a. 如果包含图片：必须先调用公共工具 upload_image（详见 references/workflows.md）上传每张图片获取 image_id，
      然后将 image_id 设置到 Image 的 src 属性中（如 <Image src="image_id值" alt="描述" />）
   b. 严禁在 src 或 cover 中直接使用 http/https 外部网络链接，系统不支持外部 URL，会导致图片无法显示
   c. 如果图片过大导致 upload_image 上传失败，必须先本地压缩图片，再重新上传，严禁回退使用 URL
4. 对照 mdx_references 逐条自校验，确保格式合规（特别检查 frontmatter cover 和所有 <Image> 的 src 值均为 image_id 而非 URL）
5. 调用 create_smartcanvas_by_mdx 创建文档（传入 title + MDX 内容）
6. 从返回结果中获取 file_id 和 url
```

> ⚠️ **图片强制约束（创建场景 & 编辑场景均适用，包括 frontmatter cover 封面图和正文 Image 组件）**：
> - **绝对禁止**在 `<Image src="...">` 或 frontmatter `cover` 字段中直接使用外部网络 URL（如 `https://example.com/image.png`），系统不支持外部链接，图片将无法显示。
> - **所有图片**（包括封面图 cover 和正文 `<Image>`）**必须**先通过公共工具 `upload_image`（详见 `references/workflows.md`）上传获取 `image_id`，再将 `image_id` 设置到对应属性中。
> - 正确示例（正文图片）：`<Image src="aGVsbG8gd29ybGQ=" alt="描述" />`（src 值为 upload_image 返回的 image_id）
> - 正确示例（封面图）：`cover: aGVsbG8gd29ybGQ=`（cover 值为 upload_image 返回的 image_id）
> - 错误示例：`<Image src="https://example.com/photo.jpg" alt="描述" />`（❌ 严禁使用 URL）
> - 错误示例：`cover: https://example.com/banner.jpg`（❌ 严禁使用 URL）
> - 图片地址必须使用 `src` 属性，严禁使用 `imageId`、`image_id` 等非标准属性名。
> - `image_id` 有效期为一天，请在获取后及时使用。
> - **图片过大处理**：如果图片文件过大导致 `upload_image` 上传失败，**必须先在本地压缩图片**（建议压缩到 50KB 以内）再重新上传，**严禁**因上传失败而回退使用 URL。
> - **注意**：由于 `upload_image` 的 `image_base64` 字段可能非常大（图片越大 Base64 字符串越长），建议通过 Python 脚本等方式直接构造 HTTP 请求调用 MCP 接口，避免 AI 模型逐 token 生成 Base64 字符串导致超时或截断。

### 参数说明

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `title` | string | ✅ | 文档标题，不超过36个字符 |
| `mdx` | string | ✅ | 严格符合 mdx_references 规范的 MDX 格式文本 |

### 调用示例

```json
{
  "title": "项目需求文档",
  "mdx": "---\ntitle: 项目需求文档\nicon: 📋\n---\n\n# 项目需求\n\n<Callout icon=\"📌\" blockColor=\"light_blue\" borderColor=\"blue\">\n    本项目旨在开发一套智能文档管理系统。\n</Callout>\n\n## 功能需求\n\n<BulletedList>\n    文档创建功能\n</BulletedList>\n<BulletedList>\n    文档编辑功能\n</BulletedList>\n<BulletedList>\n    协作功能\n</BulletedList>"
}
```

### 返回值说明

```json
{
  "file_id": "doc_1234567890",
  "url": "https://docs.qq.com/doc/DV2h5cWJ0R1lQb0lH",
  "error": "",
  "trace_id": "trace_1234567890"
}
```

---

## 统一编辑工具（推荐）

> 💡 **推荐使用统一编辑工具**：`smartcanvas.read` + `smartcanvas.find` + `smartcanvas.edit` 组合，支持 MDX 格式内容、更简洁的 API 设计。

### smartcanvas.read

**功能**：获取智能文档指定页面的完整 MDX 格式内容，用于阅读和理解文档全文。

**使用场景**：
- 在编辑文档前先阅读全文，了解文档结构和内容
- 获取页面完整内容用于分析、总结或摘要
- 配合 `smartcanvas.find` + `smartcanvas.edit` 实现"先读取全文，再精准编辑"的工作流

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_id` | string | ✅ | 智能文档的唯一标识符 |
| `page_id` | string | | 要读取的页面 ID，为空时自动获取文档的第一个页面 |

**返回字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `content` | string | 页面全部内容的 MDX 格式文本 |
| `error` | string | 错误信息 |
| `trace_id` | string | 调用链追踪 ID |

**调用示例（读取文档第一个页面的全部内容）**：

```json
{
  "file_id": "your_file_id"
}
```

**调用示例（读取指定页面的全部内容）**：

```json
{
  "file_id": "your_file_id",
  "page_id": "page_abc123"
}
```

**返回示例**：

```json
{
  "content": "## 项目背景\n\n本项目旨在提升用户体验...\n\n## 技术方案\n\n采用微服务架构..."
}
```

---

### smartcanvas.find

**功能**：根据文本搜索智能文档中的 Block，返回匹配 Block 的 ID 和 MDX 格式内容。搜索结果中的 Block ID 可作为锚点，用于 `smartcanvas.edit` 的精准编辑操作。

**使用场景**：
- 定位文档中某段内容的位置，获取 Block ID 作为编辑锚点
- 搜索包含特定关键词的内容块

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_id` | string | ✅ | 智能文档的唯一标识符 |
| `query` | string | ✅ | 搜索文本，系统将在文档所有页面中搜索包含该文本的 Block |

**返回字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `blocks` | array | 匹配的 Block 列表 |
| `blocks[].id` | string | Block 的唯一标识符（锚点 ID） |
| `blocks[].content` | string | Block 的 MDX 格式内容 |
| `error` | string | 错误信息 |
| `trace_id` | string | 调用链追踪 ID |

**调用示例**：

```json
{
  "file_id": "your_file_id",
  "query": "项目背景"
}
```

**返回示例**：

```json
{
  "blocks": [
    {
      "id": "block_abc123",
      "content": "## 项目背景\n\n本项目旨在提升用户体验..."
    }
  ]
}
```

---

### smartcanvas.edit

**功能**：编辑智能文档，支持 4 种操作类型：在指定位置前/后插入、删除、修改。

**操作类型说明**：

| Action | 说明 | id 参数 | content 参数 |
|--------|------|---------|----------|
| `INSERT_BEFORE` | 在指定 Block 前插入内容 | 锚点 Block ID（为空则追加到末尾） | MDX 格式内容（必填） |
| `INSERT_AFTER` | 在指定 Block 后插入内容 | 锚点 Block ID（为空则追加到末尾） | MDX 格式内容（必填） |
| `DELETE` | 删除指定 Block | 要删除的 Block ID（必填，⚠️ 必须先通过 find/read 获取） | 不需要 |
| `UPDATE` | 修改指定 Block 的内容 | 要修改的 Block ID（必填，⚠️ 必须先通过 find/read 获取） | 新的 MDX 格式内容（必填） |

> ⚠️ **强制约束**：`UPDATE` 和 `DELETE` 操作的 `id` 参数**必须**来源于 `smartcanvas.find` 或 `smartcanvas.read` 的返回结果，**禁止**在未获取文档数据的情况下直接传入 id 执行 UPDATE 或 DELETE 操作。

> ⚠️ **readonly 约束**：当 `smartcanvas.find` 或 `smartcanvas.read` 返回的 MDX 内容中，某个块级组件（如 `<Table>`）带有 `readonly` 属性时，表示该组件及其所有子元素为只读状态。**禁止**使用只读组件或其内部子元素的 `id` 作为 `smartcanvas.edit` 的锚点（INSERT_BEFORE / INSERT_AFTER / UPDATE / DELETE 均不可用）。如需在只读组件附近操作，应选择只读组件上方或下方的非只读 Block 作为锚点。

**请求参数**：

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `file_id` | string | ✅ | 智能文档的唯一标识符 |
| `action` | enum | ✅ | 操作类型：INSERT_BEFORE / INSERT_AFTER / DELETE / UPDATE |
| `id` | string | 条件 | 锚点 Block ID，见上表说明 |
| `content` | string | 条件 | MDX 格式内容，见上表说明 |

**返回字段**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `error` | string | 错误信息 |
| `trace_id` | string | 调用链追踪 ID |

**调用示例（在指定 Block 后插入内容）**：

```json
{
  "file_id": "your_file_id",
  "action": "INSERT_AFTER",
  "id": "block_abc123",
  "content": "## 新章节\n\n这是插入的新内容。"
}
```

**调用示例（追加到文档末尾）**：

```json
{
  "file_id": "your_file_id",
  "action": "INSERT_AFTER",
  "content": "追加到文档末尾的内容"
}
```

**调用示例（删除指定 Block）**：

```json
{
  "file_id": "your_file_id",
  "action": "DELETE",
  "id": "block_abc123"
}
```

**调用示例（修改指定 Block）**：

```json
{
  "file_id": "your_file_id",
  "action": "UPDATE",
  "id": "block_abc123",
  "content": "## 修改后的标题\n\n这是更新后的内容。"
}
```

---

### 图片编辑说明

当 `smartcanvas.edit` 的 `content`（MDX 内容）中包含 `<Image>` 图片元素时，需要遵循以下流程：

**图片处理流程**：

```
步骤 1：上传图片获取 image_id
  → 调用公共工具 upload_image（详见 references/workflows.md）上传图片的 base64 内容
  → 从返回结果中获取 image_id（有效期一天）

步骤 2：将 image_id 设置到 MDX 的 Image 组件 src 属性中
  → <Image src="upload_image返回的image_id" alt="描述" />
```

> ⚠️ **重要**：
> - **编辑场景与创建场景一致**：所有图片（包括 frontmatter cover 和正文 `<Image>`）**必须**先通过公共工具 `upload_image`（详见 `references/workflows.md`）上传获取 `image_id`，再将 `image_id` 设置到对应属性中。
> - 图片地址必须使用 `src` 属性，严禁使用 `imageId`、`image_id` 等非标准属性名。
> - `upload_image` 返回的 `image_id` 值也必须设置到 `src` 属性中（如 `<Image src="image_id值" />`）。
> - `image_id` 有效期为一天，请在获取后及时使用。
> - **图片过大处理**：如果图片文件过大导致 `upload_image` 上传失败，**必须先在本地压缩图片**（建议压缩到 50KB 以内）再重新上传，**严禁**因上传失败而回退使用 URL。
> - **注意**：由于 `upload_image` 的 `image_base64` 字段可能非常大（图片越大 Base64 字符串越长），建议通过 Python 脚本等方式直接构造 HTTP 请求调用 MCP 接口，避免 AI 模型逐 token 生成 Base64 字符串导致超时或截断。

**调用示例（使用 upload_image 上传后插入图片）**：

```json
// 步骤 1：先调用 upload_image 获取 image_id
// 步骤 2：将 image_id 设置到 content 的 Image src 属性中
{
  "file_id": "your_file_id",
  "action": "INSERT_AFTER",
  "id": "block_abc123",
  "content": "<Image src=\"upload_image返回的image_id\" alt=\"示例图片\" />"
}
```

---

## 已废弃工具

> ⚠️ 以下工具已废弃，请使用新的统一编辑工具替代。

| 已废弃工具 | 替代方案 |
|-----------|----------|
| `smartcanvas.get_top_level_pages` | 使用 `smartcanvas.read`（page_id 为空时自动获取第一个页面） |
| `smartcanvas.get_page_info` | 使用 `smartcanvas.read` 读取页面完整内容 |
| `smartcanvas.get_element_info` | 使用 `smartcanvas.find` 搜索 Block |
| `smartcanvas.create_smartcanvas_element` | 使用 `smartcanvas.edit`（action=INSERT_BEFORE / INSERT_AFTER） |
| `smartcanvas.delete_element` | 使用 `smartcanvas.edit`（action=DELETE） |
| `smartcanvas.update_element` | 使用 `smartcanvas.edit`（action=UPDATE） |
| `smartcanvas.append_insert_smartcanvas_by_markdown` | 使用 `smartcanvas.edit`（action=INSERT_AFTER，id 为空） |

---

## 典型工作流示例

> ⚠️ **编辑位置定位策略（核心原则）**：
> - **有查询意图 / 用户指定了编辑位置关键词**：优先使用 `smartcanvas.find` 搜索定位。找到后展示给用户确认锚点位置，再执行编辑。找不到则降级使用 `smartcanvas.read` 获取全文来猜测位置。
> - **无查询意图 / 用户未指定编辑位置**：直接使用 `smartcanvas.read` 获取全文内容，根据文档结构猜测合适的锚点位置。插入到最前使用 `INSERT_BEFORE`（指定首个 Block ID），插入到最后使用 `INSERT_AFTER`（id 为空）。
> - **⚠️ UPDATE/DELETE 强制前置条件**：执行 `UPDATE` 或 `DELETE` 操作前，**必须**先通过 `smartcanvas.find` 或 `smartcanvas.read` 获取文档数据，从返回结果中选择具体的锚点 Block ID 后才能执行，**禁止跳过此步骤**。

### 工作流一：用户指定了编辑位置（有查询意图）

```
步骤 1：使用 find 搜索目标 Block
  → smartcanvas.find(file_id, query="用户指定的关键词")
  → 检查搜索结果

步骤 2A：find 找到匹配 Block
  → 将 find 返回的 Block 列表展示给用户确认
  → 用户确认锚点位置后，调用 smartcanvas.edit 传入确认的锚点 ID 执行操作

步骤 2B：find 未找到匹配 Block（降级）
  → 调用 smartcanvas.read(file_id) 获取文档全文内容
  → 根据全文内容分析并猜测合适的锚点位置
  → 调用 smartcanvas.edit 执行编辑操作
```

### 工作流二：用户未指定编辑位置（无查询意图）

```
步骤 1：读取文档全部内容（⚠️ UPDATE/DELETE 操作此步骤为必须）
  → smartcanvas.read(file_id)
  → 获取页面完整 MDX 内容，了解文档结构

步骤 2：根据文档内容和用户意图猜测锚点位置，执行编辑操作
  → 插入到文档最前面：smartcanvas.edit(action=INSERT_BEFORE, id=首个Block ID, content=MDX内容)
  → 插入到文档最后面：smartcanvas.edit(action=INSERT_AFTER, id为空, content=MDX内容)
  → 插入到特定位置：smartcanvas.edit(action=INSERT_BEFORE/INSERT_AFTER, id=猜测的锚点ID, content=MDX内容)
  → 修改特定内容：smartcanvas.edit(action=UPDATE, id=目标Block ID, content=新MDX内容)【id 必须来自 find/read 结果】
  → 删除特定内容：smartcanvas.edit(action=DELETE, id=目标Block ID)【id 必须来自 find/read 结果】
```

### 工作流三：在「XXX」后插入内容

```
步骤 1：搜索定位目标 Block
  → smartcanvas.find(file_id, query="XXX")

步骤 2A：找到匹配 Block
  → 展示 find 结果给用户确认锚点位置
  → 用户确认后，调用 smartcanvas.edit(action=INSERT_AFTER, id=确认的锚点ID, content=MDX内容)

步骤 2B：未找到匹配 Block（降级）
  → smartcanvas.read(file_id) 获取全文
  → 根据全文内容猜测"XXX"附近的锚点位置
  → smartcanvas.edit(action=INSERT_AFTER, id=猜测的锚点ID, content=MDX内容)
```

### 工作流四：修改「XXX」为新内容

```
步骤 1：搜索定位目标 Block
  → smartcanvas.find(file_id, query="XXX")

步骤 2A：找到匹配 Block
  → 展示 find 结果给用户确认目标 Block
  → 用户确认后，调用 smartcanvas.edit(action=UPDATE, id=确认的Block ID, content=新MDX内容)

步骤 2B：未找到匹配 Block（降级）
  → smartcanvas.read(file_id) 获取全文
  → 根据全文内容定位目标位置
  → smartcanvas.edit(action=UPDATE, id=目标Block ID, content=新MDX内容)
```

### 工作流五：删除「XXX」

```
步骤 1：搜索定位目标 Block
  → smartcanvas.find(file_id, query="XXX")

步骤 2A：找到匹配 Block
  → 展示 find 结果给用户确认要删除的 Block
  → 用户确认后，调用 smartcanvas.edit(action=DELETE, id=确认的Block ID)

步骤 2B：未找到匹配 Block（降级）
  → smartcanvas.read(file_id) 获取全文
  → 根据全文内容定位目标位置
  → smartcanvas.edit(action=DELETE, id=目标Block ID)
```

### 工作流六：直接追加内容到文档末尾

```
步骤 1：直接追加到文档末尾（无需定位）
  → smartcanvas.edit(file_id, action=INSERT_AFTER, id为空, content=MDX内容)
```

---

> 📌 **提示**：
> - 所有操作都需要先获取 `file_id`，可通过 `manage.search_file` 搜索文档获取，或在创建文档时从返回结果中获取。
> - **编辑定位优先级**：有查询意图时先 `find` → find 不到则降级 `read` 全文；无查询意图时直接 `read` 全文猜测锚点或使用 `INSERT_BEFORE`（最前）/ `INSERT_AFTER`（id 为空，最后）。
> - **UPDATE/DELETE 强制前置条件**：执行 `UPDATE` 或 `DELETE` 操作前，**必须**先通过 `find` 或 `read` 获取文档数据并从中选择具体锚点 Block ID，禁止跳过此步骤。
> - `find` 返回结果后应展示给用户确认锚点位置，而非直接使用第一个结果。
> - 所有内容块必须挂载在 `Page` 下，完整组件列表详见 `mdx_references.md`。
