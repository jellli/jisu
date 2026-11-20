import {test} from 'node:test';
import assert from 'node:assert/strict';
import {shouldTakeover, filenameHint} from './decide.js';

const item = {url: 'https://example.com/a.zip', state: 'in_progress'};

test('browser filename hints contain only the basename', () => {
  assert.equal(filenameHint('/home/user/Downloads/file.zip'), 'file.zip');
  assert.equal(filenameHint('C:\\Users\\user\\Downloads\\file.zip'), 'file.zip');
  assert.equal(filenameHint('C:\\Downloads/mixed/file.zip'), 'file.zip');
  assert.equal(filenameHint(), '');
});

test('common types and large files', () => {
  assert.equal(shouldTakeover(item), true);
  assert.equal(shouldTakeover({...item, url: 'https://example.com/a', totalBytes: 5242880}), true);
  assert.equal(shouldTakeover({...item, url: 'https://example.com/a', mime: 'video/mp4'}), true);
});

test('empty and invalid downloads stay in browser', () => {
  for (const v of [
    {},
    {...item, url: 'file:///a.zip'},
    {...item, state: 'complete'},
    {...item, byExtensionId: 'self'},
    {...item, url: 'https://example.com/tiny.txt'}
  ]) {
    assert.equal(shouldTakeover(v), false);
  }
  assert.equal(shouldTakeover(item, true), false);
});
