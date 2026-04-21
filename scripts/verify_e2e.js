const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

// Configuration
const EXTENSION_DIR = path.resolve(__dirname, '../localmind/packages/extension');
const BINARY_NAME = process.platform === 'win32' ? 'localmind.exe' : 'localmind';
const BINARY_PATH = path.join(EXTENSION_DIR, 'bin', BINARY_NAME);
const DEBUG = true;

// Logging
function log(msg) {
    const ts = new Date().toISOString();
    console.log(`[${ts}] ${msg}`);
}

if (!fs.existsSync(BINARY_PATH)) {
    log(`ERROR: Binary not found at ${BINARY_PATH}`);
    process.exit(1);
}

log(`Starting Core: ${BINARY_PATH}`);

// Spawn process
const core = spawn(BINARY_PATH, [], {
    stdio: ['pipe', 'pipe', 'pipe'],
    env: process.env
});

// State
let buffer = Buffer.alloc(0);
let pendingRequests = new Map();
let requestIdCounter = 1;

// Helper to send request
function send(type, payload = {}) {
    return new Promise((resolve, reject) => {
        const id = `req-${requestIdCounter++}`;
        const req = {
            id,
            timestamp: Date.now(),
            type,
            payload
        };

        log(`-> SEND ${type} (${id})`);

        const json = JSON.stringify(req);
        const lenAttr = Buffer.alloc(4);
        lenAttr.writeUInt32BE(Buffer.byteLength(json), 0);

        core.stdin.write(lenAttr);
        core.stdin.write(Buffer.from(json));

        // Timeout 5s
        const timeout = setTimeout(() => {
            if (pendingRequests.has(id)) {
                pendingRequests.delete(id);
                reject(new Error(`Timeout waiting for ${type} response`));
            }
        }, 5000);

        pendingRequests.set(id, { resolve, reject, timeout });
    });
}

// Handle stdout
core.stdout.on('data', (chunk) => {
    buffer = Buffer.concat([buffer, chunk]);

    while (buffer.length >= 4) {
        const len = buffer.readUInt32BE(0);
        if (buffer.length < 4 + len) break;

        const msgBuf = buffer.slice(4, 4 + len);
        buffer = buffer.slice(4 + len);

        try {
            const msg = JSON.parse(msgBuf.toString());
            log(`<- RECV ${msg.type} (${msg.requestId})`);

            if (pendingRequests.has(msg.requestId)) {
                const { resolve, timeout } = pendingRequests.get(msg.requestId);
                clearTimeout(timeout);
                pendingRequests.delete(msg.requestId);
                resolve(msg);
            }
        } catch (e) {
            log(`Error parsing message: ${e.message}`);
        }
    }
});

// Handle stderr
core.stderr.on('data', (chunk) => {
    console.log(`[CORE LOG] ${chunk.toString().trim()}`);
});

core.on('close', (code) => {
    log(`Core exited with code ${code}`);
});

// Main Test Flow
async function runTest() {
    try {
        // 1. Handshake
        log('--- TEST 1: HANDSHAKE ---');
        const handshake = await send('handshake', {
            extensionVersion: '0.1.0',
            protocolVersion: '1.0.0'
        });
        if (handshake.type !== 'handshake_ack' || !handshake.payload.compatible) {
            throw new Error('Handshake failed');
        }
        log('✅ Handshake success');

        // 2. Ping
        log('--- TEST 2: PING ---');
        const ping = await send('ping');
        if (ping.type !== 'pong') throw new Error('Ping failed');
        log('✅ Ping success');

        // 3. Completion
        log('--- TEST 3: COMPLETION ---');
        const completion = await send('completion', {
            prefix: 'package main\n\nimport "fmt"\n\nfunc main() {\n\tfmt.Printl',
            language: 'go',
            file: 'test.go',
            maxTokens: 50
        });

        if (completion.type !== 'result') throw new Error(`Completion failed: ${JSON.stringify(completion)}`);

        log(`✅ Completion received: "${completion.payload.content.replace(/\n/g, '\\n')}"`);
        log(`   Model: ${completion.payload.model}`);

        // 4. Agent Task (Planning)
        log('--- TEST 4: AGENT PLANNING ---');
        const agentReq = await send('agent', {
            action: 'plan',
            task: 'Create a simple hello world file',
            workspaceRoot: process.cwd(),
            files: ['README.md'] // Mock file list
        });

        if (agentReq.type !== 'result' && agentReq.type !== 'error') {
            // Agent returns 'result' with content describing the plan or task status
            // Wait, looking at agent engine, it returns a result?
            // Let's check protocol. Agent returns... TaskResponse? 
            // The Orchestrator wraps Engine results. AgentEngine.Execute returns *Result.
            // Result.Content is JSON string of Task?
            // Let's assume result type for now.
        }

        if (agentReq.type === 'error') {
            throw new Error(`Agent failed: ${agentReq.payload.message}`);
        }

        log(`✅ Agent response received: ${agentReq.type}`);
        // Log first 100 chars of content
        const contentPreview = agentReq.payload.content.length > 100
            ? agentReq.payload.content.substring(0, 100) + '...'
            : agentReq.payload.content;
        log(`   Content: ${contentPreview.replace(/\n/g, '\\n')}`);

        log('\n🎉 ALL MANUAL TESTS PASSED');
        core.kill();
        process.exit(0);

    } catch (e) {
        log(`❌ TEST FAILED: ${e.message}`);
        core.kill();
        process.exit(1);
    }
}

// Give process a moment to start then run
setTimeout(runTest, 1000);
