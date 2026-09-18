import { versionFiles } from './scripts/version.mjs';

export const analysisPlugins = [
  '@semantic-release/commit-analyzer',
  '@semantic-release/release-notes-generator',
];

export default {
  branches: ['main'],
  repositoryUrl: 'https://github.com/d0kur0/Yoru.git',
  tagFormat: 'v${version}',
  plugins: [
    ...analysisPlugins,
    './scripts/release-plugin.mjs',
    ['@semantic-release/changelog', { changelogFile: 'CHANGELOG.md' }],
    ['@semantic-release/git', {
      assets: ['CHANGELOG.md', ...versionFiles],
      message: 'chore(release): ${nextRelease.version} [skip ci]\n\n${nextRelease.notes}',
    }],
    ['@semantic-release/github', {
      assets: ['release/*'],
      successComment: false,
      failComment: false,
      failTitle: false,
      releasedLabels: false,
    }],
  ],
};
