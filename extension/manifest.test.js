import {test} from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {createHash, createPublicKey} from 'node:crypto';
test('stable RSA extension identity matches native host', () => {
  const m = JSON.parse(readFileSync(new URL('./manifest.json', import.meta.url)));
  const der = Buffer.from(m.key, 'base64');
  assert.equal(createPublicKey({key: der, format: 'der', type: 'spki'}).asymmetricKeyDetails.modulusLength, 2048);
  const id = createHash('sha256').update(der).digest('hex').slice(0, 32).replace(/[0-9a-f]/g, c => String.fromCharCode(97 + parseInt(c, 16)));
  assert.ok(readFileSync(new URL('../internal/takeover/register.go', import.meta.url), 'utf8').includes(`ExtensionID = "${id}"`));
});
