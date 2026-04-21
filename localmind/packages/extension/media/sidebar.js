const vscode = acquireVsCodeApi();

window.addEventListener('load', () => {
    const input = document.getElementById('commandInput');
    const approveBtn = document.getElementById('btnApprove');
    const rejectBtn = document.getElementById('btnReject');
    const abortBtn = document.getElementById('btnAbort');

    // Handle Enter key in input
    input.addEventListener('keydown', (e) => {
        if (e.key === 'Enter') {
            const command = input.value.trim();
            if (command) {
                vscode.postMessage({
                    type: 'command',
                    text: command
                });
                input.value = '';
            }
        }
    });

    // Handle buttons
    approveBtn.addEventListener('click', () => {
        vscode.postMessage({ type: 'approve' });
    });

    rejectBtn.addEventListener('click', () => {
        vscode.postMessage({ type: 'reject' });
    });

    abortBtn.addEventListener('click', () => {
        vscode.postMessage({ type: 'abort' });
    });

    // Handle messages from extension
    window.addEventListener('message', event => {
        const message = event.data;
        switch (message.type) {
            case 'updatePlan':
                updatePlan(message.plan);
                break;
            case 'log':
                addLog(message.text, message.level);
                break;
            case 'status':
                updateStatus(message.status);
                break;
            case 'stream':
                handleStream(message.requestId, message.chunk);
                break;
        }
    });
});

function updatePlan(plan) {
    const list = document.getElementById('planList');
    list.innerHTML = '';

    plan.steps.forEach(step => {
        const li = document.createElement('li');
        li.className = `plan-item ${step.status}`;
        if (step.status === 'active') li.classList.add('active');

        let icon = '[ ]';
        if (step.status === 'done') icon = '[x]';
        if (step.status === 'active') icon = '[>]';

        li.innerHTML = `
            <span class="checkbox">${icon}</span>
            <span class="${step.status === 'done' ? 'line-through' : ''}">${step.description}</span>
        `;
        list.appendChild(li);
    });
}

function addLog(text, level = 'info') {
    // Reset stream state
    currentStreamId = null;
    currentStreamElement = null;

    const container = document.getElementById('logContainer');
    const div = document.createElement('div');
    const time = new Date().toLocaleTimeString('en-US', { hour12: false });

    div.className = `log-entry`;
    div.innerHTML = `
        <span class="log-time">${time} ~</span>
        <span class="log-msg ${level}">${text}</span>
    `;

    container.appendChild(div);
    container.scrollTop = container.scrollHeight;
}

function updateStatus(status) {
    const dot = document.getElementById('statusDot');
    const text = document.getElementById('statusText');

    if (status === 'online') {
        dot.style.backgroundColor = 'var(--success)';
        dot.style.boxShadow = '0 0 4px var(--success)';
        text.innerText = 'ONLINE';
    } else {
        dot.style.backgroundColor = 'var(--danger)';
        dot.style.boxShadow = 'none';
        text.innerText = 'OFFLINE';
    }
}


let currentStreamId = null;
let currentStreamElement = null;

function handleStream(requestId, chunk) {
    const container = document.getElementById('logContainer');

    // If new request or stream interrupted, create new entry
    if (currentStreamId !== requestId || !currentStreamElement) {
        currentStreamId = requestId;

        const div = document.createElement('div');
        div.className = 'log-entry stream';
        div.innerHTML = `
            <span class="log-time">Thinking...</span>
            <span class="log-msg stream-content"></span>
        `;

        container.appendChild(div);
        currentStreamElement = div.querySelector('.stream-content');
    }

    if (currentStreamElement) {
        // Append text safely
        currentStreamElement.textContent += chunk;
        container.scrollTop = container.scrollHeight;
    }
}


