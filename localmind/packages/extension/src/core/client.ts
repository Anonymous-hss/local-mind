/**
 * LocalMind IPC Client
 * 
 * Manages communication with the Go core engine via STDIO.
 * Uses length-prefixed JSON messages matching the Go IPC protocol.
 */

import { spawn, ChildProcess } from 'child_process';
import * as path from 'path';
import * as vscode from 'vscode';
import { ensureCoreBinary } from './downloader';
import {
    AnyRequest,
    AnyResponse,
    MessageId,
    PongPayload,
    createHandshakeRequest,
    createPingRequest,
    createCancelRequest,
    isErrorResponse,
    StreamPayload,
} from './protocol';

// =============================================================================
// Types
// =============================================================================

interface PendingRequest {
    resolve: (response: AnyResponse) => void;
    reject: (error: Error) => void;
    timeout: NodeJS.Timeout;
}

export interface ClientOptions {
    corePath?: string;
    timeout?: number;
    extensionVersion?: string;
    debug?: boolean;
}

export type ClientState = 'stopped' | 'starting' | 'ready' | 'error';

// =============================================================================
// IPC Client
// =============================================================================

export class IPCClient {
    private process: ChildProcess | null = null;
    private pendingRequests: Map<MessageId, PendingRequest> = new Map();
    private state: ClientState = 'stopped';
    private buffer: Buffer = Buffer.alloc(0);
    private outputChannel: vscode.OutputChannel;
    private streamEmitter = new vscode.EventEmitter<{ requestId: string; chunk: string }>();
    public readonly onStream = this.streamEmitter.event;

    private corePath: string;  // Resolved at start-time via downloader
    private readonly timeout: number;
    private readonly extensionVersion: string;
    private readonly debug: boolean;

    constructor(private readonly context: vscode.ExtensionContext, options: ClientOptions = {}) {
        this.corePath = options.corePath || '';  // Resolved in start()
        this.timeout = options.timeout || 30000; // Longer timeout for download
        this.extensionVersion = options.extensionVersion || '0.1.0';
        this.debug = options.debug || false;
        this.outputChannel = vscode.window.createOutputChannel('LocalMind Core');

        this.log(`Debug mode: ${this.debug}`);
    }

    /**
     * Show the output channel with logs
     */
    showLogs(): void {
        this.outputChannel.show(true);
    }

    // =========================================================================
    // Lifecycle
    // =========================================================================

    /**
     * Start the core engine process
     */
    async start(): Promise<void> {
        if (this.state === 'ready' || this.state === 'starting') {
            return;
        }

        this.state = 'starting';
        this.log('Starting core engine...');

        try {
            // --- Step 1: Ensure binary is downloaded ---
            const isDev = this.context.extensionMode === vscode.ExtensionMode.Development;
            let spawnCmd = '';
            let spawnArgs: string[] = [];
            // In dev mode, we want to run from the extension source tree (../core), not the user's opened workspace
            let spawnCwd = isDev ? path.join(this.context.extensionPath, '..', 'core') : vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;

            if (isDev && spawnCwd) {
                this.log(`Development mode: using \`go run\` from source at ${spawnCwd}`);
                spawnCmd = process.platform === 'win32' ? 'go.exe' : 'go';
                spawnArgs = ['run', './cmd/localmind'];
            } else {
                this.corePath = await vscode.window.withProgress(
                    {
                        location: vscode.ProgressLocation.Notification,
                        title: 'LocalMind',
                        cancellable: false,
                    },
                    async (progress) => {
                        progress.report({ message: 'Checking core engine...' });
                        return ensureCoreBinary(
                            this.context,
                            this.extensionVersion,
                            (msg) => {
                                progress.report({ message: msg });
                                this.log(msg);
                            }
                        );
                    }
                );

                this.log(`Core binary path: ${this.corePath}`);
                spawnCmd = this.corePath;
            }

            // --- Step 2: Spawn the core process ---
            this.process = spawn(spawnCmd, spawnArgs, {
                stdio: ['pipe', 'pipe', 'pipe'],
                cwd: spawnCwd,
            });

            this.setupProcessHandlers();

            // Wait for process to be ready
            await this.waitForReady();

            // Perform handshake
            await this.handshake();

            this.state = 'ready';
            this.log('Core engine ready');
        } catch (error) {
            this.state = 'error';
            this.log(`Failed to start: ${error}`);
            throw error;
        }
    }

    /**
     * Stop the core engine process
     */
    async stop(): Promise<void> {
        if (!this.process) {
            return;
        }

        this.log('Stopping core engine...');

        // Cancel all pending requests
        for (const [id, pending] of this.pendingRequests) {
            clearTimeout(pending.timeout);
            pending.reject(new Error('Client stopped'));
        }
        this.pendingRequests.clear();

        // Kill process
        this.process.kill('SIGTERM');
        this.process = null;
        this.state = 'stopped';
        this.log('Core engine stopped');
    }

    /**
     * Check if client is ready
     */
    isReady(): boolean {
        return this.state === 'ready';
    }

    /**
     * Get current state
     */
    getState(): ClientState {
        return this.state;
    }

    // =========================================================================
    // Requests
    // =========================================================================

    /**
     * Send a request and wait for response
     */
    async send<T extends AnyResponse>(request: AnyRequest, timeoutMs?: number): Promise<T> {
        // Allow requests during 'starting' (for handshake) and 'ready' states
        if (!this.process || (this.state !== 'ready' && this.state !== 'starting')) {
            throw new Error('Client not ready');
        }

        return new Promise<T>((resolve, reject) => {
            const timeout = setTimeout(() => {
                this.pendingRequests.delete(request.id);
                reject(new Error(`Request timeout after ${timeoutMs || this.timeout}ms`));
            }, timeoutMs || this.timeout);

            this.pendingRequests.set(request.id, {
                resolve: resolve as (response: AnyResponse) => void,
                reject,
                timeout,
            });

            this.writeMessage(request);
        });
    }

    /**
     * Send a ping request
     */
    async ping(): Promise<boolean> {
        try {
            const response = await this.send(createPingRequest(), 5000);
            return response.type === 'pong';
        } catch {
            return false;
        }
    }

    /**
     * Send a ping request and return the full pong payload with model info
     */
    async pingFull(): Promise<PongPayload | null> {
        try {
            const response = await this.send(createPingRequest(), 5000);
            if (response.type === 'pong') {
                return (response as any).payload || {};
            }
            return null;
        } catch {
            return null;
        }
    }

    /**
     * Cancel a pending request
     */
    async cancel(requestId: MessageId): Promise<void> {
        if (!this.process || this.state !== 'ready') {
            return;
        }

        // Send cancel request (fire and forget)
        this.writeMessage(createCancelRequest(requestId));

        // Clean up pending request
        const pending = this.pendingRequests.get(requestId);
        if (pending) {
            clearTimeout(pending.timeout);
            pending.reject(new Error('Cancelled'));
            this.pendingRequests.delete(requestId);
        }
    }

    // =========================================================================
    // Private Methods
    // =========================================================================

    private getDefaultCorePath(): string {
        // Look for the binary relative to extension
        const extensionPath = this.context.extensionPath;
        const platform = process.platform;
        const binaryName = platform === 'win32' ? 'localmind.exe' : 'localmind';

        // Try bin directory first
        return path.join(extensionPath, 'bin', binaryName);
    }

    private setupProcessHandlers(): void {
        if (!this.process) return;

        // Handle stdout (responses from core)
        this.process.stdout?.on('data', (data: Buffer) => {
            this.handleData(data);
        });

        // Handle stderr (logs from core)
        this.process.stderr?.on('data', (data: Buffer) => {
            this.log(`[core] ${data.toString().trim()}`);
        });

        // Handle process exit
        this.process.on('exit', (code, signal) => {
            this.log(`Core exited with code ${code}, signal ${signal}`);
            this.state = 'stopped';

            // Reject all pending requests
            for (const [id, pending] of this.pendingRequests) {
                clearTimeout(pending.timeout);
                pending.reject(new Error('Core process exited'));
            }
            this.pendingRequests.clear();
        });

        // Handle process error
        this.process.on('error', (error) => {
            this.log(`Process error: ${error.message}`);
            this.state = 'error';
        });
    }

    private async waitForReady(): Promise<void> {
        return new Promise((resolve, reject) => {
            const timeout = setTimeout(() => {
                reject(new Error('Timeout waiting for core to start'));
            }, 5000);

            // Process is ready when it doesn't exit immediately
            const checkReady = () => {
                if (this.process && !this.process.killed) {
                    clearTimeout(timeout);
                    resolve();
                }
            };

            // Give it a moment to start
            setTimeout(checkReady, 100);
        });
    }

    private async handshake(): Promise<void> {
        const request = createHandshakeRequest(this.extensionVersion);
        const response = await this.send(request, 5000);

        if (response.type === 'handshake_ack') {
            const payload = (response as any).payload;
            if (!payload.compatible) {
                throw new Error(`Protocol mismatch: extension ${this.extensionVersion}, core ${payload.coreVersion}`);
            }
            this.log(`Handshake complete: core v${payload.coreVersion}`);
        } else if (isErrorResponse(response)) {
            throw new Error(`Handshake failed: ${response.payload.message}`);
        } else {
            throw new Error(`Unexpected handshake response: ${response.type}`);
        }
    }

    /**
     * Handle incoming data from core
     * Uses length-prefixed framing: [4-byte length][JSON data]
     */
    private handleData(data: Buffer): void {
        this.buffer = Buffer.concat([this.buffer, data]);

        while (this.buffer.length >= 4) {
            // Read message length (first 4 bytes, big-endian)
            const length = this.buffer.readUInt32BE(0);

            // Check if we have the full message
            if (this.buffer.length < 4 + length) {
                break;
            }

            // Extract message
            const messageData = this.buffer.slice(4, 4 + length);
            this.buffer = this.buffer.slice(4 + length);

            try {
                const message = JSON.parse(messageData.toString('utf-8')) as AnyResponse;
                this.handleMessage(message);
            } catch (error) {
                this.log(`Failed to parse message: ${error}`);
            }
        }
    }

    /**
     * Handle a parsed response message
     */
    private handleMessage(response: AnyResponse): void {
        if (this.debug) {
            // throttle stream logs
            if (response.type !== 'stream') {
                this.log(`<- ${JSON.stringify(response)}`);
            }
        }

        // Handle streaming responses
        if (response.type === 'stream') {
            const payload = response.payload as StreamPayload;
            this.streamEmitter.fire({
                requestId: response.requestId,
                chunk: payload.chunk
            });
            return;
        }

        // Find and resolve pending request
        const pending = this.pendingRequests.get(response.requestId);
        if (pending) {
            clearTimeout(pending.timeout);
            this.pendingRequests.delete(response.requestId);
            pending.resolve(response);
        }
    }

    /**
     * Write a message to the core process
     * Uses length-prefixed framing: [4-byte length][JSON data]
     */
    private writeMessage(message: AnyRequest): void {
        if (!this.process?.stdin) {
            throw new Error('Process not available');
        }

        if (this.debug) {
            this.log(`-> ${JSON.stringify(message)}`);
        }

        const data = Buffer.from(JSON.stringify(message), 'utf-8');
        const lengthBuffer = Buffer.alloc(4);
        lengthBuffer.writeUInt32BE(data.length, 0);

        this.process.stdin.write(lengthBuffer);
        this.process.stdin.write(data);
    }

    private log(message: string): void {
        const timestamp = new Date().toISOString();
        this.outputChannel.appendLine(`[${timestamp}] ${message}`);

        if (this.debug) {
            console.log(`[LocalMind] ${message}`);
        }
    }
}
