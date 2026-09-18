import { spawn } from 'node:child_process';

const [os, arch] = process.argv.slice(2);
if (!['windows', 'darwin'].includes(os) || !['amd64', 'arm64'].includes(arch)) {
  throw new Error('Usage: node scripts/package.mjs windows|darwin amd64|arm64');
}
const child = spawn(process.platform === 'win32' ? 'wails3.exe' : 'wails3',
  ['package', `GOOS=${os}`, `ARCH=${arch}`], { stdio: ['ignore', 'pipe', 'pipe'] });
let tail = '';
for (const [stream, destination] of [[child.stdout, process.stdout], [child.stderr, process.stderr]]) {
  stream.on('data', data => { destination.write(data); tail = (tail + data.toString()).slice(-16000); });
}
child.on('error', error => { console.error(error.message); process.exitCode = 1; });
child.on('close', code => {
  if (code && process.env.GITHUB_ACTIONS) {
    // Put useful compiler/installer diagnostics into the public check result too.
    const message = tail.split(/\r?\n/).slice(-35).join('\n')
      .replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');
    console.log(`::error title=Packaging failed::${message}`);
  }
  process.exitCode = code ?? 1;
});
