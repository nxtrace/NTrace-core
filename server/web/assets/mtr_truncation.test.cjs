/**
 * Tests for the mtrRawKnownFinalTTL truncation logic in app.js.
 *
 * Since ingestMTRRawRecord lives inside app.js (a browser script with DOM
 * dependencies), we replicate the minimal subset of state and logic here to
 * exercise the truncation behaviour in isolation under Node.js.
 */
const test = require('node:test');
const assert = require('node:assert/strict');

// --- Minimal replica of app.js globals & helpers needed for truncation ---

function createCtx(resolvedIP) {
  const ctx = {
    mtrRawAggStore: new Map(),
    mtrRawOrderSeq: 0,
    mtrRawKnownFinalTTL: Infinity,
    latestSummary: { resolved_ip: resolvedIP },
  };

  function mtrRawKey(rec) {
    const ttl = Number(rec && rec.ttl);
    const ip = rec && rec.ip ? String(rec.ip).trim() : '';
    const host = rec && rec.host ? String(rec.host).trim().toLowerCase() : '';
    if (ip) return `${ttl}|ip:${ip}`;
    if (host) return `${ttl}|host:${host}`;
    return `${ttl}|unknown`;
  }

  /** Mirrors the truncation + ingestion logic from app.js ingestMTRRawRecord */
  ctx.ingest = function ingest(rec) {
    if (!rec || !Number.isFinite(Number(rec.ttl))) return;
    const ttl = Number(rec.ttl);

    if (ttl > ctx.mtrRawKnownFinalTTL) return;

    const resolvedIP = ctx.latestSummary && ctx.latestSummary.resolved_ip
      ? String(ctx.latestSummary.resolved_ip).trim() : '';
    const recIP = rec.ip ? String(rec.ip).trim() : '';
    if (rec.success && recIP && resolvedIP && recIP === resolvedIP && ttl < ctx.mtrRawKnownFinalTTL) {
      ctx.mtrRawKnownFinalTTL = ttl;
      for (const [k, v] of ctx.mtrRawAggStore) {
        if (v.ttl > ctx.mtrRawKnownFinalTTL) {
          ctx.mtrRawAggStore.delete(k);
        }
      }
    }

    const key = mtrRawKey(rec);
    let row = ctx.mtrRawAggStore.get(key);
    if (!row) {
      row = {
        ttl, host: '', ip: '',
        sent: 0, received: 0,
        _order: ctx.mtrRawOrderSeq++,
      };
      ctx.mtrRawAggStore.set(key, row);
    }
    row.sent += 1;
    if (rec.ip) row.ip = String(rec.ip).trim();
    if (rec.host) row.host = String(rec.host).trim();
    if (rec.success) row.received += 1;
  };

  ctx.ttls = function () {
    return Array.from(ctx.mtrRawAggStore.values())
      .sort((a, b) => a.ttl - b.ttl || a._order - b._order)
      .map((r) => r.ttl);
  };

  ctx.rows = function () {
    return Array.from(ctx.mtrRawAggStore.values())
      .sort((a, b) => a.ttl - b.ttl || a._order - b._order);
  };

  return ctx;
}

// --- Tests ---

test('drops records beyond knownFinalTTL', () => {
  const ctx = createCtx('10.0.0.1');
  // Simulate: TTL 1-3 arrive, then TTL 2 hits destination.
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 3, ip: '10.0.0.5', success: true }); // not destination
  ctx.ingest({ ttl: 2, ip: '10.0.0.1', success: true }); // destination!

  assert.equal(ctx.mtrRawKnownFinalTTL, 2);
  // TTL 3 should have been pruned.
  assert.deepEqual(ctx.ttls(), [1, 2]);

  // Further TTL 3 records should be silently dropped.
  ctx.ingest({ ttl: 3, ip: '10.0.0.5', success: true });
  assert.deepEqual(ctx.ttls(), [1, 2]);
});

test('prunes stale high-TTL entries when finalTTL lowers', () => {
  const ctx = createCtx('10.0.0.1');
  // Initial burst: TTL 1-5 all arrive before any destination is known.
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 2, ip: '10.0.0.2', success: true });
  ctx.ingest({ ttl: 3, ip: '10.0.0.3', success: true });
  ctx.ingest({ ttl: 4, ip: '10.0.0.4', success: true });
  ctx.ingest({ ttl: 5, ip: '10.0.0.1', success: true }); // first destination at TTL 5
  assert.equal(ctx.mtrRawKnownFinalTTL, 5);
  assert.deepEqual(ctx.ttls(), [1, 2, 3, 4, 5]);

  // Later: destination found at lower TTL 3.
  ctx.ingest({ ttl: 3, ip: '10.0.0.1', success: true });
  assert.equal(ctx.mtrRawKnownFinalTTL, 3);
  // TTL 4 and 5 should be pruned; TTL 3 now has two paths (10.0.0.3 + 10.0.0.1).
  const ips = ctx.rows().map((r) => `${r.ttl}:${r.ip}`);
  assert.deepEqual(ips, ['1:192.168.0.1', '2:10.0.0.2', '3:10.0.0.3', '3:10.0.0.1']);
  // No TTL > 3 remains.
  assert.ok(ctx.ttls().every((t) => t <= 3));
});

test('timeout records at high TTL are also dropped once finalTTL is set', () => {
  const ctx = createCtx('10.0.0.1');
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 2, ip: '10.0.0.1', success: true }); // destination
  assert.equal(ctx.mtrRawKnownFinalTTL, 2);

  // Late-arriving timeout for TTL 5 should be silently dropped.
  ctx.ingest({ ttl: 5, success: false });
  assert.deepEqual(ctx.ttls(), [1, 2]);
});

test('non-destination IP at same TTL does not trigger truncation', () => {
  const ctx = createCtx('10.0.0.1');
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 2, ip: '10.0.0.2', success: true }); // not destination
  ctx.ingest({ ttl: 3, ip: '10.0.0.3', success: true }); // not destination

  assert.equal(ctx.mtrRawKnownFinalTTL, Infinity);
  assert.deepEqual(ctx.ttls(), [1, 2, 3]);
});

test('does not crash with missing latestSummary.resolved_ip', () => {
  const ctx = createCtx('');
  ctx.ingest({ ttl: 1, ip: '10.0.0.1', success: true });
  ctx.ingest({ ttl: 2, ip: '10.0.0.1', success: true });
  // Without resolved_ip, nothing should be truncated.
  assert.equal(ctx.mtrRawKnownFinalTTL, Infinity);
  assert.deepEqual(ctx.ttls(), [1, 2]);
});

test('reset clears knownFinalTTL (simulated clearResult)', () => {
  const ctx = createCtx('10.0.0.1');
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 2, ip: '10.0.0.1', success: true });
  assert.equal(ctx.mtrRawKnownFinalTTL, 2);

  // Simulate clearResult(true).
  ctx.mtrRawAggStore = new Map();
  ctx.mtrRawOrderSeq = 0;
  ctx.mtrRawKnownFinalTTL = Infinity;

  assert.equal(ctx.mtrRawKnownFinalTTL, Infinity);

  // New session should work fresh.
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 5, ip: '10.0.0.1', success: true });
  assert.equal(ctx.mtrRawKnownFinalTTL, 5);
  assert.deepEqual(ctx.ttls(), [1, 5]);
});

test('destination at TTL equal to current finalTTL does not re-prune', () => {
  const ctx = createCtx('10.0.0.1');
  ctx.ingest({ ttl: 1, ip: '192.168.0.1', success: true });
  ctx.ingest({ ttl: 3, ip: '10.0.0.1', success: true }); // set finalTTL=3
  ctx.ingest({ ttl: 2, ip: '10.0.0.2', success: true });
  assert.equal(ctx.mtrRawKnownFinalTTL, 3);

  // Same TTL destination (ttl === mtrRawKnownFinalTTL): should not lower.
  ctx.ingest({ ttl: 3, ip: '10.0.0.1', success: true });
  assert.equal(ctx.mtrRawKnownFinalTTL, 3);
  assert.deepEqual(ctx.ttls(), [1, 2, 3]);
});
