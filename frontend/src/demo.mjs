// Documentation-only preview. No native bindings or personal configuration.
import './reference/style.css';
import './reference/desktop.css';
import './reference/operations.css';
import 'overlayscrollbars/overlayscrollbars.css';
import { emptyConfig } from './reference/runtime.js';
import product from '../product.json';

window.APP_PRODUCT = product;
window.APP_DESKTOP = true;
document.documentElement.dataset.desktop = 'true';
const config = emptyConfig();
config.revision = 1;
config.settings.groupMode = 'fallback';
config.settings.latencyURL = 'https://www.gstatic.com/generate_204';
config.subscriptions = [{ id: 'demo-sub', name: 'Моя подписка', url: 'https://subscription.example/servers', intervalHours: 24, updated: new Date().toISOString() }];
config.servers = [
  { id: 'ams', name: 'Amsterdam', country: 'Нидерланды', flag: '🇳🇱', protocol: 'VLESS', transport: 'Reality', host: 'nl.example', port: 443, subscription: 'demo-sub', options: { type: 'vless', server: 'nl.example', port: 443, tls: true, network: 'tcp', 'reality-opts': { 'public-key': 'DEMO' } } },
  { id: 'fra', name: 'Frankfurt', country: 'Германия', flag: '🇩🇪', protocol: 'Hysteria2', transport: 'TLS', host: 'de.example', port: 443, subscription: 'demo-sub', options: { type: 'hysteria2', server: 'de.example', port: 443, tls: true } },
  { id: 'hel', name: 'Helsinki', country: 'Финляндия', flag: '🇫🇮', protocol: 'Trojan', transport: 'TLS', host: 'fi.example', port: 443, subscription: '', options: { type: 'trojan', server: 'fi.example', port: 443, tls: true } },
];
config.selected = 'ams';
config.settings.fallbackEnabled = true;
config.settings.serverOrder = ['ams', 'fra', 'hel'];
config.sets = [
  { id: 'work', name: 'Рабочая сеть', enabled: true, action: 'DIRECT', domains: ['+.company.example'], keywords: [], resolvers: ['10.20.0.53'], cidrs: ['10.20.0.0/16'], processes: [], realIP: true, bypassTUN: true },
  { id: 'media', name: 'Сервисы и приложения', enabled: true, action: 'PROXY', domains: ['+.openai.com', '+.github.com', '+.discord.com'], keywords: [], resolvers: [], cidrs: [], processes: ['Discord.exe'], realIP: false, bypassTUN: false },
];
config.rules = [
  { id: 'r1', type: 'PROCESS-NAME', value: 'Discord.exe', action: 'PROXY', enabled: true },
  { id: 'r2', type: 'DOMAIN-SUFFIX', value: 'github.com', action: 'PROXY', enabled: true },
  { id: 'r3', type: 'DOMAIN-SUFFIX', value: 'company.example', action: 'DIRECT', enabled: true },
];
const rows = [
  ['chrome.exe', 'chatgpt.com', 'PROXY', 18400000, 430000],
  ['Discord.exe', 'gateway.discord.gg', 'PROXY', 3200000, 810000],
  ['Code.exe', 'github.com', 'PROXY', 4800000, 92000],
  ['chrome.exe', 'portal.company.example', 'DIRECT', 7200000, 260000],
  ['Steam.exe', 'store.steampowered.com', 'PROXY', 68000000, 1200000],
  ['explorer.exe', 'files.company.example', 'DIRECT', 2400000, 78000],
];
const started = Date.now() - 1047000;
let down = 106 * 1024 ** 2;
let up = 12 * 1024 ** 2;
window.APP_API = {
  LoadConfig: async () => JSON.stringify(config),
  Status: async () => JSON.stringify({ installed: true, bundled: true, running: true, version: 'v1.19.30', revision: 1, started, activeServer: 'ams', downloadTotal: down += 124000, uploadTotal: up += 7000, connections: rows.map(([app, host, action, down, up], i) => ({ id: String(i), app, host, action, down, up, type: 'tcp', processPath: '', ip: '', rule: 'DomainSuffix' })) }),
  TestServerLatency: async id => ({ ams: 42, fra: 58, hel: 67 })[id],
  TestRunningServerLatency: async () => 42,
  LogsLocation: async () => 'C:\\Users\\User\\AppData\\Roaming\\Yoru\\logs',
  OpenLogsFolder: async () => { throw new Error('Папка доступна в установленном приложении'); },
  ProcessIcon: async () => '',
  SaveConfig: async () => { throw new Error('Демонстрационный режим: настройки не сохраняются'); },
};
const status = document.createElement('div');
status.id = 'operation-status'; status.hidden = true; document.body.append(status);
await import('./reference/app.js');
await import('./reference/controls.js');
await import('./reference/scrollbars.js');
