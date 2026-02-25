// API 基础地址
const API_BASE = '';

// 状态
let state = {
    sessionId: null,
    selectedPrompt: null,
    isLoading: false
};

// DOM 元素
const elements = {
    settingsPanel: document.getElementById('settingsPanel'),
    chatPanel: document.getElementById('chatPanel'),
    promptGrid: document.getElementById('promptGrid'),
    startBtn: document.getElementById('startBtn'),
    hintText: document.getElementById('hintText'),
    providerBadge: document.getElementById('providerBadge'),
    toolBadge: document.getElementById('toolBadge'),
    chatContainer: document.getElementById('chatContainer'),
    userInput: document.getElementById('userInput'),
    sendBtn: document.getElementById('sendBtn'),
    resetBtn: document.getElementById('resetBtn'),
    settingsBtn: document.getElementById('settingsBtn')
};

// 初始化
async function init() {
    await loadPrompts();
    setupEventListeners();
}

// 加载提示词模板
async function loadPrompts() {
    try {
        const response = await fetch(`${API_BASE}/api/prompts`);
        const prompts = await response.json();
        
        elements.promptGrid.innerHTML = prompts.map(p => `
            <div class="select-option" data-id="${p.id}" data-type="prompt">
                <div class="option-name">${p.name}</div>
                <div class="option-desc">${p.description}</div>
            </div>
        `).join('');
        
        if (prompts.length > 0) {
            selectOption('prompt', prompts[0].id);
        }
    } catch (error) {
        console.error('加载提示词模板失败:', error);
        elements.hintText.textContent = '加载失败，请刷新页面重试';
        elements.hintText.classList.add('error');
    }
}

// 选择选项
function selectOption(type, id) {
    const grid = elements.promptGrid;
    const options = grid.querySelectorAll('.select-option');
    
    options.forEach(opt => {
        if (opt.dataset.id === id) {
            opt.classList.add('selected');
            state.selectedPrompt = id;
        } else {
            opt.classList.remove('selected');
        }
    });
    
    updateStartButton();
}

// 更新开始按钮状态
function updateStartButton() {
    const canStart = state.selectedPrompt;
    elements.startBtn.disabled = !canStart;
    elements.hintText.textContent = canStart 
        ? '点击按钮开始对话' 
        : '请选择一个人设模板';
    elements.hintText.classList.remove('error');
}

// 开始会话
async function startSession() {
    if (!state.selectedPrompt) return;
    
    elements.startBtn.disabled = true;
    elements.startBtn.textContent = '连接中...';
    
    try {
        const response = await fetch(`${API_BASE}/api/session`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                promptTemplate: state.selectedPrompt
            })
        });
        
        const data = await response.json();
        
        if (data.success) {
            state.sessionId = data.sessionId;
            elements.providerBadge.textContent = data.provider;
            if (elements.toolBadge) {
                elements.toolBadge.textContent = `${data.toolCount || 0} tools`;
            }
            showChatPanel();
        } else {
            throw new Error(data.error);
        }
    } catch (error) {
        elements.hintText.textContent = `连接失败: ${error.message}`;
        elements.hintText.classList.add('error');
        elements.startBtn.disabled = false;
        elements.startBtn.textContent = '开始对话';
    }
}

// 显示聊天面板
function showChatPanel() {
    elements.settingsPanel.style.display = 'none';
    elements.chatPanel.style.display = 'flex';
    elements.userInput.focus();
}

// 显示设置面板
function showSettingsPanel() {
    elements.chatPanel.style.display = 'none';
    elements.settingsPanel.style.display = 'block';
    elements.startBtn.textContent = '开始对话';
    elements.startBtn.disabled = false;
    updateStartButton();
}

// 发送消息
async function sendMessage() {
    const message = elements.userInput.value.trim();
    if (!message || state.isLoading || !state.sessionId) return;
    
    const welcome = elements.chatContainer.querySelector('.welcome-message');
    if (welcome) welcome.remove();
    
    addMessage(message, 'user');
    elements.userInput.value = '';
    elements.userInput.style.height = 'auto';
    
    state.isLoading = true;
    elements.sendBtn.disabled = true;
    showTypingIndicator();
    
    try {
        const response = await fetch(`${API_BASE}/api/chat`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                sessionId: state.sessionId,
                message
            })
        });
        
        const data = await response.json();
        hideTypingIndicator();
        
        if (data.success) {
            // 如果有工具调用，先显示工具调用过程
            if (data.toolCalls && data.toolCalls.length > 0) {
                addToolCallsBlock(data.toolCalls);
            }
            addMessage(data.response, 'assistant');
        } else {
            addMessage(`错误: ${data.error}`, 'assistant');
        }
    } catch (error) {
        hideTypingIndicator();
        addMessage(`网络错误: ${error.message}`, 'assistant');
    }
    
    state.isLoading = false;
    elements.sendBtn.disabled = false;
    elements.userInput.focus();
}

// 添加工具调用可视化块
function addToolCallsBlock(toolCalls) {
    const div = document.createElement('div');
    div.className = 'tool-calls-block';
    
    const header = document.createElement('div');
    header.className = 'tool-calls-header';
    header.innerHTML = `<span class="tool-icon">🔧</span> 工具调用 (${toolCalls.length})`;
    header.addEventListener('click', () => {
        div.classList.toggle('collapsed');
    });
    div.appendChild(header);

    const content = document.createElement('div');
    content.className = 'tool-calls-content';

    for (const call of toolCalls) {
        const item = document.createElement('div');
        item.className = `tool-call-item ${call.success ? 'success' : 'error'}`;
        
        const toolName = call.tool;
        const serverName = call.server;
        const duration = call.duration;
        const argsPreview = JSON.stringify(call.args, null, 2);
        const resultPreview = call.result.length > 300 
            ? call.result.slice(0, 300) + '...' 
            : call.result;

        item.innerHTML = `
            <div class="tool-call-name">
                <span class="status-dot ${call.success ? 'success' : 'error'}"></span>
                <strong>${toolName}</strong>
                <span class="tool-server">${serverName}</span>
                <span class="tool-duration">${duration}ms</span>
            </div>
            <details class="tool-call-details">
                <summary>查看详情</summary>
                <div class="tool-call-args">
                    <div class="detail-label">参数:</div>
                    <pre>${escapeHtml(argsPreview)}</pre>
                </div>
                <div class="tool-call-result">
                    <div class="detail-label">结果:</div>
                    <pre>${escapeHtml(resultPreview)}</pre>
                </div>
            </details>
        `;
        content.appendChild(item);
    }

    div.appendChild(content);
    elements.chatContainer.appendChild(div);
    scrollToBottom();
}

// HTML 转义
function escapeHtml(text) {
    return text
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

// 添加消息到聊天
function addMessage(content, role) {
    const div = document.createElement('div');
    div.className = `message ${role}`;
    div.innerHTML = formatMessage(content);
    elements.chatContainer.appendChild(div);
    scrollToBottom();
}

// 格式化消息
function formatMessage(content) {
    let escaped = content
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
    
    escaped = escaped.replace(/```(\w*)\n?([\s\S]*?)```/g, (match, lang, code) => {
        return `<pre><code>${code.trim()}</code></pre>`;
    });
    
    escaped = escaped.replace(/`([^`]+)`/g, '<code>$1</code>');
    escaped = escaped.replace(/\n/g, '<br>');
    
    return escaped;
}

// 显示打字指示器
function showTypingIndicator() {
    const indicator = document.createElement('div');
    indicator.className = 'typing-indicator';
    indicator.id = 'typingIndicator';
    indicator.innerHTML = '<span></span><span></span><span></span>';
    elements.chatContainer.appendChild(indicator);
    scrollToBottom();
}

// 隐藏打字指示器
function hideTypingIndicator() {
    const indicator = document.getElementById('typingIndicator');
    if (indicator) indicator.remove();
}

// 滚动到底部
function scrollToBottom() {
    elements.chatContainer.scrollTop = elements.chatContainer.scrollHeight;
}

// 重置对话
async function resetChat() {
    if (!state.sessionId) return;
    
    try {
        await fetch(`${API_BASE}/api/reset`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ sessionId: state.sessionId })
        });
        
        elements.chatContainer.innerHTML = `
            <div class="welcome-message">
                <p>👋 对话已重置，有什么可以帮助你的吗？</p>
            </div>
        `;
    } catch (error) {
        console.error('重置失败:', error);
    }
}

// 设置事件监听
function setupEventListeners() {
    document.addEventListener('click', (e) => {
        const option = e.target.closest('.select-option');
        if (option && !option.classList.contains('disabled')) {
            selectOption(option.dataset.type, option.dataset.id);
        }
    });
    
    elements.startBtn.addEventListener('click', startSession);
    elements.sendBtn.addEventListener('click', sendMessage);
    
    elements.userInput.addEventListener('input', function() {
        this.style.height = 'auto';
        this.style.height = Math.min(this.scrollHeight, 150) + 'px';
    });
    
    elements.userInput.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    });
    
    elements.resetBtn.addEventListener('click', resetChat);
    
    elements.settingsBtn.addEventListener('click', () => {
        if (state.sessionId) {
            fetch(`${API_BASE}/api/session/${state.sessionId}`, { method: 'DELETE' });
            state.sessionId = null;
        }
        showSettingsPanel();
    });
}

// 启动应用
init();
