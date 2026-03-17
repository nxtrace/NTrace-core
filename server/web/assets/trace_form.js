(function (root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
  root.NextTraceForm = api;
})(typeof globalThis !== 'undefined' ? globalThis : this, function () {
  function readNumericValueFromRaw(raw) {
    const text = String(raw ?? '').trim();
    if (text === '') {
      return undefined;
    }
    const num = Number(text);
    return Number.isFinite(num) ? num : undefined;
  }

  function defaultOptionValue(defaultOptions, key, fallback) {
    if (defaultOptions && Object.prototype.hasOwnProperty.call(defaultOptions, key)) {
      return defaultOptions[key];
    }
    return fallback;
  }

  function buildTracePayload(values) {
    const payload = {
      target: String(values.target || '').trim(),
      protocol: values.protocol,
      data_provider: values.dataProvider,
      disable_maptrace: Boolean(values.disableMaptrace),
      language: values.language,
      mode: values.mode || 'single',
    };

    const isMtrMode = payload.mode === 'mtr';
    if (isMtrMode) {
      payload.queries = 10;
      payload.hop_interval_ms = 1000;
      payload.max_rounds = 0;
    } else {
      const queries = readNumericValueFromRaw(values.queries);
      if (queries !== undefined) {
        payload.queries = Math.max(1, Math.min(63, queries));
      }
    }

    const maxHops = readNumericValueFromRaw(values.maxHops);
    if (maxHops !== undefined) {
      payload.max_hops = maxHops;
    }

    const dstPort = readNumericValueFromRaw(values.dstPort);
    if (dstPort !== undefined) {
      payload.port = dstPort;
    }

    const packetSize = readNumericValueFromRaw(values.packetSize);
    if (packetSize !== undefined) {
      payload.packet_size = packetSize;
    }

    const tos = readNumericValueFromRaw(values.tos);
    if (tos !== undefined) {
      payload.tos = tos;
    }

    return payload;
  }

  return {
    buildTracePayload,
    defaultOptionValue,
    readNumericValueFromRaw,
  };
});
