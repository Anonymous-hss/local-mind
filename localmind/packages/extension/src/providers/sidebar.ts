import * as vscode from 'vscode';
import { IPCClient } from '../core/client';
import { createAgentRequest, isResultResponse, ResultResponse, AgentPayload } from '../core/protocol';

export class AgentSidebarProvider implements vscode.WebviewViewProvider {
    public static readonly viewType = 'localmind.agentView';
    private _view?: vscode.WebviewView;

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
        context: vscode.WebviewViewResolveContext,
        _token: vscode.CancellationToken,
    ) {
        this._view = webviewView;

        webviewView.webview.options = {
            enableScripts: true,
            localResourceRoots: [
                this._extensionUri
            ]
        };

        webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);

        webviewView.webview.onDidReceiveMessage(async (data) => {
            switch (data.type) {
                case 'command':
                    await this.handleCommand(data.text);
                    break;
                case 'approve':
                    await this.handleApprove();
                    break;
                case 'reject':
                    await this.handleReject();
                    break;
                case 'abort':
                    await this.handleAbort();
                    break;
            }
        });

        // Initialize view
        setTimeout(() => this.checkStatus(), 1000);
    }

    private async checkStatus() {
        if (this._view && this._client) {
            const isConnected = await this._client.ping();
            this._view.webview.postMessage({
                type: 'status',
                status: isConnected ? 'online' : 'offline'
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
            }));

            if (isResultResponse(response)) {
                const plan = JSON.parse(response.payload.content);
                this._view?.webview.postMessage({ type: 'updatePlan', plan });
                this.log(`Plan generated with ${plan.steps.length} steps`, 'success');
            }
        } catch (e: any) {
            this.log(`Error: ${e.message}`, 'error');
        }
    }

    private async handleApprove() {
        // Implementation for approve all / next step
        this.log('Approved step', 'success');
        // Logic to send 'approve' action to core
    }

    private async handleReject() {
        this.log('Rejected step', 'warning');
        // Logic to send 'reject' action to core
    }

    private async handleAbort() {
        this.log('Aborting task...', 'error');
        // Logic to send 'cancel' or 'rollback' to core
    }

    private log(text: string, level: string = 'info') {
        this._view?.webview.postMessage({ type: 'log', text, level });
    }

    private _getHtmlForWebview(webview: vscode.Webview) {
        const styleUri = webview.asWebviewUri(vscode.Uri.joinPath(this._extensionUri, 'media', 'sidebar.css'));
        const scriptUri = webview.asWebviewUri(vscode.Uri.joinPath(this._extensionUri, 'media', 'sidebar.js'));

        return `<!DOCTYPE html>
            <html lang="en">
            <head>
                <meta charset="UTF-8">
                <meta name="viewport" content="width=device-width, initial-scale=1.0">
                <link href="${styleUri}" rel="stylesheet">
                <title>LocalMind Agent</title>
            </head>
            <body>
                <div class="header">
                    <div class="header-top">
                        <span class="brand">LocalMind</span>
                        <div class="status-badge">
                            <div class="status-dot" id="statusDot"></div>
                            <span id="statusText">CONNECTING...</span>
                        </div>
                    </div>
                    <div class="model-info">
                        Running: Llama 3 (8B) | 42ms
                    </div>
                </div>

                <div class="content">
                    <section>
                        <h3>Execution Plan</h3>
                        <ul class="plan-list" id="planList">
                            <li class="plan-item">
                                <span class="checkbox">[ ]</span>
                                <span>Waiting for command...</span>
                            </li>
                        </ul>
                    </section>

                    <section class="risk-alert" style="display: none;" id="riskAlert">
                        <svg class="risk-icon" viewBox="0 0 16 16" width="16" height="16" fill="currentColor">
                            <path d="M8 1L1 14h14L8 1zm0 2.5l5 9H3l5-9z M7 10h2v2H7v-2zm0-5h2v4H7V5z"/>
                        </svg>
                        <div class="risk-content">
                            <strong>High Risk Action</strong>
                            <p>File deletion detected.</p>
                        </div>
                    </section>

                    <section>
                        <h3>System Log</h3>
                        <div class="log-container" id="logContainer">
                            <div class="log-entry">
                                <span class="log-time">System ~</span>
                                <span class="log-msg">Ready.</span>
                            </div>
                        </div>
                    </section>
                </div>

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
                </div>
                <script src="${scriptUri}"></script>
            </body>
            </html>`;
    }
}
