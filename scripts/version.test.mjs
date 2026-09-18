import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, mkdir, copyFile, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { setVersion, versionFiles } from './version.mjs';

test('one release version reaches UI, package locks and native installer metadata', async () => {
  const root = await mkdtemp(join(tmpdir(), 'yoru-version-'));
  try {
    for (const file of versionFiles) {
      await mkdir(dirname(join(root, file)), { recursive: true });
      await copyFile(file, join(root, file));
    }
    await setVersion('12.34.56', root);
    for (const file of versionFiles) {
      const text = await readFile(join(root, file), 'utf8');
      assert.ok(text.includes('12.34.56'), file);
      if (file.endsWith('package-lock.json')) {
        const data = JSON.parse(text);
        assert.equal(data.version, '12.34.56');
        assert.equal(data.packages[''].version, '12.34.56');
      }
      if (file.endsWith('.plist')) {
        assert.match(text, /<key>CFBundleVersion<\/key>\s*<string>12\.34\.56<\/string>/);
        assert.match(text, /<key>CFBundleShortVersionString<\/key>\s*<string>12\.34\.56<\/string>/);
      }
    }
    const info = JSON.parse(await readFile(join(root, 'build/windows/info.json')));
    assert.equal(info.fixed.file_version, '12.34.56.0');
    assert.equal(info.info['0409'].ProductVersion, '12.34.56');
    await assert.rejects(setVersion('1.0.0-beta.1', root));
    await assert.rejects(setVersion('65536.0.0', root));
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
