/**
 * LocalMind VS Code Extension
 * 
 * Entry point for the extension. Manages:
 * - Core engine process lifecycle
 * - IPC communication via STDIO
 * - Inline completion provider registration
 * - Status bar and commands
 */

import * as vscode from 'vscode';
import { IPCClient } from './core/client';
import { PongPayload } from './core/protocol';
import { registerCompletionProvider, LocalMindCompletionProvider } from './providers/completion';
import { registerAgentCommands, AgentProvider } from './providers/agent';
import { DashboardProvider } from './providers/dashboard';
import { ActionCodeLensProvider, suggestRefactorCommand } from './providers/codelens';
import { SuggestionContentProvider } from './providers/content';

// =============================================================================
// Extension State
// =============================================================================

let client: IPCClient | null = null;
let completionProvider: LocalMindCompletionProvider | null = null;
let agentProvider: AgentProvider | null = null;
let statusBarItem: vscode.StatusBarItem | null = null;
let lastPongData: PongPayload | null = null;

// =============================================================================
// Activation
// =============================================================================

export async function activate(context: vscode.ExtensionContext): Promise<void> {
    console.log('LocalMind extension activating...');

    // Create status bar item
    statusBarItem = vscode.window.createStatusBarItem(
        vscode.StatusBarAlignment.Right,
        100
    );
    statusBarItem.text = '$(loading~spin) LocalMind';
    statusBarItem.tooltip = 'LocalMind: Starting...';
    statusBarItem.command = 'localmind.showStatus';
    statusBarItem.show();
    context.subscriptions.push(statusBarItem);

    // Create IPC client
    const debug = vscode.workspace.getConfiguration('localmind').get('debug', false);
    client = new IPCClient(context, {
        debug,
        extensionVersion: context.extension.packageJSON.version || '0.1.0',
    });

    // Register commands
    registerCommands(context);

    // Try to start the core engine
    try {
        await client.start();
        updateStatusBar('ready');

        // Register completion provider
        completionProvider = registerCompletionProvider(context, client);

        // Register agent commands
        agentProvider = registerAgentCommands(context, client);

        // Register operator dashboard (unified sidebar)
        const dashboardProvider = new DashboardProvider(context.extensionUri, client);
        context.subscriptions.push(
            vscode.window.registerWebviewViewProvider(
                DashboardProvider.viewType,
                dashboardProvider
            )
        );

        // Register suggestion content provider
        const contentProvider = new SuggestionContentProvider();
        context.subscriptions.push(
            vscode.workspace.registerTextDocumentContentProvider(
                SuggestionContentProvider.scheme,
                contentProvider
            )
        );

        // Register code lens provider
        context.subscriptions.push(
            vscode.languages.registerCodeLensProvider(
                { scheme: 'file' },
                new ActionCodeLensProvider(client)
            )
        );

        // Register refactor command
        context.subscriptions.push(
            vscode.commands.registerCommand('localmind.suggestRefactor', (document: vscode.TextDocument, range: vscode.Range, name: string) => {
                suggestRefactorCommand(contentProvider, client!, document, range, name);
            })
        );

        // Removed memory dashboard auto-refresh

        // Run onboarding health check on first activation
        checkOllamaHealth();

        vscode.window.showInformationMessage('LocalMind: Ready for completions');
    } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        console.error('LocalMind failed to start:', message);
        updateStatusBar('error');

        // Detect specific failure types and show actionable messages
        const isOllamaIssue = message.toLowerCase().includes('ollama') ||
            message.toLowerCase().includes('connection refused') ||
            message.toLowerCase().includes('econnrefused');

        if (isOllamaIssue) {
            const action = await vscode.window.showErrorMessage(
                'LocalMind: Ollama is not running. Start Ollama to enable AI features.',
                'Retry',
                'Install Ollama',
                'Open Settings'
            );
            if (action === 'Retry') {
                vscode.commands.executeCommand('localmind.restart');
            } else if (action === 'Install Ollama') {
                vscode.env.openExternal(vscode.Uri.parse('https://ollama.ai/download'));
            } else if (action === 'Open Settings') {
                vscode.commands.executeCommand('workbench.action.openSettings', 'localmind');
            }
        } else {
            const action = await vscode.window.showErrorMessage(
                `LocalMind: Failed to start. ${message}`,
                'Retry',
                'View Logs',
                'Open Settings'
            );
            if (action === 'Retry') {
                vscode.commands.executeCommand('localmind.restart');
            } else if (action === 'View Logs') {
                vscode.commands.executeCommand('localmind.showLogs');
            } else if (action === 'Open Settings') {
                vscode.commands.executeCommand('workbench.action.openSettings', 'localmind');
            }
        }
    }
}

// =============================================================================
// Deactivation
// =============================================================================

export async function deactivate(): Promise<void> {
    console.log('LocalMind extension deactivating...');

    if (client) {
        await client.stop();
        client = null;
    }

    completionProvider = null;
}

// =============================================================================
// Commands
// =============================================================================

function registerCommands(context: vscode.ExtensionContext): void {
    // Show status command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.showStatus', async () => {
            if (!client) {
                vscode.window.showInformationMessage('LocalMind: Not initialized');
                return;
            }

            const pong = await client.pingFull();
            if (!pong) {
                vscode.window.showErrorMessage('LocalMind: Core engine not responding');
                return;
            }

            // Build detailed status message
            const lines: string[] = [];
            lines.push(`Core Engine: v${pong.version || 'unknown'}`);
            lines.push(`Ollama: ${pong.ollamaStatus || 'unknown'}`);

            if (pong.models && pong.models.length > 0) {
                lines.push(``);
                lines.push(`Models (${pong.models.length}):`);
                for (const m of pong.models) {
                    const sizeMB = (m.size / (1024 * 1024)).toFixed(0);
                    const roleTag = m.role ? ` [${m.role}]` : '';
                    lines.push(`  • ${m.name} (${sizeMB} MB)${roleTag}`);
                }
            } else {
                lines.push(``);
                lines.push(`No models found. Pull one with: ollama pull qwen2.5-coder:1.5b`);
            }

            if (pong.activeModels && Object.keys(pong.activeModels).length > 0) {
                lines.push(``);
                lines.push(`Role Assignments:`);
                for (const [role, model] of Object.entries(pong.activeModels)) {
                    lines.push(`  ${role} → ${model}`);
                }
            }

            vscode.window.showInformationMessage(lines.join('\n'), { modal: true });
        })
    );

    // Restart command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.restart', async () => {
            if (!client) {
                vscode.window.showErrorMessage('LocalMind: Not initialized');
                return;
            }

            updateStatusBar('starting');

            try {
                await client.stop();
                await client.start();
                updateStatusBar('ready');
                vscode.window.showInformationMessage('LocalMind: Restarted successfully');
            } catch (error) {
                const message = error instanceof Error ? error.message : String(error);
                updateStatusBar('error');
                vscode.window.showErrorMessage(`LocalMind: Restart failed. ${message}`);
            }
        })
    );

    // Toggle completions command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.toggleCompletions', () => {
            if (!completionProvider) {
                vscode.window.showErrorMessage('LocalMind: Completion provider not available');
                return;
            }

            // Toggle would need a state variable; for now just show status
            vscode.window.showInformationMessage('LocalMind: Completions toggled');
        })
    );

    // Show logs command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.showLogs', () => {
            if (client) {
                client.showLogs();
            } else {
                vscode.window.showErrorMessage('LocalMind: Not initialized');
            }
        })
    );

    // Ping command (for testing)
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.ping', async () => {
            if (!client) {
                vscode.window.showErrorMessage('LocalMind: Not initialized');
                return;
            }

            const start = Date.now();
            const success = await client.ping();
            const latency = Date.now() - start;

            if (success) {
                vscode.window.showInformationMessage(`LocalMind: Pong! (${latency}ms)`);
            } else {
                vscode.window.showErrorMessage('LocalMind: Ping failed');
            }
        })
    );

    // Open Dashboard command removed (premium feature)
}

// =============================================================================
// Status Bar
// =============================================================================

type StatusBarState = 'starting' | 'ready' | 'error' | 'disabled';

function updateStatusBar(state: StatusBarState, modelName?: string): void {
    if (!statusBarItem) return;

    const modelSuffix = modelName ? ` • ${modelName}` : '';

    switch (state) {
        case 'starting':
            statusBarItem.text = '$(loading~spin) LocalMind';
            statusBarItem.tooltip = 'LocalMind: Starting core engine...';
            statusBarItem.backgroundColor = undefined;
            break;
        case 'ready':
            statusBarItem.text = `$(check) LocalMind${modelSuffix}`;
            statusBarItem.tooltip = modelName
                ? `LocalMind: Ready — Model: ${modelName}\nClick for details`
                : 'LocalMind: Ready\nClick for details';
            statusBarItem.backgroundColor = undefined;
            break;
        case 'error':
            statusBarItem.text = '$(error) LocalMind';
            statusBarItem.tooltip = 'LocalMind: Error - Click to retry';
            statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
            break;
        case 'disabled':
            statusBarItem.text = '$(circle-slash) LocalMind';
            statusBarItem.tooltip = 'LocalMind: Disabled';
            statusBarItem.backgroundColor = undefined;
            break;
    }
}

// =============================================================================
// Onboarding & Health Check
// =============================================================================

async function checkOllamaHealth(): Promise<void> {
    if (!client) return;

    try {
        const pong = await client.pingFull();
        if (!pong) {
            updateStatusBar('error');
            vscode.window.showWarningMessage(
                'LocalMind: Core engine not responding. Try restarting.',
                'Restart'
            ).then(action => {
                if (action === 'Restart') {
                    vscode.commands.executeCommand('localmind.restart');
                }
            });
            return;
        }

        lastPongData = pong;

        // Check Ollama status
        if (pong.ollamaStatus === 'disconnected') {
            updateStatusBar('ready');
            if (statusBarItem) {
                statusBarItem.text = '$(warning) LocalMind • Ollama Offline';
                statusBarItem.tooltip = 'LocalMind: Ollama not running. Start with: ollama serve';
                statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
            }
            vscode.window.showWarningMessage(
                'LocalMind: Ollama is not running. AI features are disabled. Start Ollama to enable.',
                'Start Ollama', 'Install Ollama'
            ).then(action => {
                if (action === 'Install Ollama') {
                    vscode.env.openExternal(vscode.Uri.parse('https://ollama.ai/download'));
                } else if (action === 'Start Ollama') {
                    const terminal = vscode.window.createTerminal('LocalMind: Ollama');
                    terminal.show();
                    terminal.sendText('ollama serve');
                }
            });
            return;
        }

        // Check models
        if (!pong.models || pong.models.length === 0) {
            updateStatusBar('ready');
            if (statusBarItem) {
                statusBarItem.text = '$(warning) LocalMind • No Models';
                statusBarItem.tooltip = 'LocalMind: No Ollama models found. Pull one to get started.';
                statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
            }
            vscode.window.showWarningMessage(
                'LocalMind: No models found in Ollama. Pull a model to get started.',
                'Pull qwen2.5-coder:1.5b'
            ).then(action => {
                if (action) {
                    pullModel('qwen2.5-coder:1.5b');
                }
            });
            return;
        }

        // Everything is good — show active model in status bar
        const completionModel = pong.activeModels?.completion || pong.models[0]?.name || 'unknown';
        updateStatusBar('ready', completionModel);

        // Log role assignments
        if (pong.activeModels) {
            const roles = Object.entries(pong.activeModels)
                .map(([role, model]) => `${role}→${model}`)
                .join(', ');
            console.log(`LocalMind: Model roles: ${roles}`);
        }
    } catch {
        // Pong failed — core engine issue, already shown error in activation
        console.log('LocalMind: Health check failed (core engine not responding)');
    }
}

function pullModel(model: string): void {
    const terminal = vscode.window.createTerminal('LocalMind: Pull Model');
    terminal.show();
    terminal.sendText(`ollama pull ${model}`);
    vscode.window.showInformationMessage(
        `LocalMind: Pulling model "${model}"... Check the terminal for progress.`
    );
}
