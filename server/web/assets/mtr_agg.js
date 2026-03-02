(function(root, factory) {
  const api = factory();
  if (typeof module !== 'undefined' && module.exports) {
    module.exports = api;
  }
  if (root) {
    root.nextTraceMTRAgg = api;
  }
})(typeof globalThis !== 'undefined' ? globalThis : this, function() {
  const PROTOTYPE_POLLUTION_KEYS = new Set(['__proto__', 'prototype', 'constructor']);

  function normalizeErrorKey(key) {
    const trimmed = String(key || '').trim();
    if (!trimmed || PROTOTYPE_POLLUTION_KEYS.has(trimmed)) {
      return null;
    }
    return trimmed;
  }

  function mergeErrorMaps(target, source) {
    const result = Object.create(null);
    if (target) {
      Object.keys(target).forEach((key) => {
        const normalizedKey = normalizeErrorKey(key);
        if (!normalizedKey) {
          return;
        }
        result[normalizedKey] = Number(target[key]) || 0;
      });
    }
    if (!source) {
      return result;
    }
    Object.keys(source).forEach((key) => {
      const normalizedKey = normalizeErrorKey(key);
      if (!normalizedKey) {
        return;
      }
      const current = Number(result[normalizedKey]) || 0;
      const addition = Number(source[key]) || 0;
      result[normalizedKey] = current + addition;
    });
    return result;
  }

  function cloneErrors(source) {
    if (!source) {
      return null;
    }
    const result = Object.create(null);
    Object.keys(source).forEach((key) => {
      const normalizedKey = normalizeErrorKey(key);
      if (!normalizedKey) {
        return;
      }
      result[normalizedKey] = Number(source[key]) || 0;
    });
    return result;
  }

  function pickFailureType(current, candidate) {
    const priority = {
      all_timeout: 3,
      partial_timeout: 2,
      mixed: 1,
    };
    const normalizedCurrent = current || '';
    const normalizedCandidate = candidate || '';
    const currentPriority = priority[normalizedCurrent] || 0;
    const candidatePriority = priority[normalizedCandidate] || 0;
    if (candidatePriority > currentPriority) {
      return normalizedCandidate;
    }
    return normalizedCurrent;
  }

  function cloneStat(stat) {
    const out = {...stat};
    out.errors = cloneErrors(stat && stat.errors);
    if (Array.isArray(stat && stat.mpls)) {
      out.mpls = [...stat.mpls];
    }
    return out;
  }

  function isKnownStat(stat) {
    const hasIp = stat && stat.ip && String(stat.ip).trim();
    const hasHost = stat && stat.host && String(stat.host).trim();
    return !!(hasIp || hasHost);
  }

  function aggregateUnknown(group) {
    const acc = {
      sent: 0,
      loss: 0,
      errors: null,
      failureType: '',
    };
    group.unknown.forEach(({stat}) => {
      acc.sent += Number(stat.sent) || 0;
      acc.loss += Number(stat.loss_count) || 0;
      acc.errors = mergeErrorMaps(acc.errors, stat.errors || null);
      acc.failureType = pickFailureType(acc.failureType, stat.failure_type || '');
    });
    return acc;
  }

  function mergeUnknownIntoSingleKnown(rows) {
    const ttlGroups = new Map();
    rows.forEach((stat, idx) => {
      if (!stat) {
        return;
      }
      const ttl = Number(stat.ttl) || 0;
      let group = ttlGroups.get(ttl);
      if (!group) {
        group = {known: [], unknown: []};
        ttlGroups.set(ttl, group);
      }
      if (isKnownStat(stat)) {
        group.known.push({idx, stat});
      } else {
        group.unknown.push({idx, stat});
      }
    });

    const mergedUnknownIdx = new Set();
    ttlGroups.forEach((group) => {
      if (group.known.length !== 1 || group.unknown.length === 0) {
        return;
      }
      const primary = group.known[0].stat;
      const unknown = aggregateUnknown(group);
      const existingSent = Number(primary.sent) || 0;
      const existingLoss = Number(primary.loss_count) || 0;
      const totalSent = existingSent + unknown.sent;
      const totalLoss = existingLoss + unknown.loss;
      primary.sent = totalSent;
      primary.loss_count = totalLoss;
      primary.loss_percent = totalSent > 0 ? (totalLoss / totalSent) * 100 : 0;
      primary.received = Math.max(0, totalSent - totalLoss);
      primary.errors = mergeErrorMaps(primary.errors, unknown.errors);
      primary.failure_type = pickFailureType(primary.failure_type, unknown.failureType);

      group.unknown.forEach(({idx}) => mergedUnknownIdx.add(idx));
    });

    return rows.filter((_, idx) => !mergedUnknownIdx.has(idx));
  }

  function normalizeRenderableMTRStats(stats) {
    const rows = Array.isArray(stats) ? stats.map(cloneStat) : [];
    return mergeUnknownIntoSingleKnown(rows);
  }

  return {
    normalizeRenderableMTRStats,
  };
});
