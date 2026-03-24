# agent毕设


## 快速开始

### 安装依赖

```bash
npm install
```

### 配置环境变量

创建一个 `.env` 文件并填入你的 API Key：

```bash
# 至少配置一个模型的 API Key
MOONSHOT_API_KEY= xxx
```

### 运行

```bash
# 同时启动前后端
npm run dev

# 或分别启动
cd server && go run ./cmd/server/    # Go 后端 :3001
cd client && npm run dev              # React 前端 :5173

# 构建
npm run build:server    # 编译 Go 二进制
```


## License

MIT
