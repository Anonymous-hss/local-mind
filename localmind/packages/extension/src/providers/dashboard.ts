/**
 * Operator Dashboard Provider
 * 
 * Unified WebView sidebar combining:
 * - Agent Command Center (plan/approve/reject/execute)
 * - Task History Timeline
 * - Score Visualizations
 * - Strategy Rankings
 * - Lesson Library
 */

import * as vscode from 'vscode';
import { IPCClient } from '../core/client';
import { createAgentRequest, isResultResponse, ResultResponse, AgentPayload } from '../core/protocol';

export class DashboardProvider implements vscode.WebviewViewProvider {
    public static readonly viewType = 'localmind.agentView';
    private _view?: vscode.WebviewView;
    private _refreshTimer?: ReturnType<typeof setInterval>;
    // Agent state: tracks the current planned task so approve/reject/execute work
    private _currentTaskId: string | null = null;
    private _currentStepId: string | null = null;

    constructor(
        private readonly _extensionUri: vscode.Uri,
        private readonly _client: IPCClient
    ) {
        // Forward streams to webview
        this._client.onStream(event => {
            this._view?.webview.postMessage({
                type: 'stream',
                chunk: event.chunk,
                requestId: event.requestId
            });
        });
    }

    public resolveWebviewView(
        webviewView: vscode.WebviewView,
        _context: vscode.WebviewViewResolveContext,
        _token: vscode.CancellationToken,
    ) {
        this._view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [this._extensionUri]
        };

        webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);

        // Handle messages from WebView
        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                // Agent commands
                case 'command': await this.handleCommand(data.text); break;
                case 'approve': await this.handleApprove(); break;
                case 'reject': await this.handleReject(); break;
                case 'abort': await this.handleAbort(); break;
                // Dashboard data requests
                // Premium data fetches removed (history, scores, strategies, lessons)
            }
        });

        // Check status on load
        setTimeout(() => this.checkStatus(), 1000);

        // Auto-refresh dashboard data (every 30s)
        this._refreshTimer = setInterval(() => this.checkStatus(), 30000);

        webviewView.onDidDispose(() => {
            if (this._refreshTimer) {
                clearInterval(this._refreshTimer);
            }
        });
    }

    // =====================================================================
    // Agent Commands
    // =====================================================================

    private async checkStatus() {
        if (this._view && this._client) {
            const pong = await (this._client as any).pingFull?.();
            const isConnected = pong !== null;
            const ollamaStatus = pong?.ollamaStatus || 'unknown';
            const models = pong?.models || [];
            const activeModels = pong?.activeModels || {};

            // Get the primary model name for display
            const completionModel = activeModels.completion || (models[0]?.name) || '';

            this._view.webview.postMessage({
                type: 'status',
                status: isConnected ? (ollamaStatus === 'connected' ? 'online' : 'ollama_offline') : 'offline',
                model: completionModel,
                ollamaStatus,
                modelCount: models.length,
                activeModels,
            });
        }
    }

    private async handleCommand(text: string) {
        this.log(`Planning task: ${text}`, 'info');

        const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
        if (!workspaceRoot) {
            this.log('Error: No workspace open', 'error');
            return;
        }

        try {
            const response = await this._client.send<ResultResponse>(createAgentRequest({
                action: 'plan',
                task: text,
                workspaceRoot
            }), 600000); // 10 minutes timeout for planning

            if (isResultResponse(response)) {
                let plan: any;

                // Core may return JSON plan or a plain-text message
                try {
                    plan = JSON.parse(response.payload.content);
                } catch {
                    this.log(response.payload.content, 'info');
                    return;
                }

                // Store the taskId so approve/reject/execute can reference it
                this._currentTaskId = plan.id || plan.taskId || null;
                this._currentStepId = plan.steps?.[0]?.id || null;

                // If stub (no real executor), show the steps but note it's a stub
                if (plan.status === 'stub') {
                    this.log('⚠ Agent not fully connected to Ollama. Showing stub plan.', 'warning');
                    this.log(`Workspace: ${plan.workspace || workspaceRoot}`, 'info');
                }

                this._view?.webview.postMessage({ type: 'updatePlan', plan });
                this.log(`Plan ready — click APPROVE to execute, REJECT to skip`, 'success');
            }
        } catch (e: any) {
            this.log(`Error: ${e.message}`, 'error');
        }
    }

    private async handleApprove() {
        if (!this._currentTaskId) {
            this.log('No active task to approve. Run a command first.', 'warning');
            return;
        }
        
        const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';

        // Safely approve pending step first
        if (this._currentStepId) {
            try {
                await this._client.send<ResultResponse>(createAgentRequest({
                    action: 'approve',
                    taskId: this._currentTaskId,
                    stepId: this._currentStepId,
                    workspaceRoot
                }), 60000); // 1 minute timeout
                this.log(`Approved step ${this._currentStepId}.`, 'success');
            } catch (e: any) {
                this.log(`Approval error: ${e.message}`, 'error');
                return;
            }
        }

        this.log('Executing task...', 'info');
        try {
            const response = await this._client.send<ResultResponse>(createAgentRequest({
                action: 'execute',
                taskId: this._currentTaskId,
                workspaceRoot
            }), 600000); // 10 minutes timeout for execution
            if (isResultResponse(response)) {
                this.log('Task executed successfully.', 'success');
                this._currentTaskId = null;
            }
        } catch (e: any) {
            this.log(`Execute error: ${e.message}`, 'error');
        }
    }

    private async handleReject() {
        if (!this._currentTaskId) {
            this.log('No active task to reject.', 'warning');
            return;
        }
        const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath || '';
        try {
            await this._client.send<ResultResponse>(createAgentRequest({
                action: 'reject',
                taskId: this._currentTaskId,
                stepId: this._currentStepId || undefined,
                workspaceRoot
            }));
            this.log('Step rejected.', 'warning');
            this._currentTaskId = null;
        } catch (e: any) {
            this.log(`Reject error: ${e.message}`, 'error');
        }
    }

    private async handleAbort() {
        this.log('Task aborted.', 'error');
        this._currentTaskId = null;
        this._currentStepId = null;
    }

    // =====================================================================
    // Dashboard Data Fetchers
    // =====================================================================

    // Dashboard Data Fetchers removed (Premium Features)

    // =====================================================================
    // Helpers
    // =====================================================================

    private log(text: string, level: string = 'info') {
        this._view?.webview.postMessage({ type: 'log', text, level });
    }

    // =====================================================================
    // HTML Template
    // =====================================================================

    private _getHtmlForWebview(webview: vscode.Webview) {
        const styleUri = webview.asWebviewUri(vscode.Uri.joinPath(this._extensionUri, 'media', 'dashboard.css'));
        const scriptUri = webview.asWebviewUri(vscode.Uri.joinPath(this._extensionUri, 'media', 'dashboard.js'));

        return `<!DOCTYPE html>
            <html lang="en">
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <link href="${styleUri}" rel="stylesheet">
                <title>LocalMind Dashboard</title>
            </head>
            <body>
                <!-- Header -->
                <div class="header">
                    <div class="header-top">
                        <span class="brand">
                            <span class="brand-icon">⚡</span>
                            LocalMind
                        </span>
                        <div class="status-badge">
                            <div class="status-dot" id="statusDot"></div>
                            <span id="statusText">CONNECTING</span>
                        </div>
                    </div>
                    <div class="model-info" id="modelInfo">Ollama • Local-First Agent</div>
                </div>

                <!-- Tab Bar -->
                <div class="tab-bar">
                    <button class="tab-btn active" data-tab="agent">Agent</button>
                    <button class="tab-btn" style="opacity: 0.5; cursor: not-allowed;" title="Premium Feature">History 🔒</button>
                    <button class="tab-btn" style="opacity: 0.5; cursor: not-allowed;" title="Premium Feature">Scores 🔒</button>
                    <button class="tab-btn" style="opacity: 0.5; cursor: not-allowed;" title="Premium Feature">Strategy 🔒</button>
                    <button class="tab-btn" style="opacity: 0.5; cursor: not-allowed;" title="Premium Feature">Lessons 🔒</button>
                </div>

                <!-- ===== AGENT TAB ===== -->
                <div class="tab-panel active" id="panel-agent">
                    <div class="card">
                        <div class="card-header">
                            <span class="card-title">Execution Plan</span>
                        </div>
                        <ul class="plan-list" id="planList">
                            <li class="plan-item">
                                <span class="checkbox">[ ]</span>
                                <span>Awaiting command...</span>
                            </li>
                        </ul>
                    </div>

                    <div class="risk-alert" style="display: none;" id="riskAlert">
                        <div class="risk-content">
                            <strong>⚠ Risk Detected</strong>
                            <p id="riskText">—</p>
                        </div>
                    </div>

                    <div class="card">
                        <div class="card-header">
                            <span class="card-title">System Log</span>
                        </div>
                        <div class="log-container" id="logContainer">
                            <div class="log-entry">
                                <span class="log-time">sys</span>
                                <span class="log-msg info">Ready.</span>
                            </div>
                        </div>
                    </div>
                </div>

                <!-- ===== EXCLUDED TABS (PREMIUM) ===== -->
                <div class="tab-panel" id="panel-history" style="display:none;"></div>
                <div class="tab-panel" id="panel-scores" style="display:none;"></div>
                <div class="tab-panel" id="panel-strategies" style="display:none;"></div>
                <div class="tab-panel" id="panel-lessons" style="display:none;"></div>

                <!-- Footer — Command Input + Actions -->
                <div class="footer">
                    <div class="action-bar">
                        <button class="btn-action btn-approve" id="btnApprove">
                            <span>[y]</span> APPROVE
                        </button>
                        <button class="btn-action btn-reject" id="btnReject">
                            <span>[n]</span> REJECT
                        </button>
                        <button class="btn-action btn-abort" id="btnAbort">
                            ABORT
                        </button>
                    </div>

                    <div class="input-container">
                        <span class="prompt-symbol">&gt;</span>
                        <input type="text"
                               class="command-input"
                               id="commandInput"
                               placeholder="Refactor auth loop @login.go"
                               autocomplete="off">
                        <div class="input-line"></div>
                    </div>
                    <div class="ollama-footer" style="text-align: center; font-size: 10px; opacity: 0.6; margin-top: 12px; pointer-events: none;">
                        ⚡ Powered by Ollama
                    </div>
                </div>

                <script src="${scriptUri}"></script>
            </body>
            </html>`;
    }
}
