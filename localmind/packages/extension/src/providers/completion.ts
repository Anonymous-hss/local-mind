/**
 * LocalMind Inline Completion Provider
 * 
 * Provides AI-powered code completions via VS Code's inline completion API.
 * Communicates with the Go core engine through the IPC client.
 */

import * as vscode from 'vscode';
import { IPCClient } from '../core/client';
import {
    createCompletionRequest,
    ResultResponse,
    isResultResponse,
    isErrorResponse,
    MessageId,
} from '../core/protocol';

// =============================================================================
// Configuration
// =============================================================================

const DEBOUNCE_DELAY_MS = 150;  // Wait 150ms after typing stops
const COMPLETION_TIMEOUT_MS = 3500;  // Allow enough time for Ollama
const MAX_PREFIX_CHARS = 1500;  // Match Go engine limit
const MAX_SUFFIX_CHARS = 500;

// =============================================================================
// Completion Provider
// =============================================================================

export class LocalMindCompletionProvider implements vscode.InlineCompletionItemProvider {
    private debounceTimer: NodeJS.Timeout | null = null;
    private lastRequestId: MessageId | null = null;
    private enabled: boolean = true;

    constructor(private readonly client: IPCClient) { }

    /**
     * Enable or disable the completion provider
     */
    setEnabled(enabled: boolean): void {
        this.enabled = enabled;
    }

    /**
     * Provide inline completions for the current cursor position
     */
    async provideInlineCompletionItems(
        document: vscode.TextDocument,
        position: vscode.Position,
        context: vscode.InlineCompletionContext,
        token: vscode.CancellationToken
    ): Promise<vscode.InlineCompletionItem[] | undefined> {
        // Check if enabled
        if (!this.enabled) {
            return undefined;
        }

        // Check if client is ready
        if (!this.client.isReady()) {
            return undefined;
        }

        // Cancel any previous pending request
        if (this.lastRequestId) {
            this.client.cancel(this.lastRequestId);
            this.lastRequestId = null;
        }

        // Clear existing debounce timer
        if (this.debounceTimer) {
            clearTimeout(this.debounceTimer);
        }

        // Debounce: wait for user to stop typing
        return new Promise((resolve) => {
            this.debounceTimer = setTimeout(async () => {
                try {
                    const items = await this.fetchCompletions(document, position, token);
                    resolve(items);
                } catch (error) {
                    console.error('[LocalMind] Completion error:', error);
                    resolve(undefined);
                }
            }, DEBOUNCE_DELAY_MS);

            // Handle cancellation
            token.onCancellationRequested(() => {
                if (this.debounceTimer) {
                    clearTimeout(this.debounceTimer);
                    this.debounceTimer = null;
                }
                if (this.lastRequestId) {
                    this.client.cancel(this.lastRequestId);
                    this.lastRequestId = null;
                }
                resolve(undefined);
            });
        });
    }

    /**
     * Fetch completions from the core engine
     */
    private async fetchCompletions(
        document: vscode.TextDocument,
        position: vscode.Position,
        token: vscode.CancellationToken
    ): Promise<vscode.InlineCompletionItem[] | undefined> {
        // Check cancellation
        if (token.isCancellationRequested) {
            return undefined;
        }

        // Extract context
        const { prefix, suffix } = this.extractContext(document, position);

        // Skip if prefix is too short
        if (prefix.trim().length < 3) {
            return undefined;
        }

        // Get language ID
        const language = document.languageId;

        // Get relative file path
        const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
        const filePath = workspaceFolder
            ? vscode.workspace.asRelativePath(document.uri)
            : document.fileName;

        // Create and send request
        const request = createCompletionRequest(prefix, language, {
            suffix: suffix || undefined,
            filePath,
            maxTokens: 50,
        });

        this.lastRequestId = request.id;

        try {
            const response = await this.client.send(request, COMPLETION_TIMEOUT_MS);

            // Check if cancelled during request
            if (token.isCancellationRequested) {
                return undefined;
            }

            // Handle response
            if (isResultResponse(response)) {
                return this.createCompletionItems(response, position);
            } else if (isErrorResponse(response)) {
                // Log but don't show error to user (might be timeout/cancel)
                if (response.payload.code !== 'CANCELLED' && response.payload.code !== 'TIMEOUT') {
                    console.warn('[LocalMind] Completion error:', response.payload.message);
                }
                return undefined;
            } else if (response.type === 'cancelled') {
                return undefined;
            }

            return undefined;
        } catch (error) {
            // Timeouts and cancellations are expected, don't log them
            if (error instanceof Error &&
                !error.message.includes('timeout') &&
                !error.message.includes('Cancelled')) {
                console.error('[LocalMind] Request error:', error);
            }
            return undefined;
        } finally {
            this.lastRequestId = null;
        }
    }

    /**
     * Extract prefix and suffix from document around cursor
     */
    private extractContext(
        document: vscode.TextDocument,
        position: vscode.Position
    ): { prefix: string; suffix: string } {
        const offset = document.offsetAt(position);
        const text = document.getText();

        // Get prefix (text before cursor)
        const prefixStart = Math.max(0, offset - MAX_PREFIX_CHARS);
        const prefix = text.substring(prefixStart, offset);

        // Get suffix (text after cursor)
        const suffixEnd = Math.min(text.length, offset + MAX_SUFFIX_CHARS);
        const suffix = text.substring(offset, suffixEnd);

        return { prefix, suffix };
    }

    /**
     * Create VS Code completion items from response
     */
    private createCompletionItems(
        response: ResultResponse,
        position: vscode.Position
    ): vscode.InlineCompletionItem[] {
        const content = response.payload.content;

        // Skip empty or whitespace-only completions
        if (!content || content.trim().length === 0) {
            return [];
        }

        // Create the inline completion item
        const item = new vscode.InlineCompletionItem(
            content,
            new vscode.Range(position, position)
        );

        return [item];
    }
}

// =============================================================================
// Registration
// =============================================================================

/**
 * Register the completion provider for supported languages
 */
export function registerCompletionProvider(
    context: vscode.ExtensionContext,
    client: IPCClient
): LocalMindCompletionProvider {
    const provider = new LocalMindCompletionProvider(client);

    // Register for all supported languages
    const languages = [
        'go',
        'typescript',
        'typescriptreact',
        'javascript',
        'javascriptreact',
        'python',
        'rust',
        'java',
        'c',
        'cpp',
        'csharp',
    ];

    const disposable = vscode.languages.registerInlineCompletionItemProvider(
        languages.map(lang => ({ language: lang })),
        provider
    );

    context.subscriptions.push(disposable);

    return provider;
}
