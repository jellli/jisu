import {test} from 'node:test';
import assert from 'node:assert/strict';
import {createTakeoverTracker} from './pending.js';

const item = {id: 1, url: 'https://example.com/download', state: 'in_progress', mime: 'application/octet-stream', totalBytes: 100};

test('delayed filename rechecks once using creation-time Alt state', () => {
  const tracker = createTakeoverTracker();
  assert.equal(tracker.created(item, false), false);
  assert.equal(tracker.claim({...item, filename: 'installer.exe'}), true);
  assert.equal(tracker.claim({...item, filename: 'installer.exe'}), false);
  assert.equal(tracker.created({...item, id: 2}, true), false);
  assert.equal(tracker.claim({...item, id: 2, filename: 'installer.exe'}), false);
});

test('missing, completed and already handled downloads stay untouched', () => {
  const tracker = createTakeoverTracker();
  assert.equal(tracker.claim({...item, filename: 'installer.exe'}), false);
  assert.equal(tracker.created(item, false), false);
  assert.equal(tracker.claim({...item, filename: 'installer.exe', state: 'complete'}), false);
  assert.equal(tracker.created({...item, id: 2, filename: 'installer.exe'}, false), true);
  assert.equal(tracker.claim({...item, id: 2, filename: 'installer.exe'}), false);
});
