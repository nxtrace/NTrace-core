const form = document.getElementById('trace-form');
const protocolSelect = document.getElementById('protocol');
const providerSelect = document.getElementById('data-provider');
const queriesInput = document.getElementById('queries');
const maxHopsInput = document.getElementById('max-hops');
const disableMaptraceInput = document.getElementById('disable-maptrace');
const statusNode = document.getElementById('status');
const resultNode = document.getElementById('result');
const resultMetaNode = document.getElementById('result-meta');
const submitBtn = document.getElementById('submit-btn');

function setStatus(state, message) {
  statusNode.className = `status status--${state}`;
  statusNode.textContent = message;
}

async function loadOptions() {
  try {
    const res = await fetch('/api/options');
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`);
    }
    const data = await res.json();
    fillSelect(protocolSelect, data.protocols, data.defaultOptions.protocol);
    fillSelect(providerSelect, data.dataProviders, data.defaultOptions.data_provider);
    queriesInput.value = data.defaultOptions.queries;
    maxHopsInput.value = data.defaultOptions.max_hops;
    disableMaptraceInput.checked = data.defaultOptions.disable_maptrace;
  } catch (err) {
    setStatus('error', `无法加载选项: ${err.message}`);
    submitBtn.disabled = true;
  }
}

function fillSelect(selectEl, values, defaultValue) {
  selectEl.innerHTML = '';
  values.forEach((val) => {
    const option = document.createElement('option');
    option.value = val;
    option.textContent = val;
    if (String(val).toLowerCase() === String(defaultValue).toLowerCase()) {
      option.selected = true;
    }
    selectEl.appendChild(option);
  });
}

function readNumericValue(inputEl) {
  const raw = inputEl.value.trim();
  if (raw === '') {
    return undefined;
  }
  const num = Number(raw);
  return Number.isFinite(num) ? num : undefined;
}

function clearResult() {
  resultNode.innerHTML = '';
  resultNode.classList.add('hidden');
  resultMetaNode.innerHTML = '';
  resultMetaNode.classList.add('hidden');
}

function renderMeta(response) {
  const rows = [];
  rows.push(`解析结果：<strong>${response.resolved_ip}</strong>`);
  rows.push(`数据源：<strong>${response.data_provider}</strong>`);
  rows.push(`耗时：<strong>${response.duration_ms} ms</strong>`);
  if (response.trace_map_url) {
    rows.push(`地图：<a href="${response.trace_map_url}" target="_blank" rel="noreferrer">打开地图</a>`);
  }
  resultMetaNode.innerHTML = rows.map((line) => `<div>${line}</div>`).join('');
  resultMetaNode.classList.remove('hidden');
}

function renderResult(response) {
  if (!response.hops || response.hops.length === 0) {
    resultNode.innerHTML = '<p>未获取到有效路由信息。</p>';
    resultNode.classList.remove('hidden');
    return;
  }

  const table = document.createElement('table');
  const thead = document.createElement('thead');
  thead.innerHTML = `
    <tr>
      <th>TTL</th>
      <th>探测详情</th>
    </tr>
  `;
  table.appendChild(thead);

  const tbody = document.createElement('tbody');
  response.hops.forEach((hop) => {
    const tr = document.createElement('tr');
    const ttlCell = document.createElement('td');
    ttlCell.textContent = hop.ttl;
    tr.appendChild(ttlCell);

    const attemptsCell = document.createElement('td');
    attemptsCell.appendChild(renderAttempts(hop.attempts));
    tr.appendChild(attemptsCell);

    tbody.appendChild(tr);
  });

  table.appendChild(tbody);
  resultNode.innerHTML = '';
  resultNode.appendChild(table);
  resultNode.classList.remove('hidden');
}

function renderAttempts(attempts) {
  const container = document.createElement('div');
  container.className = 'attempts';

  attempts.forEach((attempt, idx) => {
    const box = document.createElement('div');
    box.className = 'attempt';

    const badge = document.createElement('span');
    badge.className = 'attempt__badge';
    badge.textContent = `探测 ${idx + 1}`;
    if (!attempt.success) {
      badge.classList.add('attempt__badge--fail');
    }
    box.appendChild(badge);

    const meta = document.createElement('div');
    meta.className = 'attempt__meta';
    if (attempt.hostname) {
      meta.appendChild(createMetaItem('主机', attempt.hostname));
    }
    if (attempt.ip) {
      meta.appendChild(createMetaItem('地址', attempt.ip));
    }
    if (attempt.rtt_ms !== undefined && attempt.rtt_ms !== null) {
      meta.appendChild(createMetaItem('延迟', `${attempt.rtt_ms.toFixed(2)} ms`));
    }
    if (attempt.error) {
      meta.appendChild(createMetaItem('错误', attempt.error));
    }
    if (attempt.mpls && attempt.mpls.length > 0) {
      meta.appendChild(createMetaItem('MPLS', attempt.mpls.join(', ')));
    }
    box.appendChild(meta);

    if (attempt.geo && (attempt.geo.country || attempt.geo.owner || attempt.geo.asnumber)) {
      const geo = document.createElement('div');
      geo.className = 'attempt__geo';
      const segments = [];
      if (attempt.geo.asnumber) {
        segments.push(`AS${attempt.geo.asnumber}`);
      }
      if (attempt.geo.country) {
        segments.push(attempt.geo.country);
      }
      if (attempt.geo.prov) {
        segments.push(attempt.geo.prov);
      }
      if (attempt.geo.city) {
        segments.push(attempt.geo.city);
      }
      if (attempt.geo.owner || attempt.geo.isp) {
        segments.push(attempt.geo.owner || attempt.geo.isp);
      }
      geo.textContent = segments.join(' · ');
      box.appendChild(geo);
    }

    container.appendChild(box);
  });

  return container;
}

function createMetaItem(label, value) {
  const span = document.createElement('span');
  span.innerHTML = `<strong>${label}:</strong> ${value}`;
  return span;
}

function buildPayload() {
  const payload = {
    target: form.target.value.trim(),
    protocol: protocolSelect.value,
    data_provider: providerSelect.value,
    disable_maptrace: disableMaptraceInput.checked,
  };

  const queries = readNumericValue(queriesInput);
  if (queries !== undefined) {
    payload.queries = queries;
  }

  const maxHops = readNumericValue(maxHopsInput);
  if (maxHops !== undefined) {
    payload.max_hops = maxHops;
  }

  return payload;
}

async function runTrace(evt) {
  evt.preventDefault();
  clearResult();

  const payload = buildPayload();
  if (!payload.target) {
    setStatus('error', '请填写目标地址');
    return;
  }

  setStatus('running', '正在探测，请稍候...');
  submitBtn.disabled = true;

  try {
    const res = await fetch('/api/trace', {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const error = await res.json().catch(() => ({}));
      const message = error.error || `探测失败: HTTP ${res.status}`;
      throw new Error(message);
    }
    const data = await res.json();
    renderMeta(data);
    renderResult(data);
    setStatus('success', '探测完成');
  } catch (err) {
    setStatus('error', err.message);
  } finally {
    submitBtn.disabled = false;
  }
}

document.addEventListener('DOMContentLoaded', () => {
  loadOptions();
  form.addEventListener('submit', runTrace);
});
