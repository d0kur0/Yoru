import { spawn } from 'node:child_process';

async function check(args) {
  return new Promise(resolve => {
    const child = spawn(process.platform === 'win32' ? 'go.exe' : 'go', args,
      { stdio: ['ignore', 'pipe', 'pipe'] });
    let output = '';
    for (const [stream, target] of [[child.stdout, process.stdout], [child.stderr, process.stderr]]) {
      stream.on('data', data => { target.write(data); output = (output + data.toString()).slice(-32000); });
    }
    child.on('error', error => { console.error(error.message); resolve(1); });
    child.on('close', code => {
      if (code && process.env.GITHUB_ACTIONS) {
        const message = output.replaceAll('%', '%25').replaceAll('\r', '%0D').replaceAll('\n', '%0A');
        console.log(`::error title=Go ${args[0]} failed::${message}`);
      }
      resolve(code ?? 1);
    });
  });
}

process.exitCode = await check(['test', './...']);
if (!process.exitCode) process.exitCode = await check(['vet', './...']);
