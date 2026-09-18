import semanticRelease from 'semantic-release';
import { appendFile } from 'node:fs/promises';
import config, { analysisPlugins } from '../release.config.mjs';

const result = await semanticRelease({ ...config, dryRun: true, plugins: analysisPlugins });
const version = result ? result.nextRelease.version : '';
if (!process.env.GITHUB_OUTPUT) throw new Error('Run release:plan in GitHub Actions');
await appendFile(process.env.GITHUB_OUTPUT, `version=${version}\n`);
