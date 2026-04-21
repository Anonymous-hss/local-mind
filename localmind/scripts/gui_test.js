/**
 * LocalMind GUI Test Suite
 *
 * Runs headless browser tests against the dashboard webview using Puppeteer.
 * Tests all UI interactions: tab switching, button clicks, command input,
 * data rendering, empty states, streaming, and edge cases.
 *
 * Usage:  node scripts/gui_test.js [--headed] [--screenshots]
 * Output: JSON report to stdout
 */

const puppeteer = require('puppeteer');
const path = require('path');
const fs = require('fs');

// ─── Configuration ───────────────────────────────────────────────────────────

const HARNESS_PATH = path.resolve(__dirname, '../packages/extension/test/gui_harness.html');
const SCREENSHOT_DIR = path.resolve(__dirname, '../packages/extension/test/screenshots');
const HEADED = process.argv.includes('--headed');
const SCREENSHOTS = process.argv.includes('--screenshots');

// ─── Test Framework ──────────────────────────────────────────────────────────

const results = [];
let browser, page;
let jsErrors = [];

async function setup() {
    browser = await puppeteer.launch({
        headless: HEADED ? false : 'new',
        args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage']
    });
    page = await browser.newPage();
    await page.setViewport({ width: 400, height: 800 });

    // Capture JS errors
    page.on('pageerror', err => {
        jsErrors.push(err.message);
    });
    page.on('console', msg => {
        if (msg.type() === 'error') {
            jsErrors.push(msg.text());
        }
    });

    // Ensure screenshot directory exists
    if (SCREENSHOTS && !fs.existsSync(SCREENSHOT_DIR)) {
        fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });
    }
}

async function teardown() {
    if (browser) await browser.close();
}

async function loadPage() {
    jsErrors = [];
    const fileUrl = `file:///${HARNESS_PATH.replace(/\\/g, '/')}`;
    await page.goto(fileUrl, { waitUntil: 'networkidle0' });
    // Wait for dashboard.js to initialize (it binds on 'load')
    await page.waitForSelector('#commandInput', { timeout: 5000 });
    // Clear any messages from initialization
    await page.evaluate(() => window.__clearMessages());
}

async function runTest(name, fn) {
    const start = Date.now();
    const result = { name, passed: false, duration: '0ms', error: null };
    try {
        await fn();
        result.passed = true;
    } catch (err) {
        result.error = err.message || String(err);
        // Take screenshot on failure
        if (SCREENSHOTS) {
            const safeName = name.replace(/[^a-zA-Z0-9]/g, '_').toLowerCase();
            const screenshotPath = path.join(SCREENSHOT_DIR, `FAIL_${safeName}.png`);
            try {
                await page.screenshot({ path: screenshotPath, fullPage: true });
                result.screenshot = screenshotPath;
            } catch { /* ignore screenshot errors */ }
        }
    }
    result.duration = `${Date.now() - start}ms`;
    results.push(result);
}

function assert(condition, message) {
    if (!condition) throw new Error(`Assertion failed: ${message}`);
}

function assertEqual(actual, expected, label) {
    if (actual !== expected) {
        throw new Error(`${label}: expected "${expected}", got "${actual}"`);
    }
}

function assertIncludes(text, substring, label) {
    if (!text.includes(substring)) {
        throw new Error(`${label}: expected to include "${substring}", got "${text}"`);
    }
}

// ─── Helper: Inject a message into the webview ───────────────────────────────

async function injectMessage(data) {
    await page.evaluate((d) => window.__injectMessage(d), data);
    // Small delay for DOM updates
    await new Promise(r => setTimeout(r, 100));
}

async function getCapturedMessages() {
    return page.evaluate(() => window.__testMessages);
}

async function clearMessages() {
    return page.evaluate(() => window.__clearMessages());
}

// ─── Tests ───────────────────────────────────────────────────────────────────

async function test_01_PageLoads() {
    await loadPage();
    // Verify key elements exist
    const elements = await page.evaluate(() => ({
        commandInput: !!document.getElementById('commandInput'),
        btnApprove: !!document.getElementById('btnApprove'),
        btnReject: !!document.getElementById('btnReject'),
        btnAbort: !!document.getElementById('btnAbort'),
        planList: !!document.getElementById('planList'),
        logContainer: !!document.getElementById('logContainer'),
        statusDot: !!document.getElementById('statusDot'),
        statusText: !!document.getElementById('statusText'),
        historyList: !!document.getElementById('historyList'),
        scoresGrid: !!document.getElementById('scoresGrid'),
        strategyList: !!document.getElementById('strategyList'),
        lessonList: !!document.getElementById('lessonList'),
    }));

    for (const [id, exists] of Object.entries(elements)) {
        assert(exists, `Element #${id} should exist`);
    }

    // No JS errors
    assert(jsErrors.length === 0, `No JS errors expected, got: ${jsErrors.join('; ')}`);
}

async function test_02_InitialState() {
    // Status should show CONNECTING
    const statusText = await page.$eval('#statusText', el => el.innerText);
    assertEqual(statusText, 'CONNECTING', 'Initial status text');

    // Agent tab should be active
    const activeTab = await page.$eval('.tab-btn.active', el => el.dataset.tab);
    assertEqual(activeTab, 'agent', 'Initial active tab');

    // Agent panel should be visible
    const agentPanelActive = await page.$eval('#panel-agent', el => el.classList.contains('active'));
    assert(agentPanelActive, 'Agent panel should be active initially');

    // Plan list should show "Awaiting command..."
    const planText = await page.$eval('#planList', el => el.textContent);
    assertIncludes(planText, 'Awaiting command', 'Initial plan text');
}

async function test_03_StatusOnline() {
    await injectMessage({ type: 'status', status: 'online' });

    const statusText = await page.$eval('#statusText', el => el.innerText);
    assertEqual(statusText, 'ONLINE', 'Status text after online');

    const hasOffline = await page.$eval('#statusDot', el => el.classList.contains('offline'));
    assert(!hasOffline, 'Status dot should NOT have offline class');
}

async function test_04_StatusOffline() {
    await injectMessage({ type: 'status', status: 'offline' });

    const statusText = await page.$eval('#statusText', el => el.innerText);
    assertEqual(statusText, 'OFFLINE', 'Status text after offline');

    const hasOffline = await page.$eval('#statusDot', el => el.classList.contains('offline'));
    assert(hasOffline, 'Status dot should have offline class');
}

async function test_05_TabSwitching() {
    const tabs = ['history', 'scores', 'strategies', 'lessons', 'agent'];

    for (const tab of tabs) {
        await clearMessages();
        // Click the tab button
        await page.click(`.tab-btn[data-tab="${tab}"]`);
        await new Promise(r => setTimeout(r, 100));

        // Verify the button is active
        const activeTab = await page.$eval('.tab-btn.active', el => el.dataset.tab);
        assertEqual(activeTab, tab, `Active tab after clicking ${tab}`);

        // Verify the panel is visible
        const panelActive = await page.$eval(`#panel-${tab}`, el => el.classList.contains('active'));
        assert(panelActive, `Panel #panel-${tab} should be active`);

        // Other panels should NOT be active
        for (const other of tabs) {
            if (other !== tab) {
                const otherActive = await page.$eval(`#panel-${other}`, el => el.classList.contains('active'));
                assert(!otherActive, `Panel #panel-${other} should NOT be active when ${tab} is selected`);
            }
        }
    }
}

async function test_06_TabSwitchingTriggersDataFetch() {
    await clearMessages();

    // Switching to history tab should trigger a fetchHistory message
    await page.click('.tab-btn[data-tab="history"]');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const fetchHistory = msgs.find(m => m.type === 'fetchHistory');
    assert(!!fetchHistory, 'Switching to history tab should send fetchHistory message');

    // Switch to scores
    await clearMessages();
    await page.click('.tab-btn[data-tab="scores"]');
    await new Promise(r => setTimeout(r, 100));

    const msgs2 = await getCapturedMessages();
    const fetchScores = msgs2.find(m => m.type === 'fetchScores');
    assert(!!fetchScores, 'Switching to scores tab should send fetchScores message');

    // Switch back to agent for remaining tests
    await page.click('.tab-btn[data-tab="agent"]');
    await new Promise(r => setTimeout(r, 100));
}

async function test_07_CommandInput() {
    await clearMessages();

    // Type a command and press Enter
    await page.click('#commandInput');
    await page.type('#commandInput', 'Refactor the auth module');
    await page.keyboard.press('Enter');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const commandMsg = msgs.find(m => m.type === 'command');
    assert(!!commandMsg, 'Enter should send a command message');
    assertEqual(commandMsg.text, 'Refactor the auth module', 'Command text');

    // Input should be cleared
    const inputVal = await page.$eval('#commandInput', el => el.value);
    assertEqual(inputVal, '', 'Command input should be cleared after Enter');
}

async function test_08_CommandInputEmptyReject() {
    await clearMessages();

    // Press Enter with empty input — should NOT send
    await page.click('#commandInput');
    await page.keyboard.press('Enter');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const commandMsg = msgs.find(m => m.type === 'command');
    assert(!commandMsg, 'Empty input should NOT send a command message');
}

async function test_09_ApproveButton() {
    await clearMessages();
    await page.click('#btnApprove');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const approveMsg = msgs.find(m => m.type === 'approve');
    assert(!!approveMsg, 'Approve button should send approve message');
}

async function test_10_RejectButton() {
    await clearMessages();
    await page.click('#btnReject');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const rejectMsg = msgs.find(m => m.type === 'reject');
    assert(!!rejectMsg, 'Reject button should send reject message');
}

async function test_11_AbortButton() {
    await clearMessages();
    await page.click('#btnAbort');
    await new Promise(r => setTimeout(r, 100));

    const msgs = await getCapturedMessages();
    const abortMsg = msgs.find(m => m.type === 'abort');
    assert(!!abortMsg, 'Abort button should send abort message');
}

async function test_12_PlanRendering() {
    const plan = {
        id: 'test-task-1',
        steps: [
            { description: 'Analyze code structure', status: 'done' },
            { description: 'Identify refactoring targets', status: 'active' },
            { description: 'Apply changes', status: '' },
            { description: 'Run tests', status: 'failed' },
        ]
    };

    await injectMessage({ type: 'updatePlan', plan });

    // Verify correct number of list items
    const itemCount = await page.$$eval('#planList .plan-item', items => items.length);
    assertEqual(itemCount, 4, 'Plan should have 4 items');

    // Verify first item has done status
    const firstItem = await page.$eval('#planList .plan-item:nth-child(1)', el => ({
        text: el.textContent,
        classes: el.className
    }));
    assertIncludes(firstItem.text, 'Analyze code structure', 'First step text');
    assertIncludes(firstItem.text, '✓', 'Done step should have checkmark');

    // Verify active step
    const activeItem = await page.$eval('#planList .plan-item:nth-child(2)', el => el.textContent);
    assertIncludes(activeItem, '▸', 'Active step should have arrow');

    // Verify failed step
    const failedItem = await page.$eval('#planList .plan-item:nth-child(4)', el => el.textContent);
    assertIncludes(failedItem, '✗', 'Failed step should have X');
}

async function test_13_LogRendering() {
    // Count initial log entries
    const initialCount = await page.$$eval('#logContainer .log-entry', items => items.length);

    // Inject info log
    await injectMessage({ type: 'log', text: 'Task started successfully', level: 'info' });
    const afterInfo = await page.$$eval('#logContainer .log-entry', items => items.length);
    assertEqual(afterInfo, initialCount + 1, 'Should add one log entry');

    const lastLog = await page.$eval('#logContainer .log-entry:last-child .log-msg', el => ({
        text: el.textContent,
        classes: el.className
    }));
    assertIncludes(lastLog.text, 'Task started successfully', 'Log text');
    assertIncludes(lastLog.classes, 'info', 'Log level class');

    // Inject error log
    await injectMessage({ type: 'log', text: 'Something went wrong', level: 'error' });
    const errorLog = await page.$eval('#logContainer .log-entry:last-child .log-msg', el => ({
        text: el.textContent,
        classes: el.className
    }));
    assertIncludes(errorLog.text, 'Something went wrong', 'Error log text');
    assertIncludes(errorLog.classes, 'error', 'Error log level class');

    // Inject success log
    await injectMessage({ type: 'log', text: 'All done!', level: 'success' });
    const successLog = await page.$eval('#logContainer .log-entry:last-child .log-msg', el => el.className);
    assertIncludes(successLog, 'success', 'Success log level class');

    // Inject warning log
    await injectMessage({ type: 'log', text: 'Watch out!', level: 'warning' });
    const warningLog = await page.$eval('#logContainer .log-entry:last-child .log-msg', el => el.className);
    assertIncludes(warningLog, 'warning', 'Warning log level class');
}

async function test_14_LogXSSPrevention() {
    await injectMessage({ type: 'log', text: '<script>alert("xss")</script>', level: 'info' });

    const lastLog = await page.$eval('#logContainer .log-entry:last-child .log-msg', el => el.innerHTML);
    assert(!lastLog.includes('<script>'), 'Log should escape HTML to prevent XSS');
    assertIncludes(lastLog, '&lt;script&gt;', 'HTML should be escaped');
}

async function test_15_StreamRendering() {
    // Inject stream chunks
    await injectMessage({ type: 'stream', requestId: 'stream-1', chunk: 'Hello ' });
    await injectMessage({ type: 'stream', requestId: 'stream-1', chunk: 'World' });

    // Verify stream content is appended
    const streamContent = await page.$eval('#logContainer .log-entry:last-child .stream-content', el => el.textContent);
    assertEqual(streamContent, 'Hello World', 'Stream chunks should be concatenated');

    // New stream ID should create a new element
    await injectMessage({ type: 'stream', requestId: 'stream-2', chunk: 'New stream' });
    const newStreamContent = await page.$eval('#logContainer .log-entry:last-child .stream-content', el => el.textContent);
    assertEqual(newStreamContent, 'New stream', 'New stream ID should create new element');
}

async function test_16_HistoryRendering() {
    // Switch to history tab
    await page.click('.tab-btn[data-tab="history"]');
    await new Promise(r => setTimeout(r, 100));

    const historyData = [
        {
            description: 'Refactor auth module',
            status: 'completed',
            steps: 5,
            duration_ms: 12500,
            created_at: Math.floor(Date.now() / 1000) - 300 // 5 min ago
        },
        {
            description: 'Fix login bug',
            status: 'failed',
            steps: 3,
            duration_ms: 8000,
            created_at: Math.floor(Date.now() / 1000) - 7200 // 2h ago
        },
        {
            description: 'Add logging',
            status: 'running',
            steps: 2,
            duration_ms: 0,
            created_at: Math.floor(Date.now() / 1000) - 30 // 30s ago
        }
    ];

    await injectMessage({ type: 'historyData', data: historyData });

    // Verify items rendered
    const itemCount = await page.$$eval('#historyList .timeline-item', items => items.length);
    assertEqual(itemCount, 3, 'Should render 3 history items');

    // Verify first item content
    const firstItem = await page.$eval('#historyList .timeline-item:first-child', el => ({
        text: el.textContent,
        statusClass: el.querySelector('.timeline-status').className
    }));
    assertIncludes(firstItem.text, 'Refactor auth module', 'First history item description');
    assertIncludes(firstItem.text, '5 steps', 'First history item steps');
    assertIncludes(firstItem.statusClass, 'completed', 'First item should have completed status');

    // Verify failed item
    const failedItem = await page.$eval('#historyList .timeline-item:nth-child(2) .timeline-status', el => el.className);
    assertIncludes(failedItem, 'failed', 'Second item should have failed status');
}

async function test_17_HistoryEmptyState() {
    await injectMessage({ type: 'historyData', data: [] });

    const emptyState = await page.$eval('#historyList', el => el.innerHTML);
    assertIncludes(emptyState, 'empty-state', 'Empty history should show empty state');
    assertIncludes(emptyState, 'No task history', 'Empty state should have correct text');
}

async function test_18_ScoresRendering() {
    await page.click('.tab-btn[data-tab="scores"]');
    await new Promise(r => setTimeout(r, 100));

    const scoresData = {
        overall: 8.5,
        completeness: 9.0,
        efficiency: 7.5,
        quality: 8.0
    };

    await injectMessage({ type: 'scoresData', data: scoresData });

    // Verify score cards rendered
    const scoreCards = await page.$$eval('#scoresGrid .score-card', cards => cards.length);
    assertEqual(scoreCards, 4, 'Should render 4 score cards (overall + 3 categories)');

    // Verify overall score
    const overallValue = await page.$eval('#scoresGrid .score-overall .score-value', el => el.textContent);
    assertEqual(overallValue.trim(), '8.5', 'Overall score value');

    // Verify score class (8.5 should be "high")
    const overallClass = await page.$eval('#scoresGrid .score-overall .score-value', el => el.className);
    assertIncludes(overallClass, 'high', 'Overall score 8.5 should have "high" class');

    // Verify feedback text
    const feedback = await page.$eval('#scoresGrid .score-overall .score-feedback', el => el.textContent);
    assertIncludes(feedback, 'Great', '8.5 should show Great feedback');
}

async function test_19_ScoresEmptyState() {
    await injectMessage({ type: 'scoresData', data: {} });

    const emptyState = await page.$eval('#scoresGrid', el => el.innerHTML);
    assertIncludes(emptyState, 'empty-state', 'Empty scores should show empty state');
}

async function test_20_ScoreClassBoundaries() {
    // Test all score class boundaries
    const testCases = [
        { score: 9.5, expectedClass: 'high', expectedFeedback: 'Exceptional' },
        { score: 8.0, expectedClass: 'high', expectedFeedback: 'Great' },
        { score: 7.0, expectedClass: 'mid', expectedFeedback: 'Good' },
        { score: 5.5, expectedClass: 'low', expectedFeedback: 'Improving' },
        { score: 3.0, expectedClass: 'danger', expectedFeedback: 'Needs work' },
    ];

    for (const tc of testCases) {
        // Note: sub-scores must be non-zero because renderScores treats 0 as falsy
        // and shows the empty state when all three are falsy.
        await injectMessage({ type: 'scoresData', data: { overall: tc.score, completeness: 1, efficiency: 1, quality: 1 } });
        const scoreClass = await page.$eval('#scoresGrid .score-overall .score-value', el => el.className);
        assertIncludes(scoreClass, tc.expectedClass, `Score ${tc.score} class`);
        const feedback = await page.$eval('#scoresGrid .score-overall .score-feedback', el => el.textContent);
        assertIncludes(feedback, tc.expectedFeedback, `Score ${tc.score} feedback`);
    }
}

async function test_21_StrategiesRendering() {
    await page.click('.tab-btn[data-tab="strategies"]');
    await new Promise(r => setTimeout(r, 100));

    const strategies = [
        { task_type: 'Refactoring', use_count: 12, success_rate: 0.85, avg_score: 8.2 },
        { task_type: 'Bug Fix', use_count: 8, success_rate: 0.75, avg_score: 7.0 },
        { task_type: 'Feature', use_count: 3, success_rate: 0.33, avg_score: 4.5 },
    ];

    await injectMessage({ type: 'strategiesData', data: strategies });

    const itemCount = await page.$$eval('#strategyList .strategy-item', items => items.length);
    assertEqual(itemCount, 3, 'Should render 3 strategy items');

    // Verify first item
    const first = await page.$eval('#strategyList .strategy-item:first-child', el => el.textContent);
    assertIncludes(first, '#1', 'First strategy should be ranked #1');
    assertIncludes(first, 'Refactoring', 'First strategy type');
    assertIncludes(first, '12', 'First strategy use count');
    assertIncludes(first, '85%', 'First strategy success rate');
}

async function test_22_StrategiesEmptyState() {
    await injectMessage({ type: 'strategiesData', data: [] });

    const emptyState = await page.$eval('#strategyList', el => el.innerHTML);
    assertIncludes(emptyState, 'empty-state', 'Empty strategies should show empty state');
    assertIncludes(emptyState, 'No strategies', 'Empty state text');
}

async function test_23_LessonsRendering() {
    await page.click('.tab-btn[data-tab="lessons"]');
    await new Promise(r => setTimeout(r, 100));

    const lessons = [
        { category: 'error-handling', pattern: 'nil check', lesson: 'Always check for nil before dereferencing' },
        { category: 'performance', pattern: 'loop optimization', lesson: 'Use buffered channels for batch processing' },
        { category: 'error-handling', pattern: 'retry logic', lesson: 'Implement exponential backoff' },
    ];

    await injectMessage({ type: 'lessonsData', data: lessons });

    // Verify lesson cards
    const cardCount = await page.$$eval('#lessonList .lesson-card', cards => cards.length);
    assertEqual(cardCount, 3, 'Should render 3 lesson cards');

    // Verify first card content
    const firstCard = await page.$eval('#lessonList .lesson-card:first-child', el => el.textContent);
    assertIncludes(firstCard, 'error-handling', 'First lesson category');
    assertIncludes(firstCard, 'nil check', 'First lesson pattern');
    assertIncludes(firstCard, 'Always check for nil', 'First lesson text');

    // Verify filter buttons were created
    const filterCount = await page.$$eval('#lessonFilters .filter-btn', btns => btns.length);
    assert(filterCount >= 3, `Should have at least 3 filter buttons (All + 2 categories), got ${filterCount}`);
}

async function test_24_LessonFiltering() {
    // Click on error-handling filter
    const buttons = await page.$$('#lessonFilters .filter-btn');
    let errorFilterFound = false;
    for (const btn of buttons) {
        const filter = await btn.evaluate(el => el.dataset.filter);
        if (filter === 'error-handling') {
            await btn.click();
            errorFilterFound = true;
            break;
        }
    }
    assert(errorFilterFound, 'error-handling filter button should exist');
    await new Promise(r => setTimeout(r, 200));

    // Should now show only 2 cards (2 error-handling lessons)
    const filteredCount = await page.$$eval('#lessonList .lesson-card', cards => cards.length);
    assertEqual(filteredCount, 2, 'Filtering by error-handling should show 2 cards');

    // Click "All" to reset
    const allBtn = await page.$('#lessonFilters .filter-btn[data-filter="all"]');
    if (allBtn) {
        await allBtn.click();
        await new Promise(r => setTimeout(r, 200));
        const allCount = await page.$$eval('#lessonList .lesson-card', cards => cards.length);
        assertEqual(allCount, 3, 'All filter should show all 3 cards');
    }
}

async function test_25_LessonsEmptyState() {
    await injectMessage({ type: 'lessonsData', data: [] });

    const emptyState = await page.$eval('#lessonList', el => el.innerHTML);
    assertIncludes(emptyState, 'empty-state', 'Empty lessons should show empty state');
    assertIncludes(emptyState, 'No lessons', 'Empty state text');
}

async function test_26_RefreshButtons() {
    // Switch to history tab and test refresh button
    await page.click('.tab-btn[data-tab="history"]');
    await new Promise(r => setTimeout(r, 100));
    await clearMessages();

    const refreshHistoryBtn = await page.$('#btnRefreshHistory');
    if (refreshHistoryBtn) {
        await refreshHistoryBtn.click();
        await new Promise(r => setTimeout(r, 100));
        const msgs = await getCapturedMessages();
        const fetchMsg = msgs.find(m => m.type === 'fetchHistory');
        assert(!!fetchMsg, 'Refresh history button should send fetchHistory message');
    }

    // Switch to scores and test refresh
    await page.click('.tab-btn[data-tab="scores"]');
    await new Promise(r => setTimeout(r, 100));
    await clearMessages();

    const refreshScoresBtn = await page.$('#btnRefreshScores');
    if (refreshScoresBtn) {
        await refreshScoresBtn.click();
        await new Promise(r => setTimeout(r, 100));
        const msgs = await getCapturedMessages();
        const fetchMsg = msgs.find(m => m.type === 'fetchScores');
        assert(!!fetchMsg, 'Refresh scores button should send fetchScores message');
    }
}

async function test_27_PlanReplacement() {
    // Switch back to agent tab
    await page.click('.tab-btn[data-tab="agent"]');
    await new Promise(r => setTimeout(r, 100));

    // Inject first plan
    await injectMessage({ type: 'updatePlan', plan: { steps: [{ description: 'Step A', status: '' }] } });
    let count = await page.$$eval('#planList .plan-item', items => items.length);
    assertEqual(count, 1, 'First plan should have 1 item');

    // Inject replacement plan
    await injectMessage({
        type: 'updatePlan', plan: {
            steps: [
                { description: 'Step X', status: 'done' },
                { description: 'Step Y', status: '' }
            ]
        }
    });
    count = await page.$$eval('#planList .plan-item', items => items.length);
    assertEqual(count, 2, 'Replacement plan should have 2 items');

    const firstText = await page.$eval('#planList .plan-item:first-child', el => el.textContent);
    assertIncludes(firstText, 'Step X', 'Replacement plan first item');
}

async function test_28_PlanWithSpecialCharacters() {
    const plan = {
        steps: [
            { description: 'Handle <script> tags & "quotes"', status: '' },
            { description: 'Process file: main.go', status: 'done' },
        ]
    };

    await injectMessage({ type: 'updatePlan', plan });

    // Verify HTML escaping
    const firstItem = await page.$eval('#planList .plan-item:first-child', el => el.innerHTML);
    assert(!firstItem.includes('<script>'), 'Plan should escape HTML tags');
}

async function test_29_MultipleRapidLogs() {
    const initialCount = await page.$$eval('#logContainer .log-entry', items => items.length);

    // Inject 20 rapid-fire log messages
    for (let i = 0; i < 20; i++) {
        await page.evaluate((idx) => {
            window.__injectMessage({ type: 'log', text: `Rapid log message ${idx}`, level: 'info' });
        }, i);
    }
    await new Promise(r => setTimeout(r, 300));

    const afterCount = await page.$$eval('#logContainer .log-entry', items => items.length);
    assertEqual(afterCount, initialCount + 20, 'All 20 rapid logs should be rendered');
}

async function test_30_DurationFormatting() {
    // Switch to history
    await page.click('.tab-btn[data-tab="history"]');
    await new Promise(r => setTimeout(r, 100));

    await injectMessage({
        type: 'historyData', data: [
            { description: 'Fast task', status: 'completed', steps: 1, duration_ms: 500, created_at: Math.floor(Date.now() / 1000) },
            { description: 'Medium task', status: 'completed', steps: 3, duration_ms: 15000, created_at: Math.floor(Date.now() / 1000) },
            { description: 'Long task', status: 'completed', steps: 10, duration_ms: 125000, created_at: Math.floor(Date.now() / 1000) },
        ]
    });

    const items = await page.$$eval('#historyList .timeline-item .timeline-meta', items =>
        items.map(el => el.textContent)
    );

    assertIncludes(items[0], '500ms', 'Sub-second duration');
    assertIncludes(items[1], '15.0s', 'Seconds duration');
    assertIncludes(items[2], '2.1m', 'Minutes duration');
}

async function test_31_ScoreBarWidths() {
    await page.click('.tab-btn[data-tab="scores"]');
    await new Promise(r => setTimeout(r, 100));

    await injectMessage({
        type: 'scoresData', data: {
            overall: 7.0, completeness: 10.0, efficiency: 0, quality: 5.0
        }
    });

    // Check that bar widths are proportional
    const widths = await page.$$eval('#scoresGrid .score-bar-fill', bars =>
        bars.map(b => b.style.width)
    );

    // overall=7.0 → 70%, completeness=10.0 → 100%, efficiency=0 → 0%, quality=5.0 → 50%
    assertEqual(widths[0], '70%', 'Overall bar width');
    assertEqual(widths[1], '100%', 'Completeness bar width');
    assertEqual(widths[2], '0%', 'Efficiency bar width');
    assertEqual(widths[3], '50%', 'Quality bar width');
}

async function test_32_FullWorkflow() {
    // Simulate a complete user workflow:
    // 1. Page loads → 2. Status online → 3. Type command → 4. See plan →
    // 5. Approve → 6. See logs → 7. Check history

    // Reload for clean state
    await loadPage();

    // 1. Status comes online
    await injectMessage({ type: 'status', status: 'online' });
    let statusText = await page.$eval('#statusText', el => el.innerText);
    assertEqual(statusText, 'ONLINE', 'Workflow: status online');

    // 2. User types command
    await clearMessages();
    await page.click('#commandInput');
    await page.type('#commandInput', 'Add unit tests for auth.go');
    await page.keyboard.press('Enter');
    const msgs = await getCapturedMessages();
    assert(msgs.some(m => m.type === 'command' && m.text === 'Add unit tests for auth.go'), 'Workflow: command sent');

    // 3. Plan arrives
    await injectMessage({ type: 'log', text: 'Planning task: Add unit tests for auth.go', level: 'info' });
    await injectMessage({
        type: 'updatePlan', plan: {
            id: 'task-42',
            steps: [
                { description: 'Analyze auth.go exports', status: 'done' },
                { description: 'Generate test cases', status: 'active' },
                { description: 'Write test file', status: '' }
            ]
        }
    });

    const planItems = await page.$$eval('#planList .plan-item', items => items.length);
    assertEqual(planItems, 3, 'Workflow: plan has 3 steps');

    // 4. User approves
    await clearMessages();
    await page.click('#btnApprove');
    const approveMsg = (await getCapturedMessages()).find(m => m.type === 'approve');
    assert(!!approveMsg, 'Workflow: approve sent');

    // 5. Execution logs arrive
    await injectMessage({ type: 'log', text: 'Executing step 1/3...', level: 'info' });
    await injectMessage({ type: 'log', text: 'Task executed successfully.', level: 'success' });

    // 6. Check history
    await page.click('.tab-btn[data-tab="history"]');
    await injectMessage({
        type: 'historyData', data: [
            { description: 'Add unit tests for auth.go', status: 'completed', steps: 3, duration_ms: 5200, created_at: Math.floor(Date.now() / 1000) }
        ]
    });

    const historyItems = await page.$$eval('#historyList .timeline-item', items => items.length);
    assertEqual(historyItems, 1, 'Workflow: history shows completed task');
}

// ─── Main ────────────────────────────────────────────────────────────────────

async function main() {
    const startTime = Date.now();

    try {
        await setup();
    } catch (err) {
        // Output minimal report if setup fails
        const report = {
            timestamp: new Date().toISOString(),
            summary: { total: 0, passed: 0, failed: 1, skipped: 0 },
            tests: [{ name: 'setup', passed: false, error: `Browser setup failed: ${err.message}` }],
            build_errors: [err.message]
        };
        console.log(JSON.stringify(report, null, 2));
        process.exit(1);
    }

    // Run all tests in order
    const tests = [
        ['Page loads without errors', test_01_PageLoads],
        ['Initial state is correct', test_02_InitialState],
        ['Status: online display', test_03_StatusOnline],
        ['Status: offline display', test_04_StatusOffline],
        ['Tab switching activates correct panels', test_05_TabSwitching],
        ['Tab switching triggers data fetch', test_06_TabSwitchingTriggersDataFetch],
        ['Command input sends message', test_07_CommandInput],
        ['Empty command input rejected', test_08_CommandInputEmptyReject],
        ['Approve button', test_09_ApproveButton],
        ['Reject button', test_10_RejectButton],
        ['Abort button', test_11_AbortButton],
        ['Plan rendering with all statuses', test_12_PlanRendering],
        ['Log rendering with all levels', test_13_LogRendering],
        ['Log XSS prevention', test_14_LogXSSPrevention],
        ['Stream chunk rendering', test_15_StreamRendering],
        ['History rendering with data', test_16_HistoryRendering],
        ['History empty state', test_17_HistoryEmptyState],
        ['Scores rendering with values', test_18_ScoresRendering],
        ['Scores empty state', test_19_ScoresEmptyState],
        ['Score class boundaries', test_20_ScoreClassBoundaries],
        ['Strategies rendering', test_21_StrategiesRendering],
        ['Strategies empty state', test_22_StrategiesEmptyState],
        ['Lessons rendering with filters', test_23_LessonsRendering],
        ['Lesson category filtering', test_24_LessonFiltering],
        ['Lessons empty state', test_25_LessonsEmptyState],
        ['Refresh buttons send messages', test_26_RefreshButtons],
        ['Plan replacement', test_27_PlanReplacement],
        ['Plan with special characters (XSS)', test_28_PlanWithSpecialCharacters],
        ['Rapid-fire log rendering', test_29_MultipleRapidLogs],
        ['Duration formatting', test_30_DurationFormatting],
        ['Score bar widths', test_31_ScoreBarWidths],
        ['Full user workflow (E2E)', test_32_FullWorkflow],
    ];

    for (const [name, fn] of tests) {
        await runTest(name, fn);
    }

    await teardown();

    // Build report
    const passed = results.filter(r => r.passed).length;
    const failed = results.filter(r => !r.passed).length;

    const report = {
        timestamp: new Date().toISOString(),
        duration: `${Date.now() - startTime}ms`,
        summary: {
            total: results.length,
            passed,
            failed,
            skipped: 0
        },
        tests: results
    };

    console.log(JSON.stringify(report, null, 2));

    // Also print human-readable summary to stderr
    console.error('');
    console.error('═══════════════════════════════════════════');
    console.error(`  GUI Tests: ${passed}/${results.length} passed`);
    if (failed > 0) {
        console.error('');
        console.error('  FAILURES:');
        for (const r of results.filter(r => !r.passed)) {
            console.error(`    ✗ ${r.name}`);
            console.error(`      ${r.error}`);
            if (r.screenshot) console.error(`      Screenshot: ${r.screenshot}`);
        }
    }
    console.error('═══════════════════════════════════════════');
    console.error('');

    process.exit(failed > 0 ? 1 : 0);
}

main().catch(err => {
    console.error(`Fatal: ${err.message}`);
    process.exit(2);
});
