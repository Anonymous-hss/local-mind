/**
 * Agent Provider
 * 
 * Manages agent task operations including:
 * - Task planning
 * - Step approval/rejection
 * - Execution
 * - Rollback
 */

import * as vscode from 'vscode';
import { IPCClient } from '../core/client';
import { createAgentRequest, AgentPayload, isResultResponse, ResultResponse } from '../core/protocol';

// =============================================================================
// Types
// =============================================================================

export interface AgentTask {
    id: string;
    description: string;
    status: string;
    files: string[];
    steps: AgentStep[];
    currentStep: number;
    actionLog: AgentLogEntry[];
}

export interface AgentStep {
    id: string;
    index: number;
    type: string;
    description: string;
    status: string;
    file?: string;
    before?: string;
    after?: string;
    diff?: string;
    requiresApproval: boolean;
    approved?: boolean;
}

export interface AgentLogEntry {
    timestamp: number;
    action: string;
    stepId?: string;
    file?: string;
    details?: string;
    success: boolean;
}

// =============================================================================
// Agent Provider
// =============================================================================

export class AgentProvider {
    private client: IPCClient;
    private outputChannel: vscode.OutputChannel;
    private currentTask: AgentTask | null = null;

    constructor(client: IPCClient) {
        this.client = client;
        this.outputChannel = vscode.window.createOutputChannel('LocalMind Agent');
    }

    /**
     * Send an agent request
     */
    private async sendAgentRequest(payload: AgentPayload): Promise<string | null> {
        const request = createAgentRequest(payload);
        const response = await this.client.send<ResultResponse>(request);

        if (isResultResponse(response)) {
            return response.payload.content;
        }
        return null;
    }

    /**
     * Plan a new agent task
     */
    async planTask(description: string): Promise<AgentTask | null> {
        const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
        if (!workspaceRoot) {
            vscode.window.showErrorMessage('LocalMind Agent: No workspace folder open');
            return null;
        }

        this.outputChannel.appendLine(`Planning task: ${description}`);
        this.outputChannel.show();

        try {
            // Read user settings
            const config = vscode.workspace.getConfiguration('localmind');
            const model = config.get<string>('model', 'llama3');
            const contextBudget = config.get<number>('contextBudget', 12000);
            const maxSteps = config.get<number>('maxSteps', 20);

            // Get cursor position from active editor
            const editor = vscode.window.activeTextEditor;
            let cursorFile: string | undefined;
            let cursorLine: number | undefined;
            if (editor) {
                cursorFile = vscode.workspace.asRelativePath(editor.document.uri);
                cursorLine = editor.selection.active.line + 1; // 1-indexed
            }

            const response = await this.sendAgentRequest({
                action: 'plan',
                task: description,
                workspaceRoot,
                model,
                contextBudget,
                maxSteps,
                cursorFile,
                cursorLine,
            });

            if (!response) {
                throw new Error('No response from agent');
            }

            this.currentTask = JSON.parse(response) as AgentTask;
            this.outputChannel.appendLine(`Task planned with ${this.currentTask.steps.length} steps`);

            return this.currentTask;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.outputChannel.appendLine(`Error: ${message}`);
            vscode.window.showErrorMessage(`LocalMind Agent: ${message}`);
            return null;
        }
    }

    /**
     * Approve a step for execution
     */
    async approveStep(stepId: string): Promise<boolean> {
        if (!this.currentTask) {
            vscode.window.showErrorMessage('LocalMind Agent: No active task');
            return false;
        }

        this.outputChannel.appendLine(`Approving step: ${stepId}`);

        try {
            await this.sendAgentRequest({
                action: 'approve',
                taskId: this.currentTask.id,
                stepId,
            });

            const step = this.currentTask.steps.find(s => s.id === stepId);
            if (step) {
                step.approved = true;
            }

            this.outputChannel.appendLine(`Step ${stepId} approved`);
            return true;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.outputChannel.appendLine(`Error: ${message}`);
            vscode.window.showErrorMessage(`LocalMind Agent: ${message}`);
            return false;
        }
    }

    /**
     * Reject a step
     */
    async rejectStep(stepId: string, reason?: string): Promise<boolean> {
        if (!this.currentTask) {
            vscode.window.showErrorMessage('LocalMind Agent: No active task');
            return false;
        }

        this.outputChannel.appendLine(`Rejecting step: ${stepId}`);

        try {
            await this.sendAgentRequest({
                action: 'reject',
                taskId: this.currentTask.id,
                stepId,
                reason: reason || 'User rejected',
            });

            const step = this.currentTask.steps.find(s => s.id === stepId);
            if (step) {
                step.approved = false;
                step.status = 'skipped';
            }

            this.outputChannel.appendLine(`Step ${stepId} rejected`);
            return true;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.outputChannel.appendLine(`Error: ${message}`);
            vscode.window.showErrorMessage(`LocalMind Agent: ${message}`);
            return false;
        }
    }

    /**
     * Execute all approved steps
     */
    async executeTask(): Promise<AgentTask | null> {
        if (!this.currentTask) {
            vscode.window.showErrorMessage('LocalMind Agent: No active task');
            return null;
        }

        this.outputChannel.appendLine(`Executing task: ${this.currentTask.id}`);

        try {
            const response = await this.sendAgentRequest({
                action: 'execute',
                taskId: this.currentTask.id,
            });

            if (response) {
                this.currentTask = JSON.parse(response) as AgentTask;
                this.outputChannel.appendLine(`Task execution complete: ${this.currentTask.status}`);
            }

            return this.currentTask;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.outputChannel.appendLine(`Error: ${message}`);
            vscode.window.showErrorMessage(`LocalMind Agent: ${message}`);
            return null;
        }
    }

    /**
     * Rollback task to last checkpoint
     */
    async rollbackTask(): Promise<boolean> {
        if (!this.currentTask) {
            vscode.window.showErrorMessage('LocalMind Agent: No active task');
            return false;
        }

        this.outputChannel.appendLine(`Rolling back task: ${this.currentTask.id}`);

        try {
            await this.sendAgentRequest({
                action: 'rollback',
                taskId: this.currentTask.id,
            });

            this.outputChannel.appendLine('Rollback complete');
            this.currentTask.status = 'rolled_back';
            return true;
        } catch (error) {
            const message = error instanceof Error ? error.message : String(error);
            this.outputChannel.appendLine(`Error: ${message}`);
            vscode.window.showErrorMessage(`LocalMind Agent: ${message}`);
            return false;
        }
    }

    /**
     * Get task status
     */
    async getStatus(): Promise<AgentTask | null> {
        if (!this.currentTask) {
            return null;
        }

        try {
            const response = await this.sendAgentRequest({
                action: 'status',
                taskId: this.currentTask.id,
            });

            if (response) {
                const summary = JSON.parse(response);
                this.outputChannel.appendLine(`Task status: ${JSON.stringify(summary)}`);
            }

            return this.currentTask;
        } catch (error) {
            return this.currentTask;
        }
    }

    /**
     * Show diff preview for a step
     */
    async showDiffPreview(stepId: string): Promise<void> {
        if (!this.currentTask) return;

        const step = this.currentTask.steps.find(s => s.id === stepId);
        if (!step || !step.before || !step.after) {
            vscode.window.showInformationMessage('No diff available for this step');
            return;
        }

        const beforeUri = vscode.Uri.parse(`localmind-before:${step.file}`);
        const afterUri = vscode.Uri.parse(`localmind-after:${step.file}`);

        await vscode.commands.executeCommand('vscode.diff', beforeUri, afterUri, `${step.file} (Preview)`);
    }

    /**
     * Get current task
     */
    getCurrentTask(): AgentTask | null {
        return this.currentTask;
    }

    /**
     * Clear current task
     */
    clearTask(): void {
        this.currentTask = null;
    }

    dispose(): void {
        this.outputChannel.dispose();
    }
}

// =============================================================================
// Command Registration
// =============================================================================

export function registerAgentCommands(
    context: vscode.ExtensionContext,
    client: IPCClient
): AgentProvider {
    const provider = new AgentProvider(client);

    // Plan Task command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.agentPlanTask', async () => {
            const description = await vscode.window.showInputBox({
                prompt: 'Describe the task you want the agent to perform',
                placeHolder: 'e.g., Add error handling to all API endpoints',
            });

            if (description) {
                const task = await provider.planTask(description);
                if (task) {
                    showTaskPlan(task);
                }
            }
        })
    );

    // Execute Task command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.agentExecuteTask', async () => {
            const task = provider.getCurrentTask();
            if (!task) {
                vscode.window.showErrorMessage('No active task. Run "LocalMind: Plan Task" first.');
                return;
            }

            const unapproved = task.steps.filter(s =>
                s.requiresApproval && s.approved === undefined
            );

            if (unapproved.length > 0) {
                const action = await vscode.window.showWarningMessage(
                    `${unapproved.length} steps need approval before execution.`,
                    'Approve All',
                    'Review Steps',
                    'Cancel'
                );

                if (action === 'Approve All') {
                    for (const step of unapproved) {
                        await provider.approveStep(step.id);
                    }
                } else if (action === 'Review Steps') {
                    showTaskPlan(task);
                    return;
                } else {
                    return;
                }
            }

            await provider.executeTask();
        })
    );

    // Rollback Task command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.agentRollback', async () => {
            const confirm = await vscode.window.showWarningMessage(
                'Are you sure you want to rollback all changes?',
                'Yes, Rollback',
                'Cancel'
            );

            if (confirm === 'Yes, Rollback') {
                await provider.rollbackTask();
            }
        })
    );

    // Show Task Status command
    context.subscriptions.push(
        vscode.commands.registerCommand('localmind.agentStatus', async () => {
            const task = provider.getCurrentTask();
            if (!task) {
                vscode.window.showInformationMessage('No active task');
                return;
            }

            showTaskPlan(task);
        })
    );

    context.subscriptions.push({ dispose: () => provider.dispose() });

    return provider;
}

// =============================================================================
// UI Helpers
// =============================================================================

async function showTaskPlan(task: AgentTask): Promise<void> {
    const items: vscode.QuickPickItem[] = [
        {
            label: '$(info) Task',
            description: task.description,
            detail: `Status: ${task.status} | Steps: ${task.steps.length}`,
        },
        { label: '', kind: vscode.QuickPickItemKind.Separator },
        ...task.steps.map((step, i) => ({
            label: getStepIcon(step) + ` Step ${i + 1}: ${step.type}`,
            description: step.file || '',
            detail: step.description,
        })),
    ];

    if (task.status !== 'completed' && task.status !== 'rolled_back') {
        items.push(
            { label: '', kind: vscode.QuickPickItemKind.Separator },
            { label: '$(play) Execute All Approved Steps', description: 'Execute' },
            { label: '$(discard) Rollback All Changes', description: 'Rollback' },
        );
    }

    const selected = await vscode.window.showQuickPick(items, {
        title: 'LocalMind Agent Task',
        placeHolder: 'Select a step to view details or an action to perform',
    });

    if (selected?.description === 'Execute') {
        vscode.commands.executeCommand('localmind.agentExecuteTask');
    } else if (selected?.description === 'Rollback') {
        vscode.commands.executeCommand('localmind.agentRollback');
    }
}

function getStepIcon(step: AgentStep): string {
    if (step.status === 'completed') return '$(check)';
    if (step.status === 'failed') return '$(error)';
    if (step.status === 'skipped') return '$(circle-slash)';
    if (step.approved === true) return '$(pass)';
    if (step.approved === false) return '$(close)';
    if (step.requiresApproval) return '$(question)';
    return '$(circle-outline)';
}
