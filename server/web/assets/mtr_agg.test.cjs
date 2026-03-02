const test = require('node:test');
const assert = require('node:assert/strict');

const { normalizeRenderableMTRStats } = require('./mtr_agg.js');

test('merges unknown stats into the only known path for a ttl', () => {
  const rows = normalizeRenderableMTRStats([
    { ttl: 1, sent: 3, received: 0, loss_count: 3, failure_type: 'all_timeout' },
    { ttl: 1, ip: '1.1.1.1', host: 'one.one.one.one', sent: 1, received: 1, loss_count: 0, failure_type: '' },
  ]);

  assert.equal(rows.length, 1);
  assert.equal(rows[0].ip, '1.1.1.1');
  assert.equal(rows[0].sent, 4);
  assert.equal(rows[0].loss_count, 3);
  assert.equal(rows[0].received, 1);
  assert.equal(rows[0].failure_type, 'all_timeout');
});

test('preserves unknown row for multipath ttl instead of merging into the first known path', () => {
  const rows = normalizeRenderableMTRStats([
    { ttl: 2, sent: 2, received: 0, loss_count: 2, failure_type: 'all_timeout' },
    { ttl: 2, ip: '2.2.2.2', host: 'a.example', sent: 3, received: 3, loss_count: 0, failure_type: '' },
    { ttl: 2, ip: '2.2.2.3', host: 'b.example', sent: 4, received: 4, loss_count: 0, failure_type: '' },
  ]);

  assert.equal(rows.length, 3);

  const unknown = rows.find((row) => !row.ip && !row.host);
  assert.ok(unknown);
  assert.equal(unknown.sent, 2);
  assert.equal(unknown.loss_count, 2);

  const firstKnown = rows.find((row) => row.ip === '2.2.2.2');
  assert.equal(firstKnown.sent, 3);
  assert.equal(firstKnown.loss_count, 0);
});
