/* eslint-disable no-undef */
// @ts-nocheck
/**
 * LocalMind Operator Dashboard — Client-side Logic
 * Merges sidebar command center + new dashboard panels
 */
const vscode = acquireVsCodeApi();

// =========================================================================
// State
// =========================================================================
let activeTab = 'agent';
let currentStreamId = null;
let currentStreamElement = null;
let lessonFilter = 'all';

// =========================================================================
// Initialization
// =========================================================================
window.addEventListener('load', () => {
    // Tab switching
    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.addEventListener('click', () => switchTab(btn.dataset.tab));
    });

    // Command input (Agent tab)
    const input = document.getElementById('commandInput');
    if (input) {
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                const command = input.value.trim();
                if (command) {
                    vscode.postMessage({ type: 'command', text: command });
                    input.value = '';
                    const list = document.getElementById('planList');
                    if (list) {
                        list.innerHTML = `<li class="plan-item active">
                            <span class="checkbox">[⚡]</span>
                            <span style="opacity:0.8; font-style:italic;">Ollama is analyzing workspace & planning...</span>
                        </li>`;
                    }
                }
            }
        });
    }

    // Action buttons
    document.getElementById('btnApprove')?.addEventListener('click', () => {
        vscode.postMessage({ type: 'approve' });
        const list = document.getElementById('planList');
        if (list) {
            const items = list.querySelectorAll('.plan-item');
            items.forEach(item => item.style.opacity = '0.5');
            const loader = document.createElement('li');
            loader.className = 'plan-item active';
            loader.innerHTML = `
                <span class="checkbox">[▸]</span>
                <span style="opacity:0.8; font-style:italic;">Ollama is executing task...</span>
            `;
            list.appendChild(loader);
        }
    });
    document.getElementById('btnReject')?.addEventListener('click', () => {
        vscode.postMessage({ type: 'reject' });
    });
    document.getElementById('btnAbort')?.addEventListener('click', () => {
        vscode.postMessage({ type: 'abort' });
    });

    // Refresh buttons
    document.getElementById('btnRefreshHistory')?.addEventListener('click', () => {
        vscode.postMessage({ type: 'fetchHistory' });
    });
    document.getElementById('btnRefreshScores')?.addEventListener('click', () => {
        vscode.postMessage({ type: 'fetchScores' });
    });

    // Message handler
    window.addEventListener('message', event => {
        const msg = event.data;
        switch (msg.type) {
            // Agent tab
            case 'updatePlan': renderPlan(msg.plan); break;
            case 'log': addLog(msg.text, msg.level); break;
            case 'status': updateStatus(msg.status, msg.model, msg.ollamaStatus, msg.modelCount); break;
            case 'stream': handleStream(msg.requestId, msg.chunk); break;
            // Dashboard tabs
            case 'historyData': renderHistory(msg.data); break;
            case 'scoresData': renderScores(msg.data); break;
            case 'strategiesData': renderStrategies(msg.data); break;
            case 'lessonsData': renderLessons(msg.data); break;
        }
    });
});

// =========================================================================
// Tab Switching
// =========================================================================
function switchTab(tabId) {
    activeTab = tabId;

    document.querySelectorAll('.tab-btn').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.tab === tabId);
    });
    document.querySelectorAll('.tab-panel').forEach(panel => {
        panel.classList.toggle('active', panel.id === `panel-${tabId}`);
    });

    // Lazy-load data when switching to a tab
    switch (tabId) {
        case 'history': vscode.postMessage({ type: 'fetchHistory' }); break;
        case 'scores': vscode.postMessage({ type: 'fetchScores' }); break;
        case 'strategies': vscode.postMessage({ type: 'fetchStrategies' }); break;
        case 'lessons': vscode.postMessage({ type: 'fetchLessons' }); break;
    }
}

// =========================================================================
// Agent Tab — Plan, Log, Status, Stream
// =========================================================================
function renderPlan(plan) {
    const list = document.getElementById('planList');
    if (!list) return;
    list.innerHTML = '';

    if (plan && plan.steps) {
        plan.steps.forEach(step => {
            const li = document.createElement('li');
            li.className = `plan-item ${step.status || ''}`;

            let icon = '[ ]';
            if (step.status === 'done') icon = '[✓]';
            if (step.status === 'active') icon = '[▸]';
            if (step.status === 'failed') icon = '[✗]';

            li.innerHTML = `
                <span class="checkbox">${icon}</span>
                <div style="display:flex; flex-direction:column; gap:4px; line-height: 1.3;">
                    <span>${escapeHtml(step.description || step.type || 'Step')}</span>
                    ${step.file ? `<span style="font-size:11px; opacity:0.6;">📁 ${escapeHtml(step.file)}</span>` : ''}
                </div>
            `;
            list.appendChild(li);
        });
    }
}

function addLog(text, level = 'info') {
    currentStreamId = null;
    currentStreamElement = null;

    const container = document.getElementById('logContainer');
    if (!container) return;

    const div = document.createElement('div');
    const time = new Date().toLocaleTimeString('en-US', { hour12: false });

    div.className = 'log-entry';
    div.innerHTML = `
        <span class="log-time">${time}</span>
        <span class="log-msg ${level}">${escapeHtml(text)}</span>
    `;

    container.appendChild(div);
    container.scrollTop = container.scrollHeight;
}

function updateStatus(status, model, ollamaStatus, modelCount) {
    const dot = document.getElementById('statusDot');
    const text = document.getElementById('statusText');
    const modelInfo = document.getElementById('modelInfo');
    if (!dot || !text) return;

    if (status === 'online') {
        dot.className = 'status-dot';
        text.innerText = 'ONLINE';
        text.style.color = 'var(--success)';
    } else if (status === 'ollama_offline') {
        dot.className = 'status-dot offline';
        text.innerText = 'OLLAMA OFFLINE';
        text.style.color = 'var(--warning, #f0ad4e)';
    } else {
        dot.className = 'status-dot offline';
        text.innerText = 'OFFLINE';
        text.style.color = 'var(--danger)';
    }

    // Update model info display
    if (modelInfo) {
        if (model) {
            modelInfo.textContent = `${model} • ${modelCount || 0} model(s)`;
        } else if (status === 'ollama_offline') {
            modelInfo.textContent = 'Ollama not running • Start with: ollama serve';
        } else {
            modelInfo.textContent = 'Ollama • Local-First Agent';
        }
    }
}

function handleStream(requestId, chunk) {
    const container = document.getElementById('logContainer');
    if (!container) return;

    if (currentStreamId !== requestId || !currentStreamElement) {
        currentStreamId = requestId;
        const div = document.createElement('div');
        div.className = 'log-entry stream';
        div.innerHTML = `
            <span class="log-time">stream</span>
            <span class="log-msg stream-content"></span>
        `;
        container.appendChild(div);
        currentStreamElement = div.querySelector('.stream-content');
    }

    if (currentStreamElement) {
        currentStreamElement.textContent += chunk;
        container.scrollTop = container.scrollHeight;
    }
}

// =========================================================================
// History Tab
// =========================================================================
function renderHistory(tasks) {
    const container = document.getElementById('historyList');
    if (!container) return;

    if (!tasks || tasks.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📋</div>
                No task history yet. Run your first agent task!
            </div>`;
        return;
    }

    container.innerHTML = tasks.map(task => {
        const statusIcon = task.status === 'completed' ? '✓' : task.status === 'failed' ? '✗' : '⟳';
        const statusClass = task.status === 'completed' ? 'completed' : task.status === 'failed' ? 'failed' : 'running';
        const steps = task.steps || 0;
        const duration = task.duration_ms ? formatDuration(task.duration_ms) : '—';
        const time = task.created_at ? formatRelativeTime(task.created_at) : '';

        return `
            <div class="timeline-item animate-in">
                <div class="timeline-status ${statusClass}">${statusIcon}</div>
                <div class="timeline-body">
                    <div class="timeline-desc">${escapeHtml(task.description)}</div>
                    <div class="timeline-meta">
                        <span>${steps} steps</span>
                        <span>⏱ ${duration}</span>
                        <span>${time}</span>
                    </div>
                </div>
            </div>`;
    }).join('');
}

// =========================================================================
// Scores Tab
// =========================================================================
function renderScores(data) {
    const container = document.getElementById('scoresGrid');
    if (!container) return;

    if (!data || (!data.completeness && !data.efficiency && !data.quality)) {
        container.innerHTML = `
            <div class="empty-state" style="grid-column: 1 / -1">
                <div class="empty-icon">📊</div>
                No scores yet. Complete tasks to see performance metrics.
            </div>`;
        return;
    }

    const overall = data.overall || 0;
    const completeness = data.completeness || 0;
    const efficiency = data.efficiency || 0;
    const quality = data.quality || 0;

    container.innerHTML = `
        <div class="score-card score-overall">
            <div class="score-label">Overall</div>
            <div class="score-value ${getScoreClass(overall)}">${overall.toFixed(1)}</div>
            <div class="score-bar-track">
                <div class="score-bar-fill ${getScoreClass(overall)}" style="width: ${overall * 10}%"></div>
            </div>
            <div class="score-feedback">${getScoreFeedback(overall)}</div>
        </div>
        ${renderScoreCard('Completeness', completeness)}
        ${renderScoreCard('Efficiency', efficiency)}
        ${renderScoreCard('Quality', quality)}
    `;
}

function renderScoreCard(label, value) {
    return `
        <div class="score-card">
            <div class="score-label">${label}</div>
            <div class="score-value ${getScoreClass(value)}">${value.toFixed(1)}</div>
            <div class="score-bar-track">
                <div class="score-bar-fill ${getScoreClass(value)}" style="width: ${value * 10}%"></div>
            </div>
        </div>`;
}

function getScoreClass(score) {
    if (score >= 8) return 'high';
    if (score >= 6) return 'mid';
    if (score >= 4) return 'low';
    return 'danger';
}

function getScoreFeedback(score) {
    if (score >= 9) return '🏆 Exceptional';
    if (score >= 8) return '✨ Great';
    if (score >= 7) return '👍 Good';
    if (score >= 5) return '📈 Improving';
    return '🔧 Needs work';
}

// =========================================================================
// Strategies Tab
// =========================================================================
function renderStrategies(strategies) {
    const container = document.getElementById('strategyList');
    if (!container) return;

    if (!strategies || strategies.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">🎯</div>
                No strategies learned yet. The agent learns from completed tasks.
            </div>`;
        return;
    }

    container.innerHTML = strategies.map((s, i) => {
        const scoreClass = getScoreClass(s.avg_score || 0);
        return `
            <div class="strategy-item animate-in">
                <div class="strategy-rank">#${i + 1}</div>
                <div class="strategy-body">
                    <div class="strategy-type">${escapeHtml(s.task_type || 'General')}</div>
                    <div class="strategy-stats">
                        <span>Used ${s.use_count || 0}×</span>
                        <span>Win rate ${((s.success_rate || 0) * 100).toFixed(0)}%</span>
                    </div>
                </div>
                <div class="strategy-score ${scoreClass}">${(s.avg_score || 0).toFixed(1)}</div>
            </div>`;
    }).join('');
}

// =========================================================================
// Lessons Tab
// =========================================================================
function renderLessons(lessons) {
    const container = document.getElementById('lessonList');
    const filterContainer = document.getElementById('lessonFilters');
    if (!container) return;

    if (!lessons || lessons.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📚</div>
                No lessons extracted yet. Complete tasks and lessons will appear here.
            </div>`;
        if (filterContainer) filterContainer.innerHTML = '';
        return;
    }

    // Extract unique categories
    const categories = [...new Set(lessons.map(l => l.category || 'general'))];

    // Render filter buttons
    if (filterContainer) {
        filterContainer.innerHTML = `
            <button class="filter-btn ${lessonFilter === 'all' ? 'active' : ''}" data-filter="all">All</button>
            ${categories.map(cat => `
                <button class="filter-btn ${lessonFilter === cat ? 'active' : ''}" data-filter="${escapeHtml(cat)}">${escapeHtml(cat)}</button>
            `).join('')}
        `;

        filterContainer.querySelectorAll('.filter-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                lessonFilter = btn.dataset.filter;
                renderLessons(lessons);
            });
        });
    }

    // Filter lessons
    const filtered = lessonFilter === 'all'
        ? lessons
        : lessons.filter(l => (l.category || 'general') === lessonFilter);

    container.innerHTML = filtered.map(l => {
        const catClass = getCategoryClass(l.category);
        return `
            <div class="lesson-card animate-in">
                <span class="lesson-category ${catClass}">${escapeHtml(l.category || 'general')}</span>
                <div class="lesson-pattern">${escapeHtml(l.pattern || '')}</div>
                <div class="lesson-text">${escapeHtml(l.lesson || '')}</div>
            </div>`;
    }).join('');
}

function getCategoryClass(category) {
    if (!category) return 'default';
    const lower = category.toLowerCase();
    if (lower.includes('error') || lower.includes('bug')) return 'error';
    if (lower.includes('perf')) return 'performance';
    if (lower.includes('pattern') || lower.includes('style')) return 'pattern';
    return 'default';
}

// =========================================================================
// Utilities
// =========================================================================
function escapeHtml(str) {
    if (!str) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

function formatDuration(ms) {
    if (ms < 1000) return `${ms}ms`;
    if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
    return `${(ms / 60000).toFixed(1)}m`;
}

function formatRelativeTime(unixTimestamp) {
    const now = Date.now();
    const ts = unixTimestamp * 1000; // Convert seconds to ms
    const diff = now - ts;

    if (diff < 60000) return 'just now';
    if (diff < 3600000) return `${Math.floor(diff / 60000)}m ago`;
    if (diff < 86400000) return `${Math.floor(diff / 3600000)}h ago`;
    return `${Math.floor(diff / 86400000)}d ago`;
}
