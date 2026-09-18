import { readdir } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';

const files = [];
for (const dir of ['scripts', 'frontend', 'frontend/src/reference']) {
  for (const name of await readdir(dir)) {
    if (name.endsWith('.test.mjs')) files.push(`${dir}/${name}`);
  }
}
const result = spawnSync(process.execPath, ['--test', ...files], { stdio: 'inherit' });
process.exit(result.status ?? 1);
