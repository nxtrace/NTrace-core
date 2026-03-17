const test = require('node:test');
const assert = require('node:assert/strict');

const { buildTracePayload, defaultOptionValue } = require('./trace_form.js');

test('buildTracePayload preserves negative packet_size and zero tos', () => {
  const payload = buildTracePayload({
    target: '1.1.1.1',
    protocol: 'icmp',
    dataProvider: 'LeoMoeAPI',
    disableMaptrace: false,
    language: 'cn',
    mode: 'single',
    queries: '3',
    maxHops: '30',
    dstPort: '',
    packetSize: '-123',
    tos: '0',
  });

  assert.equal(payload.packet_size, -123);
  assert.equal(payload.tos, 0);
  assert.equal(payload.queries, 3);
});

test('buildTracePayload carries packet_size and tos in mtr mode', () => {
  const payload = buildTracePayload({
    target: 'example.com',
    protocol: 'udp',
    dataProvider: 'disable-geoip',
    disableMaptrace: true,
    language: 'en',
    mode: 'mtr',
    queries: '5',
    maxHops: '20',
    dstPort: '33494',
    packetSize: '80',
    tos: '255',
  });

  assert.equal(payload.mode, 'mtr');
  assert.equal(payload.queries, 10);
  assert.equal(payload.hop_interval_ms, 1000);
  assert.equal(payload.max_rounds, 0);
  assert.equal(payload.packet_size, 80);
  assert.equal(payload.tos, 255);
});

test('defaultOptionValue keeps explicit zero', () => {
  assert.equal(defaultOptionValue({ tos: 0 }, 'tos', 5), 0);
  assert.equal(defaultOptionValue({}, 'tos', 5), 5);
});
