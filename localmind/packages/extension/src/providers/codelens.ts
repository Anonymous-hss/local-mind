import * as vscode from 'vscode';
import { IPCClient } from '../core/client';
import { SuggestionPayload, generateMessageId, createTimestamp, Diff, Hunk } from '../core/protocol';
import { SuggestionContentProvider } from './content';

export class ActionCodeLensProvider implements vscode.CodeLensProvider {
    constructor(private readonly client: IPCClient) { }

    public async provideCodeLenses(document: vscode.TextDocument, token: vscode.CancellationToken): Promise<vscode.CodeLens[]> {
        // Only provide lenses for supported languages
        if (!['typescript', 'javascript', 'go', 'python', 'rust'].includes(document.languageId)) {
            return [];
        }

        const lenses: vscode.CodeLens[] = [];

        // Find functions and classes
        const symbols = await vscode.commands.executeCommand<vscode.DocumentSymbol[]>(
            'vscode.executeDocumentSymbolProvider',
            document.uri
        );

        if (!symbols) {
            return [];
        }

        const visit = (symbol: vscode.DocumentSymbol) => {
            if (symbol.kind === vscode.SymbolKind.Function ||
                symbol.kind === vscode.SymbolKind.Method ||
                symbol.kind === vscode.SymbolKind.Class) {

                const range = symbol.range;

                // Add "Refactor" lens
                const refactorCmd: vscode.Command = {
                    title: '$(light-bulb) Refactor',
                    tooltip: 'Ask LocalMind to refactor this code',
                    command: 'localmind.suggestRefactor',
                    arguments: [document, range, symbol.name]
                };
                lenses.push(new vscode.CodeLens(range, refactorCmd));

                // Add "Explain" lens
                const explainCmd: vscode.Command = {
                    title: '$(info) Explain',
                    tooltip: 'Ask LocalMind to explain this code',
                    command: 'localmind.explainCode',
                    arguments: [document, range, symbol.name]
                };
                lenses.push(new vscode.CodeLens(range, explainCmd));
            }

            if (symbol.children) {
                symbol.children.forEach(visit);
            }
        };

        symbols.forEach(visit);
        return lenses;
    }
}

// Command Handler for Refactoring
export async function suggestRefactorCommand(
    contentProvider: SuggestionContentProvider,
    client: IPCClient,
    document: vscode.TextDocument,
    range: vscode.Range,
    name: string
) {
    if (!client) {
        vscode.window.showErrorMessage('LocalMind is not initialized');
        return;
    }

    const content = document.getText(range);

    // Provide visual feedback
    await vscode.window.withProgress({
        location: vscode.ProgressLocation.Notification,
        title: `Analyzing ${name}...`,
        cancellable: true
    }, async (progress, token) => {
        try {
            const payload: SuggestionPayload = {
                file: document.uri.fsPath,
                startLine: range.start.line + 1,
                endLine: range.end.line + 1,
                content: content,
                context: getTextContext(document, range),
                language: document.languageId,
                suggestionType: 'refactor'
            };

            const request = {
                id: generateMessageId(),
                timestamp: createTimestamp(),
                type: 'suggestion' as const,
                payload
            };

            // Send request
            const response = await client.send(request, 30000); // 30s timeout

            if (response.type === 'error') {
                throw new Error(response.payload.message);
            }

            if (response.type === 'result') {
                const resultObj = JSON.parse(response.payload.content);
                const suggestions = resultObj.suggestions;

                if (!suggestions || suggestions.length === 0) {
                    vscode.window.showInformationMessage('No refactoring suggestions found.');
                    return;
                }

                // Take the first suggestion
                const suggestion = suggestions[0];
                const diff: Diff = suggestion.diff;

                if (diff) {
                    // Apply diff to generate new content
                    const newContent = applyDiff(content, diff, range.start.line + 1);

                    // Create URI for the suggested content
                    const suggestedUri = vscode.Uri.parse(
                        `${SuggestionContentProvider.scheme}:${document.uri.path}.suggested.${document.languageId}`
                    );

                    // Update content provider
                    contentProvider.setContent(suggestedUri, newContent);

                    // Open Diff Editor
                    await vscode.commands.executeCommand(
                        'vscode.diff',
                        document.uri,
                        suggestedUri,
                        `Refactor Suggestion: ${name}`
                    );
                } else {
                    vscode.window.showInformationMessage(`Suggestion: ${suggestion.explanation}`);
                }
            }
        } catch (err) {
            vscode.window.showErrorMessage(`Refactoring failed: ${err}`);
        }
    });
}

// Helper: Get context (surrounding lines)
function getTextContext(document: vscode.TextDocument, range: vscode.Range): string {
    const startLine = Math.max(0, range.start.line - 10);
    const endLine = Math.min(document.lineCount - 1, range.end.line + 10);
    const contextRange = new vscode.Range(startLine, 0, endLine, 9999);
    return document.getText(contextRange);
}

// Helper: Apply Diff
// Logic: Replaces the hunk's 'before' with 'after'. 
// Since we are operating on the snippet 'content', we need to adjust line numbers.
// The diff hunks are absolute line numbers for the FILE, but we are diffing against the SNIPPET.
// Wait, diffing against snippet or file?
// The backend returns absolute line numbers.
// My applyDiff takes `content` (snippet) and `startLineOffset`.
function applyDiff(originalContent: string, diff: Diff, startLineOffset: number): string {
    const lines = originalContent.split(/\r?\n/);

    // Process hunks in reverse order to identify indexes correctly
    const hunks = (diff.hunks || []).sort((a: Hunk, b: Hunk) => b.startLineOld - a.startLineOld);

    for (const hunk of hunks) {
        // Calculate relative lines (0-indexed)
        // startLineOld is 1-based absolute line
        // startLineOffset is the absolute line number of the first line of content
        const hunkStart = hunk.startLineOld - startLineOffset;
        const hunkEnd = hunk.endLineOld - startLineOffset;

        if (hunkStart < 0 || hunkEnd >= lines.length) {
            console.warn('Hunk out of bounds', hunk, startLineOffset, lines.length);
            continue;
        }

        // Parse 'after' content from the hunk
        let newLines = hunk.after ? hunk.after.split(/\r?\n/) : [];

        // Remove trailing empty string from split if it ends with newline
        if (newLines.length > 0 && hunk.after.endsWith('\n')) {
            newLines.pop();
        }

        // Replace lines
        lines.splice(hunkStart, hunkEnd - hunkStart + 1, ...newLines);
    }

    return lines.join('\n');
}
