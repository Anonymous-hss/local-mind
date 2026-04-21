/**
 * LocalMind IPC Protocol Types
 * 
 * TypeScript types matching the JSON schema in shared/protocol/messages.schema.json
 * for communication between VS Code extension and Go core engine.
 */

// =============================================================================
// Base Types
// =============================================================================

export type MessageId = string;
export type Timestamp = number;

export type RequestType = 'handshake' | 'ping' | 'completion' | 'suggestion' | 'agent' | 'memory' | 'cancel';
export type ResponseType = 'handshake_ack' | 'pong' | 'result' | 'stream' | 'error' | 'cancelled';

export type ErrorCode =
    | 'INTERNAL_ERROR'
    | 'TIMEOUT'
    | 'CANCELLED'
    | 'INVALID_REQUEST'
    | 'MODEL_UNAVAILABLE'
    | 'CONTEXT_TOO_LARGE'
    | 'LATENCY_EXCEEDED';

// =============================================================================
// Request Types
// =============================================================================

export interface BaseMessage {
    id: MessageId;
    timestamp: Timestamp;
}

export interface Request extends BaseMessage {
    type: RequestType;
    payload?: unknown;
}

export interface HandshakePayload {
    protocolVersion: string;
    extensionVersion: string;
}

export interface HandshakeRequest extends BaseMessage {
    type: 'handshake';
    payload: HandshakePayload;
}

export interface PingRequest extends BaseMessage {
    type: 'ping';
}

export interface CompletionPayload {
    prefix: string;
    suffix?: string;
    language: string;
    filePath?: string;
    maxTokens?: number;
}

export interface CompletionRequest extends BaseMessage {
    type: 'completion';
    payload: CompletionPayload;
}

export interface SuggestionPayload {
    file: string;
    startLine: number;
    endLine: number;
    content: string;
    context: string;
    language: string;
    suggestionType: 'refactor' | 'optimize' | 'explain' | 'fix';
}

export interface SuggestionRequest extends BaseMessage {
    type: 'suggestion';
    payload: SuggestionPayload;
}

export interface AgentPayload {
    action?: 'plan' | 'execute' | 'approve' | 'reject' | 'rollback' | 'status';
    task?: string;
    files?: string[];
    workspaceRoot?: string;
    taskId?: string;
    stepId?: string;
    reason?: string;
    // Settings (from VS Code configuration)
    model?: string;
    contextBudget?: number;
    maxSteps?: number;
    cursorFile?: string;
    cursorLine?: number;
}

export interface AgentRequest extends BaseMessage {
    type: 'agent';
    payload: AgentPayload;
}

export interface CancelPayload {
    requestId: MessageId;
}

export interface CancelRequest extends BaseMessage {
    type: 'cancel';
    payload: CancelPayload;
}

export interface MemoryPayload {
    action: 'get' | 'update' | 'delete';
    workspace?: string;
    category?: string;
    key?: string;
    value?: string;
}

export interface MemoryRequest extends BaseMessage {
    type: 'memory';
    payload: MemoryPayload;
}

// =============================================================================
// Response Types
// =============================================================================

export interface Response extends BaseMessage {
    type: ResponseType;
    requestId: MessageId;
}

export interface HandshakeAckPayload {
    protocolVersion: string;
    coreVersion: string;
    compatible: boolean;
}

export interface HandshakeAckResponse extends BaseMessage {
    type: 'handshake_ack';
    requestId: MessageId;
    payload: HandshakeAckPayload;
}

export interface ModelInfo {
    name: string;
    size: number;       // bytes
    role: string;       // assigned role(s), e.g. "completion, agent"
}

export interface PongPayload {
    version?: string;
    ollamaStatus?: 'connected' | 'disconnected' | 'unknown';
    models?: ModelInfo[];
    activeModels?: Record<string, string>;  // role → model name
}

export interface PongResponse extends Response {
    type: 'pong';
    payload?: PongPayload;
}

// Hunk represents a single change in a diff
export interface Hunk {
    startLineOld: number;
    endLineOld: number;
    startLineNew: number;
    endLineNew: number;
    before: string;
    after: string;
}

// Diff represents a unified diff for a single file
export interface Diff {
    file: string;
    language: string;
    hunks: Hunk[];
}

export interface ResultPayload {
    content: string;
    latencyMs: number;
    model?: string;
}

export interface ResultResponse extends Response {
    type: 'result';
    payload: ResultPayload;
}

export interface StreamPayload {
    chunk: string;
    done: boolean;
}

export interface StreamResponse extends Response {
    type: 'stream';
    payload: StreamPayload;
}

export interface ErrorPayload {
    code: ErrorCode;
    message: string;
    details?: Record<string, unknown>;
}

export interface ErrorResponse extends Response {
    type: 'error';
    payload: ErrorPayload;
}

export interface CancelledResponse extends Response {
    type: 'cancelled';
}

// =============================================================================
// Union Types for Handling
// =============================================================================

export type AnyRequest =
    | HandshakeRequest
    | PingRequest
    | CompletionRequest
    | SuggestionRequest
    | AgentRequest
    | MemoryRequest
    | CancelRequest;

export type AnyResponse =
    | HandshakeAckResponse
    | PongResponse
    | ResultResponse
    | StreamResponse
    | ErrorResponse
    | CancelledResponse;

// =============================================================================
// Utility Functions
// =============================================================================

let messageCounter = 0;

/**
 * Generate a unique message ID
 */
export function generateMessageId(): MessageId {
    return `msg-${Date.now()}-${++messageCounter}`;
}

/**
 * Create a timestamp for the current time
 */
export function createTimestamp(): Timestamp {
    return Date.now();
}

/**
 * Create a completion request
 */
export function createCompletionRequest(
    prefix: string,
    language: string,
    options?: { suffix?: string; filePath?: string; maxTokens?: number }
): CompletionRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'completion',
        payload: {
            prefix,
            language,
            ...options,
        },
    };
}

/**
 * Create a ping request
 */
export function createPingRequest(): PingRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'ping',
    };
}

/**
 * Create a cancel request
 */
export function createCancelRequest(requestId: MessageId): CancelRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'cancel',
        payload: { requestId },
    };
}

/**
 * Create a handshake request
 */
export function createHandshakeRequest(extensionVersion: string): HandshakeRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'handshake',
        payload: {
            protocolVersion: '1.0.0',
            extensionVersion,
        },
    };
}

/**
 * Type guard for error responses
 */
export function isErrorResponse(response: AnyResponse): response is ErrorResponse {
    return response.type === 'error';
}

/**
 * Type guard for result responses
 */
export function isResultResponse(response: AnyResponse): response is ResultResponse {
    return response.type === 'result';
}

/**
 * Type guard for stream responses
 */
export function isStreamResponse(response: AnyResponse): response is StreamResponse {
    return response.type === 'stream';
}

/**
 * Create an agent request
 */
export function createAgentRequest(payload: AgentPayload): AgentRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'agent',
        payload,
    };
}

/**
 * Create a memory request
 */
export function createMemoryRequest(payload: MemoryPayload): MemoryRequest {
    return {
        id: generateMessageId(),
        timestamp: createTimestamp(),
        type: 'memory',
        payload,
    };
}
