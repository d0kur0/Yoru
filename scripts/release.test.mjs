import test from 'node:test';
import assert from 'node:assert/strict';
import { analyzeCommits } from '@semantic-release/commit-analyzer';
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
