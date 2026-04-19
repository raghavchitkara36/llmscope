/* llmscope dashboard — app.js */

const API = {
    projects: () => fetch('/api/projects').then(r => r.json()),
    stats: (pid) => fetch(`/api/stats?project_id=${pid}`).then(r => r.json()),
    traces: (pid, params) => fetch(`/api/traces?project_id=${pid}&${new URLSearchParams(params)}`).then(r => r.json()),
    trace: (id) => fetch(`/api/traces/${id}`).then(r => r.json()),
};

/* ── State ── */
const state = {
    projectID: '',
    currentPage: 1,
    pageSize: 20,
    totalCount: 0,
    filter: {},
};

/* ── DOM refs ── */
const $ = id => document.getElementById(id);
const $$ = sel => document.querySelectorAll(sel);

/* ── View routing ── */
function showView(name) {
    $$('.view').forEach(v => v.classList.remove('active'));
    $$('.nav-item').forEach(n => n.classList.remove('active'));

    const view = document.getElementById(`view-${name}`);
    if (view) view.classList.add('active');

    const navItem = document.querySelector(`.nav-item[data-view="${name}"]`);
    if (navItem) navItem.classList.add('active');

    if (name === 'overview') loadOverview();
    if (name === 'traces') loadTraces();
    if (name === 'errors') loadErrors();
}

/* ── Navigation ── */
$$('.nav-item').forEach(item => {
    item.addEventListener('click', () => showView(item.dataset.view));
});

$$('.view-all').forEach(el => {
    el.addEventListener('click', () => showView(el.dataset.view));
});

$('backBtn').addEventListener('click', () => showView('traces'));

/* ── Project selector ── */
async function loadProjects() {
    try {
        const projects = await API.projects();
        const sel = $('projectSelect');
        sel.innerHTML = '';

        if (!projects || projects.length === 0) {
            sel.innerHTML = '<option value="">No projects yet</option>';
            return;
        }

        projects.forEach(p => {
            const opt = document.createElement('option');
            opt.value = p.project_id;
            opt.textContent = p.project_name;
            sel.appendChild(opt);
        });

        state.projectID = projects[0].project_id;
        loadOverview();

    } catch (e) {
        console.error('Failed to load projects:', e);
        $('projectSelect').innerHTML = '<option value="">Error loading projects</option>';
    }
}

$('projectSelect').addEventListener('change', e => {
    state.projectID = e.target.value;
    state.currentPage = 1;
    state.totalCount = 0;
    showView('overview');
});

/* ── Overview ── */
async function loadOverview() {
    if (!state.projectID) return;

    try {
        const [stats, result] = await Promise.all([
            API.stats(state.projectID),
            API.traces(state.projectID, { limit: 8, offset: 0 }),
        ]);

        $('statTotalTraces').textContent = fmt.number(stats.total_traces);
        $('statAvgLatency').textContent = fmt.number(Math.round(stats.avg_latency_ms));
        $('statTotalCost').textContent = fmt.cost(stats.total_cost);
        $('statErrorCount').textContent = fmt.number(stats.error_count);
        $('statTotalTokens').textContent = fmt.number(stats.total_tokens);

        if (stats.start_time && stats.end_time) {
            $('overviewSubtitle').textContent =
                `${fmt.date(stats.start_time)} → ${fmt.date(stats.end_time)}`;
        }

        renderRecentTraces(result.traces || []);

    } catch (e) {
        console.error('Failed to load overview:', e);
    }
}

function renderRecentTraces(traces) {
    const container = $('recentTraces');

    if (!traces.length) {
        container.innerHTML = `
      <div class="empty-state" style="padding:20px;text-align:center;color:var(--text-3);font-family:var(--mono);font-size:12px;">
        No traces yet — make an LLM call to see data here
      </div>`;
        return;
    }

    container.innerHTML = traces.map(t => `
    <div class="trace-list-item ${t.error ? 'error' : ''}" onclick="openTrace('${t.trace_id}')">
      <span>${fmt.date(t.request_timestamp)}</span>
      <span>${t.provider}</span>
      <span style="color:var(--text);overflow:hidden;text-overflow:ellipsis;white-space:nowrap;">${t.model}</span>
      <span>${fmt.latency(t.request_timestamp, t.response_timestamp)}</span>
      <span>${t.error
            ? '<span class="badge badge-err">ERR</span>'
            : '<span class="badge badge-ok">OK</span>'}</span>
    </div>
  `).join('');
}

/* ── Traces View — server-side pagination ── */
async function loadTraces() {
    if (!state.projectID) return;

    const tbody = $('traceTableBody');
    tbody.innerHTML = '<tr><td colspan="7" class="empty-state">Loading...</td></tr>';

    try {
        const params = {
            ...state.filter,
            limit: state.pageSize,
            offset: (state.currentPage - 1) * state.pageSize,
        };

        const result = await API.traces(state.projectID, params);

        // server returns { traces, total, limit, offset }
        const traces = result.traces || [];
        state.totalCount = result.total || 0;

        renderTracesTable(traces);
        renderPagination();

    } catch (e) {
        console.error('Failed to load traces:', e);
        tbody.innerHTML = '<tr><td colspan="7" class="empty-state">Failed to load traces</td></tr>';
    }
}

function renderTracesTable(traces) {
    const tbody = $('traceTableBody');

    if (!traces.length) {
        tbody.innerHTML = '<tr><td colspan="7" class="empty-state">No traces found</td></tr>';
        return;
    }

    tbody.innerHTML = traces.map(t => `
    <tr class="${t.error ? 'error-row' : ''}" style="cursor:pointer">
      <td>${fmt.date(t.request_timestamp)}</td>
      <td>${t.provider}</td>
      <td>${t.model}</td>
      <td>${fmt.latency(t.request_timestamp, t.response_timestamp)}</td>
      <td>${((t.input_tokens || 0) + (t.output_tokens || 0)).toLocaleString()}</td>
      <td>${t.error
            ? '<span class="badge badge-err">ERROR</span>'
            : '<span class="badge badge-ok">OK</span>'}</td>
      <td><button class="btn-detail" onclick="openTrace('${t.trace_id}')">View →</button></td>
    </tr>
  `).join('');
}

function renderPagination() {
    const totalPages = Math.max(1, Math.ceil(state.totalCount / state.pageSize));
    $('pageInfo').textContent = `Page ${state.currentPage} of ${totalPages} — ${state.totalCount.toLocaleString()} traces`;
    $('prevPage').disabled = state.currentPage <= 1;
    $('nextPage').disabled = state.currentPage >= totalPages;
}

$('prevPage').addEventListener('click', () => {
    if (state.currentPage > 1) {
        state.currentPage--;
        loadTraces();
    }
});

$('nextPage').addEventListener('click', () => {
    const totalPages = Math.ceil(state.totalCount / state.pageSize);
    if (state.currentPage < totalPages) {
        state.currentPage++;
        loadTraces();
    }
});

$('applyFilter').addEventListener('click', () => {
    state.currentPage = 1;
    state.filter = {};

    const provider = $('filterProvider').value.trim();
    const model = $('filterModel').value.trim();
    const hasError = $('filterError').value;

    if (provider) state.filter.provider = provider;
    if (model) state.filter.model = model;
    if (hasError) state.filter.has_error = hasError;

    loadTraces();
});

/* ── Trace Detail ── */
async function openTrace(traceID) {
    try {
        const t = await API.trace(traceID);

        $('detailTraceID').textContent = t.trace_id;
        $('detailProvider').textContent = t.provider;
        $('detailModel').textContent = t.model;
        $('detailLatency').textContent = fmt.latency(t.request_timestamp, t.response_timestamp);
        $('detailInputTokens').textContent = (t.input_tokens || 0).toLocaleString();
        $('detailOutputTokens').textContent = (t.output_tokens || 0).toLocaleString();
        $('detailCost').textContent = t.cost_usd > 0 ? `$${t.cost_usd.toFixed(6)}` : '—';
        $('detailStatus').innerHTML = t.error
            ? '<span class="badge badge-err">ERROR</span>'
            : '<span class="badge badge-ok">OK</span>';

        // tags
        const tagsRow = $('detailTagsRow');
        if (t.tags && Object.keys(t.tags).length > 0) {
            $('detailTags').textContent = Object.entries(t.tags)
                .map(([k, v]) => `${k}=${v}`)
                .join(', ');
            tagsRow.style.display = 'flex';
        } else {
            tagsRow.style.display = 'none';
        }

        // prompt messages
        const promptEl = $('detailPrompt');
        if (t.prompt && t.prompt.length) {
            promptEl.innerHTML = t.prompt.map(m => `
        <div class="message-block">
          <div class="message-role role-${m.role}">${m.role.toUpperCase()}</div>
          <div class="message-content">${escapeHTML(m.content)}</div>
        </div>
      `).join('');
        } else {
            promptEl.textContent = '—';
        }

        // response
        $('detailResponse').textContent = t.response || '—';

        // error block
        const errorSection = $('detailErrorSection');
        if (t.error) {
            errorSection.style.display = 'block';
            $('detailError').textContent = `[${t.error.code || 'unknown'}] ${t.error.message}`;
        } else {
            errorSection.style.display = 'none';
        }

        showView('detail');

    } catch (e) {
        console.error('Failed to load trace:', e);
    }
}

/* ── Errors View ── */
async function loadErrors() {
    if (!state.projectID) return;

    const tbody = $('errorTableBody');
    tbody.innerHTML = '<tr><td colspan="5" class="empty-state">Loading...</td></tr>';

    try {
        const result = await API.traces(state.projectID, {
            has_error: 'true',
            limit: 100,
            offset: 0,
        });

        const errors = result.traces || [];

        if (!errors.length) {
            tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No errors — looking good!</td></tr>';
            return;
        }

        tbody.innerHTML = errors.map(t => `
      <tr style="cursor:pointer">
        <td>${fmt.date(t.request_timestamp)}</td>
        <td>${t.provider}</td>
        <td>${t.model}</td>
        <td style="color:var(--error);font-family:var(--mono);font-size:11px;">
          ${t.error ? escapeHTML(t.error.message) : '—'}
        </td>
        <td><button class="btn-detail" onclick="openTrace('${t.trace_id}')">View →</button></td>
      </tr>
    `).join('');

    } catch (e) {
        console.error('Failed to load errors:', e);
        tbody.innerHTML = '<tr><td colspan="5" class="empty-state">Failed to load errors</td></tr>';
    }
}

/* ── Formatters ── */
const fmt = {
    number: n => (n ?? 0).toLocaleString(),
    cost: n => n > 0 ? `$${Number(n).toFixed(4)}` : '$0.00',
    date: ms => {
        if (!ms) return '—';
        return new Date(ms).toLocaleString('en-US', {
            month: 'short', day: 'numeric',
            hour: '2-digit', minute: '2-digit', second: '2-digit',
        });
    },
    latency: (start, end) => {
        if (!start || !end) return '—';
        return `${(end - start).toLocaleString()}ms`;
    },
};

/* ── Helpers ── */
function escapeHTML(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;');
}

/* ── Boot ── */
loadProjects();