import {test} from 'node:test';
import assert from 'node:assert/strict';
import {shouldTakeover} from './decide.js';
const item = {url: 'https://example.com/a.zip', state: 'in_progress'};
test('common types and large files', () => {
  assert.equal(shouldTakeover(item), true);
  assert.equal(shouldTakeover({...item, url: 'https://example.com/a', totalBytes: 5242880}), true);
  assert.equal(shouldTakeover({...item, url: 'https://example.com/a', mime: 'video/mp4'}), true);
});
test('empty and invalid downloads stay in browser', () => {
  for (const v of [{}, {...item, url: 'file:///a.zip'}, {...item, state: 'complete'}, {...item, byExtensionId: 'self'}, {...item, url: 'https://example.com/tiny.txt'}]) assert.equal(shouldTakeover(v), false);
  assert.equal(shouldTakeover(item, true), false);
});
