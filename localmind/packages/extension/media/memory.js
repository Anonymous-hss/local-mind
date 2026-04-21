/* eslint-disable no-undef */
const vscode = acquireVsCodeApi();

window.addEventListener('load', () => {
    // Request initial data
    vscode.postMessage({ type: 'refresh' });

    // Refresh button
    const refreshBtn = document.getElementById('btnRefresh');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', () => {
            vscode.postMessage({ type: 'refresh' });
        });
    }

    // Add entry toggle
    const addBtn = document.getElementById('btnAddEntry');
    const addForm = document.getElementById('addEntryForm');
    if (addBtn && addForm) {
        addBtn.addEventListener('click', () => {
            addForm.classList.toggle('visible');
            if (addForm.classList.contains('visible')) {
                addForm.querySelector('input')?.focus();
            }
        });
    }

    // Add entry submit
    const submitBtn = document.getElementById('btnSubmitEntry');
    if (submitBtn) {
        submitBtn.addEventListener('click', submitNewEntry);
    }

    // Handle Enter in add-entry form
    document.querySelectorAll('.add-entry-form .form-input').forEach(input => {
        input.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                submitNewEntry();
            }
        });
    });

    // Handle messages from extension
    window.addEventListener('message', event => {
        const message = event.data;
        switch (message.type) {
            case 'updateStats':
                updateStats(message.stats);
                break;
            case 'updateProgress':
                updateProgress(message.progress);
                break;
            case 'error':
                console.error(message.error);
                break;
        }
    });

    // Tag removal delegation
    document.addEventListener('click', (e) => {
        if (e.target.classList.contains('tag-remove')) {
            const tag = e.target.closest('.tag');
            if (tag) {
                const category = tag.dataset.category;
                const value = tag.dataset.value;
                tag.style.opacity = '0';
                tag.style.transform = 'scale(0.8)';
                setTimeout(() => {
                    vscode.postMessage({ type: 'removeTag', category, value });
                }, 200);
            }
        }
    });
});

function submitNewEntry() {
    const category = document.getElementById('entryCategory')?.value?.trim();
    const key = document.getElementById('entryKey')?.value?.trim();
    const value = document.getElementById('entryValue')?.value?.trim();

    if (!category || !key || !value) {
        return;
    }

    vscode.postMessage({
        type: 'addEntry',
        category,
        key,
        value,
    });

    // Clear form
    document.getElementById('entryCategory').value = '';
    document.getElementById('entryKey').value = '';
    document.getElementById('entryValue').value = '';
    document.getElementById('addEntryForm')?.classList.remove('visible');
}

function updateStats(stats) {
    // Tech stack tags
    const techList = document.getElementById('techStackList');
    if (techList) {
        techList.innerHTML = renderTags(stats.techStack || [], 'techStack');
    }

    // Architecture
    const archEl = document.getElementById('architectureType');
    if (archEl) {
        archEl.innerText = stats.architecture || 'Unknown';
    }

    // Entry points
    const epEl = document.getElementById('entryPoints');
    if (epEl) {
        if (stats.entryPoints && stats.entryPoints.length > 0) {
            epEl.innerHTML = stats.entryPoints.map(ep =>
                `<span class="tag tag-entry">${escapeHtml(ep)}</span>`
            ).join(' ');
        } else {
            epEl.innerText = 'None detected';
        }
    }

    // Conventions
    const convList = document.getElementById('conventionsList');
    if (convList) {
        convList.innerHTML = renderConventions(stats.conventions || []);
    }

    // Stats
    const totalEl = document.getElementById('totalFiles');
    if (totalEl) {
        totalEl.innerText = `${stats.totalFiles || 0} files indexed`;
    }

    const scanEl = document.getElementById('lastScan');
    if (scanEl) {
        scanEl.innerText = stats.lastScan || 'Never';
    }

    // Confidence overview
    const confEl = document.getElementById('confidenceOverview');
    if (confEl && stats.confidenceSummary) {
        confEl.innerHTML = renderConfidenceSummary(stats.confidenceSummary);
    }
}

function updateProgress(progress) {
    const bar = document.getElementById('scanProgressBar');
    const text = document.getElementById('scanStatusText');

    if (bar) {
        bar.style.width = `${progress}%`;
        if (progress > 0 && progress < 100) {
            bar.classList.add('active');
        } else {
            bar.classList.remove('active');
        }
    }

    if (text) {
        if (progress <= 0) {
            text.innerText = 'Error';
        } else if (progress < 100) {
            text.innerText = `Scanning... ${progress}%`;
        } else {
            text.innerText = 'Idle';
        }
    }
}

function renderTags(items, category) {
    if (!items || items.length === 0) {
        return '<span class="empty-state">None detected</span>';
    }
    return items.map(item => `
        <span class="tag" data-category="${escapeHtml(category)}" data-value="${escapeHtml(item)}"
              style="transition: all 0.25s ease">
            ${escapeHtml(item)}
            <span class="tag-remove">&times;</span>
        </span>
    `).join('');
}

function renderConventions(items) {
    if (!items || items.length === 0) {
        return '<li class="stat-item"><span class="stat-label">None detected</span></li>';
    }
    return items.map(item => {
        const conf = item.confidence || 0;
        const dotClass = conf >= 0.9 ? 'confidence-high'
            : conf >= 0.5 ? 'confidence-medium'
                : conf > 0 ? 'confidence-low'
                    : 'confidence-unknown';

        return `
            <li class="stat-item">
                <span class="stat-label">
                    <span class="confidence-dot ${dotClass}"></span>
                    ${escapeHtml(item.key)}
                </span>
                <span class="stat-value">${escapeHtml(item.value)}</span>
            </li>
        `;
    }).join('');
}

function renderConfidenceSummary(summary) {
    const total = (summary.detected || 0) + (summary.inferred || 0) + (summary.override || 0) + (summary.unknown || 0);
    if (total === 0) return '';

    const items = [];
    if (summary.detected) items.push(`<span class="card-badge badge-detected">${summary.detected} detected</span>`);
    if (summary.inferred) items.push(`<span class="card-badge badge-inferred">${summary.inferred} inferred</span>`);
    if (summary.override) items.push(`<span class="card-badge badge-override">${summary.override} override</span>`);

    return items.join(' ');
}

function escapeHtml(str) {
    if (!str) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}
