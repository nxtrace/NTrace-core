const form = document.getElementById('trace-form');
const protocolSelect = document.getElementById('protocol');
const providerSelect = document.getElementById('data-provider');
const queriesInput = document.getElementById('queries');
const maxHopsInput = document.getElementById('max-hops');
const disableMaptraceInput = document.getElementById('disable-maptrace');
const dstPortHint = document.getElementById('dst-port-hint');
const dstPortInput = document.getElementById('dst-port');
const payloadSizeInput = document.getElementById('payload-size');
const modeSelect = document.getElementById('mode');
const statusNode = document.getElementById('status');
const resultNode = document.getElementById('result');
const resultMetaNode = document.getElementById('result-meta');
const submitBtn = document.getElementById('submit-btn');
const stopBtn = document.getElementById('stop-btn');
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
const labelDstPort = document.getElementById('label-dst-port');
const labelPSize = document.getElementById('label-psize');
const labelMode = document.getElementById('label-mode');
const targetInput = document.getElementById('target');
const groupBasicParams = document.getElementById('group-basic-params');
const groupAdvancedParams = document.getElementById('group-advanced-params');
const groupDisableMap = document.getElementById('group-disable-map');

const wsScheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
const wsUrl = `${wsScheme}://${window.location.host}/ws/trace`;

let socket = null;
let traceCompleted = false;
const hopStore = new Map();
let latestSummary = {};
let currentLang = 'cn';
let currentMode = 'single';
let currentStatus = {state: 'idle', key: 'statusReady', custom: null};
let mtrStatsStore = [];
let mtrRawAggStore = new Map();
let mtrRawOrderSeq = 0;
let singleModeQueriesValue = '';
const MTR_RENDER_MIN_INTERVAL_MS = 100;
let mtrRenderScheduled = false;
let mtrRenderTimer = null;
let mtrRenderRAF = null;
let mtrRenderLastAt = 0;

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
    labelDstPort: '目的端口',
    labelPSize: '负载大小',
    labelMode: '探测模式',
    buttonStartSingle: '开始探测',
    buttonStartMtr: '开始持续探测',
    buttonStop: '停止',
    buttonClearCache: '清空缓存',
    langToggle: 'English',
    tableTTL: 'TTL',
    tableDetails: '探测详情',
    colLoss: '丢包率',
    colSent: '发送/接收',
    colLast: '最新',
    colAvg: '平均',
    colBest: '最佳',
    colWorst: '最差',
    colHost: '主机',
    colIP: '地址',
    colFailure: '失败原因',
    statusReady: '准备就绪',
    statusRunning: '正在探测，请稍候...',
    statusMtrRunning: '持续探测中…',
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
    metaIterations: '持续轮次',
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
    hintDstPort: '仅 TCP/UDP 模式有效',
    attemptBadge: '探测',
    noResult: '未获取到有效路由信息。',
    footer: '当前会话仅提供基础功能，更多高级选项请使用 CLI。',
    modeSingle: '单次探测',
    modeMTR: '持续探测',
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
    labelDstPort: 'Destination Port',
    labelPSize: 'Payload Size',
    labelMode: 'Mode',
    buttonStartSingle: 'Start Trace',
    buttonStartMtr: 'Start Continuous Trace',
    buttonStop: 'Stop',
    buttonClearCache: 'Clear Cache',
    langToggle: '中文',
    tableTTL: 'TTL',
    tableDetails: 'Details',
    colLoss: 'Loss',
    colSent: 'Sent/Recv',
    colLast: 'Last',
    colAvg: 'Avg',
    colBest: 'Best',
    colWorst: 'Worst',
    colHost: 'Host',
    colIP: 'IP',
    colFailure: 'Failure',
    statusReady: 'Ready',
    statusRunning: 'Tracing…',
    statusMtrRunning: 'Tracing continuously…',
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
    metaIterations: 'Iterations',
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
    hintDstPort: 'Active for TCP/UDP only',
    attemptBadge: 'Probe',
    noResult: 'No valid hops collected yet.',
    footer: 'For advanced options, please use the CLI.',
    modeSingle: 'Single Trace',
    modeMTR: 'Continuous Trace',
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
    queriesInput.value = Math.min(63, data.defaultOptions.queries || 3);
    singleModeQueriesValue = queriesInput.value;
    queriesInput.dataset.defaultValue = queriesInput.value;
    maxHopsInput.value = data.defaultOptions.max_hops;
    disableMaptraceInput.checked = data.defaultOptions.disable_maptrace;
    payloadSizeInput.value = data.defaultOptions.packet_size || payloadSizeInput.value || 52;
    dstPortInput.value = data.defaultOptions.port || dstPortInput.value || '';
    updateDstPortState();
    updateModeUI();
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
  cancelScheduledMTRRender();
  resultNode.innerHTML = '';
  resultNode.classList.add('hidden');
  resultMetaNode.innerHTML = '';
  resultMetaNode.classList.add('hidden');
  if (resetState) {
    hopStore.clear();
    latestSummary = {};
    mtrStatsStore = [];
    mtrRawAggStore = new Map();
    mtrRawOrderSeq = 0;
    mtrRenderLastAt = 0;
    stopBtn.classList.add('hidden');
    stopBtn.disabled = true;
  }
}

function renderMeta(summary = {}) {
  const rows = [];
  if (summary.resolved_ip) {
    rows.push(`${t('metaResolved')}：<strong>${escapeHTML(summary.resolved_ip)}</strong>`);
  }
  if (summary.data_provider) {
    rows.push(`${t('metaProvider')}：<strong>${escapeHTML(summary.data_provider)}</strong>`);
  }
  if (summary.duration_ms !== undefined) {
    rows.push(`${t('metaDuration')}：<strong>${escapeHTML(summary.duration_ms)} ms</strong>`);
  }
  if (summary.iteration) {
    rows.push(`${t('metaIterations')}：<strong>${escapeHTML(summary.iteration)}</strong>`);
  }
  if (summary.trace_map_url) {
    // t('mapOpen') is assumed not user-supplied; escape only the URL
    rows.push(`${t('metaMap')}：<a href="${escapeHTML(summary.trace_map_url)}" target="_blank" rel="noreferrer">${t('mapOpen')}</a>`);
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
  const UNKNOWN_KEY = '__unknown__';
  const groups = new Map();
  const ipIndex = new Map();
  const hostIndex = new Map();
  let pendingUnknown = [];
  let lastGroup = null;

  const createGroup = (key) => {
    const group = {
      key,
      attempts: [],
      hosts: new Set(),
      ips: new Set(),
      firstHost: '',
      firstIP: '',
    };
    groups.set(key, group);
    return group;
  };

  attempts.forEach((attempt) => {
    const hostRaw = (attempt.hostname || '').trim();
    const hostKey = hostRaw.toLowerCase();
    const ip = (attempt.ip || '').trim();

    if (!hostRaw && !ip) {
      if (lastGroup) {
        lastGroup.attempts.push(attempt);
      } else {
        pendingUnknown.push(attempt);
      }
      return;
    }

    let group = null;
    if (ip && ipIndex.has(ip)) {
      group = groups.get(ipIndex.get(ip));
    }
    if (!group && hostRaw) {
      if (hostIndex.has(hostKey)) {
        group = groups.get(hostIndex.get(hostKey));
      }
    }

    if (!group) {
      const key = ip ? `ip:${ip}` : `host:${hostKey}`;
      group = createGroup(key);
    }

    if (pendingUnknown.length > 0) {
      group.attempts.push(...pendingUnknown);
      pendingUnknown = [];
    }

    group.attempts.push(attempt);

    if (ip) {
      group.ips.add(ip);
      if (!group.firstIP) {
        group.firstIP = ip;
      }
      ipIndex.set(ip, group.key);
    }
    if (hostRaw) {
      group.hosts.add(hostRaw);
      if (!group.firstHost) {
        group.firstHost = hostRaw;
      }
      if (hostKey) {
        hostIndex.set(hostKey, group.key);
      }
    }

    lastGroup = group;
  });

  if (pendingUnknown.length > 0) {
    const group = createGroup(UNKNOWN_KEY);
    group.attempts.push(...pendingUnknown);
  }

  const orderedGroups = Array.from(groups.values()).filter((group) => group.attempts.length > 0);

  const container = document.createElement('div');
  container.className = 'attempts attempts--grouped';

  let hasIdentifiedSummary = false;
  const summarySet = new Set();
  const summaryLabels = [];
  orderedGroups.forEach((group) => {
    const displayIp = group.firstIP || '';
    let displayHost = group.firstHost || '';
    if (displayHost && displayIp && displayHost === displayIp) {
      displayHost = '';
    }
    let label = '';
    if (displayIp && displayHost && displayHost !== displayIp) {
      label = `${displayIp} (${displayHost})`;
    } else if (displayIp) {
      label = displayIp;
    } else if (displayHost) {
      label = displayHost;
    } else {
      label = '*';
    }
    if (!summarySet.has(label)) {
      summarySet.add(label);
      summaryLabels.push(label);
    }
    if (displayIp || displayHost) {
      hasIdentifiedSummary = true;
    }
  });

  if (summaryLabels.length > 1) {
    const summary = document.createElement('div');
    summary.className = 'attempts__summary';
    summary.textContent = summaryLabels.join(' | ');
    container.appendChild(summary);
  }

  orderedGroups.forEach((group) => {
    const box = document.createElement('div');
    box.className = 'attempt attempt--group';

    const header = document.createElement('div');
    header.className = 'attempt__header';
    const mainLine = [];
    const first = group.attempts[0] || {};
    let displayHost = group.firstHost || '';
    const displayIp = group.firstIP || '';
    if (displayHost && displayIp && displayHost === displayIp) {
      displayHost = '';
    }
    if (displayHost) {
      mainLine.push(createMetaItem(t('attemptLabelHost'), displayHost));
    }
    if (displayIp) {
      mainLine.push(createMetaItem(t('attemptLabelAddress'), displayIp));
    }
    if (mainLine.length === 0) {
      if (hasIdentifiedSummary) {
        const label = document.createElement('span');
        label.className = 'attempt__star';
        label.textContent = '*';
        header.appendChild(label);
      } else {
        const star = document.createElement('span');
        star.className = 'attempt__star';
        star.textContent = '*';
        header.appendChild(star);
      }
    } else {
      mainLine.forEach((el) => header.appendChild(el));
    }

    box.appendChild(header);

    const metrics = document.createElement('div');
    metrics.className = 'attempt__meta';
    const rtts = group.attempts
      .filter((item) => item.rtt_ms !== undefined && item.rtt_ms !== null)
      .map((item) => Number(item.rtt_ms));
    if (rtts.length > 0) {
      const min = Math.min(...rtts).toFixed(2);
      const max = Math.max(...rtts).toFixed(2);
      const avg = (rtts.reduce((sum, v) => sum + v, 0) / rtts.length).toFixed(2);
      metrics.appendChild(createMetaItem(t('attemptLabelLatency'), avg + ' ms (min ' + min + ', max ' + max + ')'));
    }
    const successes = group.attempts.filter((item) => item.success).length;
    const lossCount = group.attempts.length - successes;
    const lossRate = group.attempts.length > 0 ? (((lossCount) / group.attempts.length) * 100).toFixed(0) : '0';
    metrics.appendChild(createMetaItem(t('attemptLabelLoss'), lossRate + '% (' + lossCount + '/' + group.attempts.length + ')'));

    const mplsAll = group.attempts.flatMap((item) => item.mpls || []);
    if (mplsAll.length > 0) {
      const unique = Array.from(new Set(mplsAll.map((entry) => String(entry || '').trim()).filter(Boolean)));
      if (unique.length > 0) {
        const mplsContainer = document.createElement('div');
        mplsContainer.className = 'attempt__mpls';
        unique.forEach((entry) => {
          const line = document.createElement('div');
          line.textContent = entry;
          mplsContainer.appendChild(line);
        });
        metrics.appendChild(mplsContainer);
      }
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
    group.attempts.forEach((item, index) => {
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

function escapeHTML(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

function createMetaItem(label, value, allowHTML = false) {
  const span = document.createElement('span');
  const safeLabel = escapeHTML(label);
  const strValue = value === undefined || value === null ? '' : String(value);
  if (allowHTML) {
    span.innerHTML = `<strong>${safeLabel}:</strong> ${strValue}`;
  } else {
    span.innerHTML = `<strong>${safeLabel}:</strong> ${escapeHTML(strValue)}`;
  }
  return span;
}

function buildPayload() {
  const payload = {
    target: form.target.value.trim(),
    protocol: protocolSelect.value,
    data_provider: providerSelect.value,
    disable_maptrace: disableMaptraceInput.checked,
    language: currentLang,
    mode: modeSelect.value || 'single',
  };

  const isMtrMode = payload.mode === 'mtr';
  if (isMtrMode) {
    payload.queries = 10;
    if (queriesInput.value !== '10') {
      queriesInput.value = '10';
    }
  } else {
    const queries = readNumericValue(queriesInput);
    if (queries !== undefined) {
      payload.queries = Math.max(1, Math.min(63, queries));
    }
  }

  const maxHops = readNumericValue(maxHopsInput);
  if (maxHops !== undefined) {
    payload.max_hops = maxHops;
  }

  if (payload.mode === 'mtr') {
    payload.interval_ms = 2000;
    payload.max_rounds = 0;
  }
  const dstPort = readNumericValue(dstPortInput);
  if (dstPort !== undefined) {
    payload.port = dstPort;
  }

  const psize = readNumericValue(payloadSizeInput);
  if (psize !== undefined) {
    payload.packet_size = psize;
  }

  return payload;
}

function closeExistingSocket(hideStop = true) {
  cancelScheduledMTRRender();
  if (socket) {
    socket.onclose = null;
    socket.onerror = null;
    try {
      socket.close(1000, 'client stop');
    } catch (_) {
      // ignore
    }
    socket = null;
  }
  if (hideStop) {
    stopBtn.classList.add('hidden');
    stopBtn.disabled = true;
  }
}

function flushMTRRender(force = false) {
  if (mtrRenderTimer !== null) {
    clearTimeout(mtrRenderTimer);
    mtrRenderTimer = null;
  }
  if (mtrRenderRAF !== null && typeof cancelAnimationFrame === 'function') {
    cancelAnimationFrame(mtrRenderRAF);
    mtrRenderRAF = null;
  }
  if (!force) {
    const now = Date.now();
    const elapsed = now - mtrRenderLastAt;
    if (elapsed < MTR_RENDER_MIN_INTERVAL_MS) {
      const waitMs = MTR_RENDER_MIN_INTERVAL_MS - elapsed;
      mtrRenderScheduled = true;
      mtrRenderTimer = setTimeout(() => {
        mtrRenderTimer = null;
        flushMTRRender();
      }, waitMs);
      return;
    }
  }
  mtrRenderScheduled = false;
  mtrRenderLastAt = Date.now();
  renderMTRStats(buildMTRStatsFromRawAgg());
  renderMeta(latestSummary);
}

function scheduleMTRRender() {
  if (mtrRenderScheduled) {
    return;
  }
  mtrRenderScheduled = true;

  const attemptRender = () => {
    mtrRenderRAF = null;
    const waitMs = Math.max(0, MTR_RENDER_MIN_INTERVAL_MS - (Date.now() - mtrRenderLastAt));
    if (waitMs > 0) {
      mtrRenderTimer = setTimeout(() => {
        mtrRenderTimer = null;
        flushMTRRender();
      }, waitMs);
      return;
    }
    flushMTRRender();
  };

  if (typeof requestAnimationFrame === 'function') {
    mtrRenderRAF = requestAnimationFrame(attemptRender);
    return;
  }
  mtrRenderTimer = setTimeout(() => {
    mtrRenderTimer = null;
    flushMTRRender();
  }, 0);
}

function cancelScheduledMTRRender() {
  if (mtrRenderTimer !== null) {
    clearTimeout(mtrRenderTimer);
    mtrRenderTimer = null;
  }
  if (mtrRenderRAF !== null && typeof cancelAnimationFrame === 'function') {
    cancelAnimationFrame(mtrRenderRAF);
    mtrRenderRAF = null;
  }
  mtrRenderScheduled = false;
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
      if (currentMode !== 'mtr' && msg.data && typeof msg.data.ttl === 'number') {
        hopStore.set(msg.data.ttl, msg.data);
        renderHopsFromStore();
      }
      break;
    }
    case 'mtr': {
      // Backward compatibility with old server snapshots.
      traceCompleted = false;
      cancelScheduledMTRRender();
      if (msg.data && typeof msg.data.iteration === 'number') {
        latestSummary = {...latestSummary, iteration: msg.data.iteration};
      }
      if (msg.data && Array.isArray(msg.data.stats)) {
        renderMTRStats(msg.data.stats);
      } else {
        renderMTRStats([]);
      }
      setStatus('running', 'statusMtrRunning');
      stopBtn.disabled = false;
      renderMeta(latestSummary);
      break;
    }
    case 'mtr_raw': {
      traceCompleted = false;
      if (msg.data) {
        ingestMTRRawRecord(msg.data);
        const it = Number(msg.data.iteration);
        if (Number.isFinite(it) && it > 0) {
          latestSummary = {...latestSummary, iteration: it};
        }
      }
      setStatus('running', 'statusMtrRunning');
      stopBtn.disabled = false;
      scheduleMTRRender();
      break;
    }
    case 'complete': {
      traceCompleted = true;
      submitBtn.disabled = false;
      if (currentMode === 'mtr') {
        if (msg.data && typeof msg.data.iteration === 'number') {
          latestSummary = {...latestSummary, iteration: msg.data.iteration};
        }
        stopBtn.disabled = true;
        stopBtn.classList.add('hidden');
        if (msg.data && Array.isArray(msg.data.stats)) {
          cancelScheduledMTRRender();
          renderMTRStats(msg.data.stats);
          renderMeta(latestSummary);
        } else {
          flushMTRRender(true);
        }
      } else {
        if (msg.data && Array.isArray(msg.data.hops)) {
          hopStore.clear();
          msg.data.hops.forEach((hop) => {
            if (hop && typeof hop.ttl === 'number') {
              hopStore.set(hop.ttl, hop);
            }
          });
        }
        latestSummary = {...latestSummary, ...msg.data};
        renderHopsFromStore();
        renderMeta(latestSummary);
      }
      setStatus('success', 'statusSuccess');
      closeExistingSocket();
      break;
    }
    case 'error': {
      traceCompleted = true;
      submitBtn.disabled = false;
      stopBtn.disabled = true;
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
  cancelScheduledMTRRender();
  clearResult(true);
  mtrRawAggStore = new Map();
  mtrRawOrderSeq = 0;

  const payload = buildPayload();
  if (!payload.target) {
    setStatus('error', 'statusTargetMissing');
    return;
  }

  currentMode = payload.mode || 'single';
  document.body.classList.toggle('mode-mtr', currentMode === 'mtr');
  updateStartButtonText();
  if (currentMode === 'mtr') {
    setStatus('running', 'statusMtrRunning');
    stopBtn.classList.remove('hidden');
    stopBtn.disabled = true;
  } else {
    setStatus('running', 'statusRunning');
    stopBtn.classList.add('hidden');
    stopBtn.disabled = true;
  }

  submitBtn.disabled = true;
  traceCompleted = false;

  closeExistingSocket(false);

  try {
    socket = new WebSocket(wsUrl);
  } catch (err) {
    setStatus('error', `${t('statusWsError')} ${err.message}`, false);
    submitBtn.disabled = false;
    updateModeUI();
    return;
  }

  socket.onopen = () => {
    if (currentMode === 'mtr') {
      stopBtn.disabled = false;
    }
    socket.send(JSON.stringify(payload));
  };

  socket.onmessage = handleSocketMessage;

  socket.onerror = () => {
    cancelScheduledMTRRender();
    if (!traceCompleted) {
      traceCompleted = true;
      setStatus('error', 'statusWsError');
      submitBtn.disabled = false;
      stopBtn.disabled = true;
    }
  };

  socket.onclose = () => {
    cancelScheduledMTRRender();
    if (!traceCompleted) {
      setStatus('error', 'statusDisconnected');
      submitBtn.disabled = false;
    }
    stopBtn.disabled = true;
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
  currentMode = modeSelect.value || 'single';
  titleText.textContent = t('title');
  subtitleText.textContent = t('subtitle');
  footerText.textContent = t('footer');
  labelTarget.textContent = t('labelTarget');
  labelProtocol.textContent = t('labelProtocol');
  labelProvider.textContent = t('labelProvider');
  labelQueries.textContent = t('labelQueries');
  labelMaxHops.textContent = t('labelMaxHops');
  labelDisableMap.textContent = t('labelDisableMap');
  labelDstPort.textContent = t('labelDstPort');
  labelPSize.textContent = t('labelPSize');
  labelMode.textContent = t('labelMode');
  dstPortHint.textContent = t('hintDstPort');
  targetInput.placeholder = t('placeholderTarget');
  updateStartButtonText();
  cacheBtn.textContent = t('buttonClearCache');
  langToggleBtn.textContent = t('langToggle');
  stopBtn.textContent = t('buttonStop');
  const options = modeSelect.options;
  if (options.length >= 2) {
    options[0].textContent = t('modeSingle');
    options[1].textContent = t('modeMTR');
  }
  const isMtr = currentMode === 'mtr';
  document.body.classList.toggle('mode-mtr', isMtr);
  groupBasicParams.classList.toggle('hidden', isMtr);
  groupAdvancedParams.classList.toggle('hidden', isMtr);
  groupDisableMap.classList.toggle('hidden', isMtr);
  renderMeta(latestSummary);
  if (currentMode === 'mtr') {
    renderMTRStats(mtrStatsStore);
  } else {
    renderHopsFromStore();
  }
  refreshStatus();
  updateModeUI();
  updateDstPortState();
}


function updateDstPortState() {
  const proto = (protocolSelect.value || '').toLowerCase();
  const enabled = proto === 'tcp' || proto === 'udp';
  dstPortInput.disabled = !enabled;
  dstPortInput.parentElement.classList.toggle('disabled', !enabled);
  if (!enabled) {
    dstPortInput.value = '';
  } else if (!dstPortInput.value) {
    dstPortInput.value = proto === 'tcp' ? '80' : '33494';
  }
}

document.addEventListener('DOMContentLoaded', () => {
  applyTranslations();
  updateModeUI();
  setStatus('idle', 'statusReady');
  loadOptions();
  form.addEventListener('submit', runTrace);
  langToggleBtn.addEventListener('click', toggleLanguage);
  cacheBtn.addEventListener('click', () => clearCache(false));
  providerSelect.addEventListener('change', () => clearCache(true));
  protocolSelect.addEventListener('change', () => {
    updateDstPortState();
    clearCache(true);
  });
  payloadSizeInput.addEventListener('change', () => clearCache(true));
  queriesInput.addEventListener('input', () => {
    if (!queriesInput.disabled) {
      singleModeQueriesValue = queriesInput.value;
    }
  });
  modeSelect.addEventListener('change', updateModeUI);
  stopBtn.addEventListener('click', stopTrace);
});

function updateStartButtonText() {
  if (currentMode === 'mtr') {
    submitBtn.textContent = t('buttonStartMtr');
  } else {
    submitBtn.textContent = t('buttonStartSingle');
  }
}

function updateModeUI() {
  currentMode = modeSelect.value || 'single';
  const isMtr = currentMode === 'mtr';
  document.body.classList.toggle('mode-mtr', isMtr);
  groupBasicParams.classList.toggle('hidden', isMtr);
  groupAdvancedParams.classList.toggle('hidden', isMtr);
  groupDisableMap.classList.toggle('hidden', isMtr);
  updateStartButtonText();

  const queriesContainer = queriesInput.parentElement;

  if (isMtr) {
    if (!queriesInput.disabled) {
      const currentValue = queriesInput.value.trim();
      if (currentValue) {
        singleModeQueriesValue = currentValue;
      } else if (!singleModeQueriesValue && queriesInput.dataset.defaultValue) {
        singleModeQueriesValue = queriesInput.dataset.defaultValue;
      }
    }
    queriesInput.value = '10';
    queriesInput.disabled = true;
    if (queriesContainer) {
      queriesContainer.classList.add('disabled');
    }
    stopBtn.classList.remove('hidden');
    stopBtn.disabled = true;
  } else {
    queriesInput.disabled = false;
    if (queriesContainer) {
      queriesContainer.classList.remove('disabled');
    }
    const restoreValue = singleModeQueriesValue || queriesInput.dataset.defaultValue || queriesInput.value || '3';
    queriesInput.value = restoreValue;
    stopBtn.classList.add('hidden');
    stopBtn.disabled = true;
  }
}

function stopTrace() {
  if (!socket) {
    stopBtn.disabled = true;
    stopBtn.classList.add('hidden');
    return;
  }
  traceCompleted = true;
  stopBtn.disabled = true;
  closeExistingSocket();
  submitBtn.disabled = false;
  setStatus('idle', 'statusReady');
}

function mtrRawKey(rec) {
  const ttl = Number(rec && rec.ttl);
  const ip = rec && rec.ip ? String(rec.ip).trim() : '';
  const host = rec && rec.host ? String(rec.host).trim().toLowerCase() : '';
  if (ip) {
    return `${ttl}|ip:${ip}`;
  }
  if (host) {
    return `${ttl}|host:${host}`;
  }
  return `${ttl}|unknown`;
}

function onlyTimeoutErrors(errors) {
  if (!errors) {
    return true;
  }
  const keys = Object.keys(errors);
  if (keys.length === 0) {
    return true;
  }
  return keys.every((k) => String(k).toLowerCase().includes('timeout'));
}

function recomputeMTRRawDerived(row) {
  row.loss_count = Math.max(0, row.sent - row.received);
  row.loss_percent = row.sent > 0 ? (row.loss_count / row.sent) * 100 : 0;
  row.avg_ms = row.received > 0 ? row._sum_ms / row.received : 0;
  if (row.loss_count <= 0) {
    row.failure_type = '';
  } else if (onlyTimeoutErrors(row.errors)) {
    row.failure_type = row.received > 0 ? 'partial_timeout' : 'all_timeout';
  } else {
    row.failure_type = 'mixed';
  }
}

function ingestMTRRawRecord(rec) {
  if (!rec || !Number.isFinite(Number(rec.ttl))) {
    return;
  }
  const key = mtrRawKey(rec);
  let row = mtrRawAggStore.get(key);
  if (!row) {
    row = {
      ttl: Number(rec.ttl),
      host: '',
      ip: '',
      sent: 0,
      received: 0,
      loss_percent: 0,
      loss_count: 0,
      last_ms: 0,
      avg_ms: 0,
      best_ms: 0,
      worst_ms: 0,
      geo: null,
      failure_type: '',
      errors: null,
      mpls: [],
      _sum_ms: 0,
      _order: mtrRawOrderSeq++,
    };
    mtrRawAggStore.set(key, row);
  }

  row.sent += 1;
  const ip = rec.ip ? String(rec.ip).trim() : '';
  const host = rec.host ? String(rec.host).trim() : '';
  if (ip) {
    row.ip = ip;
  }
  if (host) {
    row.host = host;
  }

  const success = !!rec.success;
  const rtt = Number(rec.rtt_ms) || 0;
  if (success && (row.ip || row.host)) {
    row.received += 1;
    if (rtt > 0) {
      row.last_ms = rtt;
      row._sum_ms += rtt;
      if (row.best_ms <= 0 || rtt < row.best_ms) {
        row.best_ms = rtt;
      }
      if (rtt > row.worst_ms) {
        row.worst_ms = rtt;
      }
    }
  } else {
    if (!row.errors) {
      row.errors = Object.create(null);
    }
    row.errors.timeout = (Number(row.errors.timeout) || 0) + 1;
  }

  if (rec.asn || rec.country || rec.prov || rec.city || rec.district || rec.owner || rec.lat || rec.lng) {
    row.geo = row.geo || {};
    if (rec.asn) {
      row.geo.asnumber = String(rec.asn).trim();
    }
    if (rec.country) {
      row.geo.country = String(rec.country).trim();
    }
    if (rec.prov) {
      row.geo.prov = String(rec.prov).trim();
    }
    if (rec.city) {
      row.geo.city = String(rec.city).trim();
    }
    if (rec.district) {
      row.geo.district = String(rec.district).trim();
    }
    if (rec.owner) {
      row.geo.owner = String(rec.owner).trim();
    }
    if (Number.isFinite(Number(rec.lat))) {
      row.geo.lat = Number(rec.lat);
    }
    if (Number.isFinite(Number(rec.lng))) {
      row.geo.lng = Number(rec.lng);
    }
  }

  if (Array.isArray(rec.mpls) && rec.mpls.length > 0) {
    const existing = new Set((row.mpls || []).map((v) => String(v)));
    rec.mpls.forEach((m) => {
      const val = String(m || '').trim();
      if (val) {
        existing.add(val);
      }
    });
    row.mpls = Array.from(existing);
  }

  recomputeMTRRawDerived(row);
}

function buildMTRStatsFromRawAgg() {
  const rows = Array.from(mtrRawAggStore.values())
    .sort((a, b) => (a.ttl - b.ttl) || (a._order - b._order))
    .map((row) => {
      const out = {...row};
      delete out._sum_ms;
      delete out._order;
      return out;
    });
  mtrStatsStore = rows;
  return rows;
}

function renderMTRStats(stats) {
  mtrStatsStore = Array.isArray(stats) ? stats : [];
  const normalizer = window.nextTraceMTRAgg && window.nextTraceMTRAgg.normalizeRenderableMTRStats;
  const data = typeof normalizer === 'function' ? normalizer(mtrStatsStore) : mtrStatsStore;
  if (!data || data.length === 0) {
    resultNode.innerHTML = `<p>${t('noResult')}</p>`;
    resultNode.classList.remove('hidden');
    return;
  }

  const table = document.createElement('table');
  const thead = document.createElement('thead');
  thead.innerHTML = `
    <tr>
      <th>${t('tableTTL')}</th>
      <th>${t('colLoss')}</th>
      <th>${t('colLast')}</th>
      <th>${t('colAvg')}</th>
      <th>${t('colBest')}</th>
      <th>${t('colWorst')}</th>
      <th>${t('colHost')}</th>
    </tr>
  `;
  table.appendChild(thead);

  const tbody = document.createElement('tbody');
  let lastTTL = null;
  data.forEach((stat) => {
    const row = document.createElement('tr');

    const lossText = `${Math.round(stat.loss_percent || 0)}% (${stat.loss_count}/${stat.sent})`;
    const lastText = formatLatency(stat.last_ms, stat.received);
    const avgText = formatLatency(stat.avg_ms, stat.received);
    const bestText = formatLatency(stat.best_ms, stat.received);
    const worstText = formatLatency(stat.worst_ms, stat.received);
    const hostParts = getHostDisplayParts(stat);
    const mplsText = formatMPLSText(stat.mpls);
    const geoText = formatGeoDisplay(stat.geo);

    const appendCell = (value) => {
      const td = document.createElement('td');
      td.textContent = value;
      row.appendChild(td);
      return td;
    };

    const displayTTL = lastTTL === stat.ttl ? '' : stat.ttl;
    appendCell(displayTTL);
    lastTTL = stat.ttl;
    appendCell(lossText);
    appendCell(lastText);
    appendCell(avgText);
    appendCell(bestText);
    appendCell(worstText);

    const hostCell = appendCell('');
    hostCell.classList.add('mtr-host-cell');
    if (hostParts.ip) {
      hostCell.appendChild(document.createTextNode(hostParts.ip));
    }
    if (hostParts.ip && hostParts.host) {
      hostCell.appendChild(document.createTextNode(' '));
    }
    if (hostParts.host) {
      const hostSpan = document.createElement('span');
      hostSpan.className = 'mtr-hostname';
      hostSpan.textContent = hostParts.host;
      hostCell.appendChild(hostSpan);
    }
    if (!hostParts.ip && !hostParts.host) {
      hostCell.textContent = '--';
    }
    if (geoText) {
      const geoDiv = document.createElement('div');
      geoDiv.className = 'attempt__geo';
      geoDiv.textContent = geoText;
      hostCell.appendChild(geoDiv);
    }
    if (mplsText) {
      const mplsDiv = document.createElement('div');
      mplsDiv.className = 'mtr-mpls';
      mplsDiv.textContent = mplsText;
      hostCell.appendChild(mplsDiv);
    }

    tbody.appendChild(row);
  });

  table.appendChild(tbody);
  resultNode.innerHTML = '';
  resultNode.appendChild(table);
  resultNode.classList.remove('hidden');
}

function getHostDisplayParts(stat) {
  const ip = stat && stat.ip ? String(stat.ip).trim() : '';
  let host = stat && stat.host ? String(stat.host).trim() : '';
  if (ip && host && host === ip) {
    host = '';
  }
  return {
    ip,
    host,
  };
}

function formatMPLSText(mpls) {
  if (!Array.isArray(mpls) || mpls.length === 0) {
    return '';
  }
  const unique = Array.from(new Set(mpls.map((item) => String(item || '').trim()).filter(Boolean)));
  return unique.join('\n');
}

function formatLatency(value, received) {
  if (!received || value === undefined || value === null || Number(value) <= 0) {
    return '--';
  }
  return Number(value).toFixed(2) + ' ms';
}

function formatGeoDisplay(geo) {
  if (!geo) {
    return '';
  }
  const parts = [];
  if (geo.asnumber) {
    parts.push('AS' + geo.asnumber);
  }
  const country = currentLang === 'en' ? (geo.country_en || geo.country) : (geo.country || geo.country_en);
  if (country) {
    parts.push(country.trim());
  }
  const prov = currentLang === 'en' ? (geo.prov_en || geo.prov) : (geo.prov || geo.prov_en);
  if (prov) {
    parts.push(prov.trim());
  }
  const city = currentLang === 'en' ? (geo.city_en || geo.city) : (geo.city || geo.city_en);
  if (city) {
    parts.push(city.trim());
  }
  if (geo.owner) {
    parts.push(geo.owner.trim());
  } else if (geo.isp) {
    parts.push(geo.isp.trim());
  }
  return parts.filter(Boolean).join(' · ');
}
