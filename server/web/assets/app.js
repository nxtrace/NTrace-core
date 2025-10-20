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
const langToggleBtn = document.getElementById('lang-toggle');
const cacheBtn = document.getElementById('cache-btn');
const titleText = document.getElementById('title-text');
const subtitleText = document.getElementById('subtitle-text');
const footerText = document.getElementById('footer-text');
const labelTarget = document.getElementById('label-target');
const labelProtocol = document.getElementById('label-protocol');
const labelProvider = document.getElementById('label-provider');
const labelQueries = document.getElementById('label-queries');
const labelMaxHops = document.getElementById('label-maxhops');
const labelDisableMap = document.getElementById('label-disable-map');
const targetInput = document.getElementById('target');

const wsScheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
const wsUrl = `${wsScheme}://${window.location.host}/ws/trace`;

let socket = null;
let traceCompleted = false;
const hopStore = new Map();
let latestSummary = {};
let currentLang = 'cn';
let currentStatus = {state: 'idle', key: 'statusReady', custom: null};

const uiText = {
  cn: {
    title: 'NextTrace Web',
    subtitle: '在浏览器中运行 NextTrace，实时查看路由探测结果。',
    labelTarget: '目标地址',
    placeholderTarget: '例如：1.1.1.1 或 www.example.com',
    labelProtocol: '协议',
    labelProvider: '地理信息源',
    labelQueries: '每跳探测次数',
    labelMaxHops: '最大跳数',
    labelDisableMap: '禁用地图生成',
    buttonStart: '开始探测',
    buttonClearCache: '清空缓存',
    langToggle: 'English',
    tableTTL: 'TTL',
    tableDetails: '探测详情',
    statusReady: '准备就绪',
    statusRunning: '正在探测，请稍候...',
    statusSuccess: '探测完成',
    statusCacheClearing: '正在清理缓存…',
    statusCacheCleared: '缓存已清空',
    statusCacheFailed: '清理缓存失败',
    statusWsError: 'WebSocket 连接出错',
    statusDisconnected: '连接已断开',
    statusOptionsFailed: '无法加载选项:',
    statusTargetMissing: '请填写目标地址',
    statusTraceFailed: '探测失败',
    metaResolved: '解析结果',
    metaProvider: '数据源',
    metaDuration: '耗时',
    metaMap: '地图',
    mapOpen: '打开地图',
    attemptLabelHost: '主机',
    attemptLabelAddress: '地址',
    attemptLabelLatency: '延迟',
    attemptLabelError: '错误',
    attemptLabelMPLS: 'MPLS',
    attemptLabelLoss: '丢包率',
    attemptLabelFailure: '失败',
    timeoutAll: '全部超时',
    timeoutPartial: '部分超时',
    unknownAddress: '未知地址',
    unknownError: '未知错误',
    attemptBadge: '探测',
    noResult: '未获取到有效路由信息。',
    footer: '当前会话仅提供基础功能，更多高级选项请使用 CLI。',
  },
  en: {
    title: 'NextTrace Web',
    subtitle: 'Run NextTrace in your browser and watch the trace in real time.',
    labelTarget: 'Target',
    placeholderTarget: 'e.g. 1.1.1.1 or www.example.com',
    labelProtocol: 'Protocol',
    labelProvider: 'Geo provider',
    labelQueries: 'Probes per hop',
    labelMaxHops: 'Max hops',
    labelDisableMap: 'Disable map generation',
    buttonStart: 'Start Trace',
    buttonClearCache: 'Clear Cache',
    langToggle: '中文',
    tableTTL: 'TTL',
    tableDetails: 'Details',
    statusReady: 'Ready',
    statusRunning: 'Tracing…',
    statusSuccess: 'Trace completed',
    statusCacheClearing: 'Clearing cache…',
    statusCacheCleared: 'Cache cleared',
    statusCacheFailed: 'Failed to clear cache',
    statusWsError: 'WebSocket error',
    statusDisconnected: 'Connection closed',
    statusOptionsFailed: 'Failed to load options:',
    statusTargetMissing: 'Please enter a target',
    statusTraceFailed: 'Trace failed',
    metaResolved: 'Resolved IP',
    metaProvider: 'Provider',
    metaDuration: 'Duration',
    metaMap: 'Map',
    mapOpen: 'Open map',
    attemptLabelHost: 'Host',
    attemptLabelAddress: 'IP',
    attemptLabelLatency: 'Latency',
    attemptLabelError: 'Error',
    attemptLabelMPLS: 'MPLS',
    attemptLabelLoss: 'Loss',
    attemptLabelFailure: 'Failure',
    timeoutAll: 'All timeout',
    timeoutPartial: 'Partial timeout',
    unknownAddress: 'Unknown',
    unknownError: 'Unknown error',
    attemptBadge: 'Probe',
    noResult: 'No valid hops collected yet.',
    footer: 'For advanced options, please use the CLI.',
  },
};

function t(key) {
  return uiText[currentLang][key] || key || '';
}

function updateStatusDisplay(state, text) {
  statusNode.className = `status status--${state}`;
  statusNode.textContent = text;
}

function setStatus(state, message, translate = true) {
  if (translate) {
    currentStatus = {state, key: message, custom: null};
    updateStatusDisplay(state, t(message));
  } else {
    currentStatus = {state, key: null, custom: message};
    updateStatusDisplay(state, message);
  }
}

function refreshStatus() {
  if (currentStatus.key) {
    updateStatusDisplay(currentStatus.state, t(currentStatus.key));
  } else if (currentStatus.custom !== null) {
    updateStatusDisplay(currentStatus.state, currentStatus.custom);
  }
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
    setStatus('error', `${t('statusOptionsFailed')} ${err.message}`, false);
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

function clearResult(resetState = false) {
  resultNode.innerHTML = '';
  resultNode.classList.add('hidden');
  resultMetaNode.innerHTML = '';
  resultMetaNode.classList.add('hidden');
  if (resetState) {
    hopStore.clear();
    latestSummary = {};
  }
}

function renderMeta(summary = {}) {
  const rows = [];
  if (summary.resolved_ip) {
    rows.push(`${t('metaResolved')}：<strong>${summary.resolved_ip}</strong>`);
  }
  if (summary.data_provider) {
    rows.push(`${t('metaProvider')}：<strong>${summary.data_provider}</strong>`);
  }
  if (summary.duration_ms !== undefined) {
    rows.push(`${t('metaDuration')}：<strong>${summary.duration_ms} ms</strong>`);
  }
  if (summary.trace_map_url) {
    rows.push(`${t('metaMap')}：<a href="${summary.trace_map_url}" target="_blank" rel="noreferrer">${t('mapOpen')}</a>`);
  }
  if (rows.length === 0) {
    resultMetaNode.classList.add('hidden');
    resultMetaNode.innerHTML = '';
    return;
  }
  resultMetaNode.innerHTML = rows.map((line) => `<div>${line}</div>`).join('');
  resultMetaNode.classList.remove('hidden');
}

function renderAttemptsGrouped(attempts) {
  const groups = new Map();
  let lastKey = null;
  attempts.forEach((attempt, index) => {
    const keyHost = attempt.hostname || '';
    const keyIp = attempt.ip || '';
    let key;
    if (keyHost || keyIp) {
      key = keyHost + '|' + keyIp;
      lastKey = key;
    } else if (lastKey) {
      key = lastKey;
    } else {
      key = 'unknown|';
      lastKey = key;
    }
    if (!groups.has(key)) {
      groups.set(key, []);
    }
    groups.get(key).push(attempt);
  });

  const container = document.createElement('div');
  container.className = 'attempts attempts--grouped';

  groups.forEach((list, key) => {
    const box = document.createElement('div');
    box.className = 'attempt attempt--group';

    const header = document.createElement('div');
    header.className = 'attempt__header';
    const mainLine = [];
    const first = list[0] || {};
    const [keyHost, keyIp] = key.split('|');
    const displayHost = first.hostname || keyHost || '';
    const displayIp = first.ip || keyIp || '';
    if (displayHost) {
      mainLine.push(createMetaItem(t('attemptLabelHost'), displayHost));
    }
    if (displayIp) {
      mainLine.push(createMetaItem(t('attemptLabelAddress'), displayIp));
    }
    if (mainLine.length === 0) {
      mainLine.push(createMetaItem(t('attemptLabelAddress'), t('unknownAddress')));
    }
    mainLine.forEach((el) => header.appendChild(el));

    box.appendChild(header);

    const metrics = document.createElement('div');
    metrics.className = 'attempt__meta';
    const rtts = list
      .filter((item) => item.rtt_ms !== undefined && item.rtt_ms !== null)
      .map((item) => Number(item.rtt_ms));
    if (rtts.length > 0) {
      const min = Math.min(...rtts).toFixed(2);
      const max = Math.max(...rtts).toFixed(2);
      const avg = (rtts.reduce((sum, v) => sum + v, 0) / rtts.length).toFixed(2);
      metrics.appendChild(createMetaItem(t('attemptLabelLatency'), avg + ' ms (min ' + min + ', max ' + max + ')'));
    }
    const successes = list.filter((item) => item.success).length;
    const lossCount = list.length - successes;
    const lossRate = list.length > 0 ? (((lossCount) / list.length) * 100).toFixed(0) : '0';
    metrics.appendChild(createMetaItem(t('attemptLabelLoss'), lossRate + '% (' + lossCount + '/' + list.length + ')'));

    const failureItems = list.filter((item) => !item.success);
    if (failureItems.length > 0) {
      const failureGroups = new Map();
      failureItems.forEach((item) => {
        const raw = (item.error || '').trim();
        const key = raw || 'unknown';
        if (!failureGroups.has(key)) {
          failureGroups.set(key, {label: raw, count: 0, timeout: raw.toLowerCase().includes('timeout')});
        }
        const entry = failureGroups.get(key);
        entry.count += 1;
      });

      const entries = Array.from(failureGroups.values());
      const allTimeout = entries.length > 0 && entries.every((entry) => entry.timeout || entry.label === '');
      let failureText = '';
      if (allTimeout) {
        failureText = (successes === 0 ? t('timeoutAll') : t('timeoutPartial')) + ' (' + failureItems.length + '/' + list.length + ')';
      } else {
        failureText = entries
          .map((entry) => {
            let label = entry.label || t('unknownError');
            return label + ' (' + entry.count + '/' + list.length + ')';
          })
          .join(' | ');
      }
      metrics.appendChild(createMetaItem(t('attemptLabelFailure'), failureText));
    }

    const mplsAll = list.flatMap((item) => item.mpls || []);
    if (mplsAll.length > 0) {
      metrics.appendChild(createMetaItem(t('attemptLabelMPLS'), Array.from(new Set(mplsAll)).join(', ')));
    }
    box.appendChild(metrics);

    const geoLine = document.createElement('div');
    geoLine.className = 'attempt__geo';
    const segments = [];
    if (first.geo) {
      if (first.geo.asnumber) {
        segments.push('AS' + first.geo.asnumber);
      }
      if (first.geo.country) {
        segments.push(first.geo.country);
      }
      if (first.geo.prov) {
        segments.push(first.geo.prov);
      }
      if (first.geo.city) {
        segments.push(first.geo.city);
      }
      if (first.geo.owner || first.geo.isp) {
        segments.push(first.geo.owner || first.geo.isp);
      }
    }
    if (segments.length > 0) {
      geoLine.textContent = segments.join(' · ');
      box.appendChild(geoLine);
    }

    const probes = document.createElement('div');
    probes.className = 'attempt__probes';
    list.forEach((item, index) => {
      const badge = document.createElement('span');
      badge.className = 'attempt__badge';
      badge.textContent = t('attemptBadge') + ' ' + (index + 1);
      if (!item.success) {
        badge.classList.add('attempt__badge--fail');
      }
      probes.appendChild(badge);
    });
    box.appendChild(probes);

    container.appendChild(box);
  });

  return container;
}

function renderHops(hops) {
  if (!hops || hops.length === 0) {
    resultNode.innerHTML = `<p>${t('noResult')}</p>`;
    resultNode.classList.remove('hidden');
    return;
  }

  const table = document.createElement('table');
  const thead = document.createElement('thead');
  thead.innerHTML = `
    <tr>
      <th>${t('tableTTL')}</th>
      <th>${t('tableDetails')}</th>
    </tr>
  `;
  table.appendChild(thead);

  const tbody = document.createElement('tbody');
  hops.forEach((hop) => {
    const tr = document.createElement('tr');
    const ttlCell = document.createElement('td');
    ttlCell.textContent = hop.ttl;
    tr.appendChild(ttlCell);

    const attemptsCell = document.createElement('td');
    attemptsCell.appendChild(renderAttemptsGrouped(hop.attempts));
    tr.appendChild(attemptsCell);

    tbody.appendChild(tr);
  });

  table.appendChild(tbody);
  resultNode.innerHTML = '';
  resultNode.appendChild(table);
  resultNode.classList.remove('hidden');
}

function renderHopsFromStore() {
  const hops = Array.from(hopStore.values()).sort((a, b) => a.ttl - b.ttl);
  renderHops(hops);
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
    language: currentLang,
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

function closeExistingSocket() {
  if (socket) {
    socket.onclose = null;
    socket.onerror = null;
    try {
      socket.close();
    } catch (_) {
      // ignore
    }
    socket = null;
  }
}

function handleSocketMessage(event) {
  let msg;
  try {
    msg = JSON.parse(event.data);
  } catch (err) {
    setStatus('error', err.message, false);
    return;
  }

  switch (msg.type) {
    case 'start': {
      latestSummary = {...latestSummary, ...msg.data};
      renderMeta(latestSummary);
      break;
    }
    case 'hop': {
      if (msg.data && typeof msg.data.ttl === 'number') {
        hopStore.set(msg.data.ttl, msg.data);
        renderHopsFromStore();
      }
      break;
    }
    case 'complete': {
      traceCompleted = true;
      submitBtn.disabled = false;
      if (msg.data && Array.isArray(msg.data.hops)) {
        hopStore.clear();
        msg.data.hops.forEach((hop) => {
          if (hop && typeof hop.ttl === 'number') {
            hopStore.set(hop.ttl, hop);
          }
        });
      }
      latestSummary = {...latestSummary, ...msg.data};
      renderMeta(latestSummary);
      renderHopsFromStore();
      setStatus('success', 'statusSuccess');
      closeExistingSocket();
      break;
    }
    case 'error': {
      traceCompleted = true;
      submitBtn.disabled = false;
      const text = msg.error || t('statusTraceFailed');
      setStatus('error', text, !msg.error);
      closeExistingSocket();
      break;
    }
    default:
      break;
  }
}

function runTrace(evt) {
  evt.preventDefault();
  clearResult(true);

  const payload = buildPayload();
  if (!payload.target) {
    setStatus('error', 'statusTargetMissing');
    return;
  }

  setStatus('running', 'statusRunning');
  submitBtn.disabled = true;
  traceCompleted = false;

  closeExistingSocket();

  try {
    socket = new WebSocket(wsUrl);
  } catch (err) {
    setStatus('error', `${t('statusWsError')} ${err.message}`, false);
    submitBtn.disabled = false;
    return;
  }

  socket.onopen = () => {
    socket.send(JSON.stringify(payload));
  };

  socket.onmessage = handleSocketMessage;

  socket.onerror = () => {
    if (!traceCompleted) {
      traceCompleted = true;
      setStatus('error', 'statusWsError');
      submitBtn.disabled = false;
    }
  };

  socket.onclose = () => {
    if (!traceCompleted) {
      setStatus('error', 'statusDisconnected');
      submitBtn.disabled = false;
    }
    socket = null;
  };
}

async function clearCache(silent = false) {
  if (!silent) {
    setStatus('running', 'statusCacheClearing');
  }
  try {
    const res = await fetch('/api/cache/clear', {method: 'POST'});
    if (!res.ok) {
      const errRes = await res.json().catch(() => ({}));
      const message = errRes.error || `${t('statusCacheFailed')} HTTP ${res.status}`;
      throw new Error(message);
    }
    if (!silent) {
      setStatus('success', 'statusCacheCleared');
    } else {
      setStatus('idle', 'statusReady');
    }
  } catch (err) {
    setStatus('error', err.message || t('statusCacheFailed'), false);
  }
}

function toggleLanguage() {
  currentLang = currentLang === 'cn' ? 'en' : 'cn';
  applyTranslations();
  clearCache(true);
}

function applyTranslations() {
  titleText.textContent = t('title');
  subtitleText.textContent = t('subtitle');
  footerText.textContent = t('footer');
  labelTarget.textContent = t('labelTarget');
  labelProtocol.textContent = t('labelProtocol');
  labelProvider.textContent = t('labelProvider');
  labelQueries.textContent = t('labelQueries');
  labelMaxHops.textContent = t('labelMaxHops');
  labelDisableMap.textContent = t('labelDisableMap');
  targetInput.placeholder = t('placeholderTarget');
  submitBtn.textContent = t('buttonStart');
  cacheBtn.textContent = t('buttonClearCache');
  langToggleBtn.textContent = t('langToggle');
  renderMeta(latestSummary);
  renderHopsFromStore();
  refreshStatus();
}

document.addEventListener('DOMContentLoaded', () => {
  applyTranslations();
  setStatus('idle', 'statusReady');
  loadOptions();
  form.addEventListener('submit', runTrace);
  langToggleBtn.addEventListener('click', toggleLanguage);
  cacheBtn.addEventListener('click', () => clearCache(false));
  providerSelect.addEventListener('change', () => clearCache(true));
});
