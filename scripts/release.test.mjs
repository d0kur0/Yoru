import test from 'node:test';
import assert from 'node:assert/strict';
import { analyzeCommits } from '@semantic-release/commit-analyzer';
import { generateNotes } from '@semantic-release/release-notes-generator';
import { analysisPlugins } from '../release.config.mjs';

for (const [message, expected] of [
  ['fix: correct latency', 'patch'],
  ['feat: add a route set', 'minor'],
  ['feat!: change configuration format', 'major'],
  ['fix: change configuration\n\nBREAKING CHANGE: old files are unsupported', 'major'],
  ['docs: refresh screenshots', null],
]) {
  test(`release classification: ${message.split('\n')[0]}`, async () => {
    assert.equal(await analyzeCommits(analysisPlugins[0][1], {
      commits: [{ hash: 'test', message }], logger: { log() {} },
    }), expected);
  });
}

test('release preset renders notes with the installed changelog writer', async () => {
  const notes = await generateNotes(analysisPlugins[1][1], {
    cwd: process.cwd(),
    options: { repositoryUrl: 'https://github.com/d0kur0/Yoru.git' },
    commits: [{ hash: '1234567890abcdef', message: 'feat: support desktop installers' }],
    lastRelease: {}, nextRelease: { version: '1.0.0', gitTag: 'v1.0.0' },
    logger: { log() {} },
  });
  assert.match(notes, /support desktop installers/);
  assert.match(notes, /1\.0\.0/);
});
