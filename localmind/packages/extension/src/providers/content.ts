import * as vscode from 'vscode';

export class SuggestionContentProvider implements vscode.TextDocumentContentProvider {
    public static readonly scheme = 'localmind-suggestion';
    private _onDidChange = new vscode.EventEmitter<vscode.Uri>();
    private _contentMap = new Map<string, string>();

    public get onDidChange(): vscode.Event<vscode.Uri> {
        return this._onDidChange.event;
    }

    public provideTextDocumentContent(uri: vscode.Uri): string {
        return this._contentMap.get(uri.toString()) || '';
    }

    public setContent(uri: vscode.Uri, content: string): void {
        this._contentMap.set(uri.toString(), content);
        this._onDidChange.fire(uri);
    }
}
