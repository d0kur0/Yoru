import { readFile, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';

export const versionFiles = [
  'package.json', 'package-lock.json',
  'frontend/package.json', 'frontend/package-lock.json', 'frontend/product.json',
  'build/config.yml', 'build/windows/info.json', 'build/windows/nsis/wails_tools.nsh',
  'build/darwin/Info.plist', 'build/darwin/Info.dev.plist',
];

export async function setVersion(version, root = process.cwd()) {
  if (!/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$/.test(version) ||
      version.split('.').some(n => Number(n) > 65535)) {
    throw new Error(`Unsupported desktop release version: ${version}`);
  }
  // Read and validate every input before changing any file.
  const updates = await Promise.all(versionFiles.map(async file => {
    let text = await readFile(resolve(root, file), 'utf8');
    const replace = (pattern, replacement) => {
      if (!pattern.test(text)) throw new Error(`Version field missing: ${file}`);
      text = text.replace(pattern, replacement);
    };
    if (file.endsWith('.json')) {
      const data = JSON.parse(text);
      if (file === 'build/windows/info.json') {
        data.fixed.file_version = data.fixed.product_version = `${version}.0`;
        for (const language of Object.values(data.info)) {
          language.FileVersion = language.ProductVersion = version;
        }
      } else {
        data.version = version;
        if (data.packages?.['']) data.packages[''].version = version;
      }
      text = `${JSON.stringify(data, null, 2)}\n`;
    } else if (file.endsWith('.plist')) {
      for (const key of ['CFBundleVersion', 'CFBundleShortVersionString']) {
        replace(new RegExp(`(<key>${key}</key>\\s*<string>)[^<]+(</string>)`), `$1${version}$2`);
      }
    } else if (file.endsWith('.nsh')) {
      replace(/(!define INFO_PRODUCTVERSION ")[^"]+(".*)/, `$1${version}$2`);
    } else {
      replace(/(  version: ")[^"]+(".*)/, `$1${version}$2`);
    }
    return [resolve(root, file), text];
  }));
  for (const [file, text] of updates) await writeFile(file, text, 'utf8');
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  await setVersion(process.argv[2]);
  console.log(`Desktop metadata: ${process.argv[2]}`);
}
