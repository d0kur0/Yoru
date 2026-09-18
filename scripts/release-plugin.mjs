import { createHash } from 'node:crypto';
import { readFile, readdir, writeFile } from 'node:fs/promises';
import { setVersion } from './version.mjs';

export async function verifyRelease(_config, { nextRelease }) {
  if (nextRelease.version !== process.env.RELEASE_VERSION) {
    throw new Error('Release version changed since builds were started; refuse to publish mismatched binaries');
  }
  const expected = [
    `Yoru-${nextRelease.version}-windows-x64-setup.exe`,
    `Yoru-${nextRelease.version}-macos-arm64.dmg`,
    `Yoru-${nextRelease.version}-macos-x64.dmg`,
  ];
  const files = await readdir('release');
  if (files.length !== expected.length || expected.some(file => !files.includes(file))) {
    throw new Error(`Incomplete release: expected ${expected.join(', ')}`);
  }
  for (const file of expected) {
    const bytes = await readFile(`release/${file}`);
    if (bytes.length < 1_000_000) throw new Error(`Unexpectedly small installer: ${file}`);
  }
}

export async function prepare(_config, { nextRelease }) {
  await setVersion(nextRelease.version);
  const lines = [];
  for (const file of (await readdir('release')).sort()) {
    const bytes = await readFile(`release/${file}`);
    lines.push(`${createHash('sha256').update(bytes).digest('hex')}  ${file}`);
  }
  await writeFile('release/SHA256SUMS.txt', `${lines.join('\n')}\n`);
}

export function generateNotes() {
  return '### Installers\n\nWindows x64: `-setup.exe`. macOS: choose `arm64` for Apple Silicon or `x64` for Intel, open the DMG and drag Yoru to Applications.\n\nThese builds are unsigned (macOS uses an ad-hoc signature), without Apple notarization. The operating system may require approval to launch them. SHA-256 checksums are attached.';
}
