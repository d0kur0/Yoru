import {
  initialSettings,
  protocols,
  ruleKinds,
  migrateDnsSettings,
  validateDnsPattern,
  validateDnsServer,
  type DnsPolicy,
  type Rule,
  type Server,
} from "./model";

export type Configuration = {
  servers: Server[];
  rules: Rule[];
  settings: typeof initialSettings;
  selected: string;
};
export type Backup = {
  format: "tiho-backup";
  version: 2;
  scope: "design-preview";
  createdAt: string;
  configuration: Configuration;
};
export function createBackup(configuration: Configuration): Backup {
  return {
    format: "tiho-backup",
    version: 2,
    scope: "design-preview",
    createdAt: new Date().toISOString(),
    configuration,
  };
}
const object = (v: unknown): v is Record<string, unknown> =>
  !!v && typeof v === "object" && !Array.isArray(v);
const string = (v: unknown): v is string =>
  typeof v === "string" && v.length <= 65536;
const route = (v: unknown) => ["vpn", "direct", "block"].includes(v as string);
function valid(condition: unknown, message: string): asserts condition {
  if (!condition) throw new Error(message);
}
export function parseBackup(text: string): Backup {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    throw new Error(
      "Не удалось прочитать JSON. Выберите файл резервной копии Yoru.",
    );
  }
  valid(
    object(data) && data.format === "tiho-backup",
    "Это не резервная копия Yoru.",
  );
  valid(
    data.version === 1 || data.version === 2,
    "Эта версия резервной копии пока не поддерживается.",
  );
  valid(
    data.scope === "design-preview",
    "Эта копия несовместима с дизайн-версией приложения.",
  );
  valid(
    string(data.createdAt) && Number.isFinite(Date.parse(data.createdAt)),
    "В копии повреждена дата создания.",
  );
  const c = data.configuration;
  valid(object(c), "В файле отсутствует конфигурация.");
  valid(
    Array.isArray(c.servers) && c.servers.length <= 5000,
    "В копии повреждён список серверов.",
  );
  const servers = c.servers.map((s) => {
    valid(
      object(s) &&
        [s.id, s.name, s.country, s.code, s.protocol, s.host].every(string) &&
        s.id &&
        s.name &&
        protocols.includes(s.protocol as string) &&
        (s.latency === null ||
          (typeof s.latency === "number" &&
            Number.isFinite(s.latency) &&
            s.latency >= 0)),
      "В копии повреждены данные сервера.",
    );
    return {
      id: s.id,
      name: s.name,
      country: s.country,
      code: s.code,
      protocol: s.protocol,
      host: s.host,
      latency: s.latency,
    } as Server;
  });
  valid(
    new Set(servers.map((s) => s.id)).size === servers.length,
    "В копии повторяются идентификаторы серверов.",
  );
  valid(
    Array.isArray(c.rules) && c.rules.length <= 20000,
    "В копии повреждён список правил.",
  );
  const rules = c.rules.map((r) => {
    valid(
      object(r) &&
        string(r.id) &&
        r.id &&
        string(r.kind) &&
        Object.prototype.hasOwnProperty.call(ruleKinds, r.kind) &&
        string(r.value) &&
        route(r.route) &&
        typeof r.enabled === "boolean" &&
        (r.system === undefined || typeof r.system === "boolean"),
      "В копии повреждены данные правила.",
    );
    return {
      id: r.id,
      kind: r.kind,
      value: r.value,
      route: r.route,
      enabled: r.enabled,
      ...(r.system === undefined ? {} : { system: r.system }),
    } as Rule;
  });
  valid(
    new Set(rules.map((r) => r.id)).size === rules.length,
    "В копии повторяются идентификаторы правил.",
  );
  valid(
    string(c.selected) &&
      (servers.length
        ? servers.some((s) => s.id === c.selected)
        : c.selected === ""),
    "Выбранный сервер отсутствует в копии.",
  );
  valid(object(c.settings), "В копии отсутствуют настройки приложения.");
  if (data.version === 1)
    valid(
      string(c.settings.domainDns) && string(c.settings.dnsDomains),
      "В копии повреждены настройки DNS.",
    );
  const s = data.version === 1 ? migrateDnsSettings(c.settings) : c.settings;
  for (const [key, value] of Object.entries(initialSettings)) {
    if (key === "dnsPolicies") continue;
    valid(
      typeof value === "boolean" ? typeof s[key] === "boolean" : string(s[key]),
      "В копии повреждены настройки приложения.",
    );
  }
  valid(
    ["system", "light", "dark"].includes(s.theme as string) && route(s.defaultRoute),
    "В копии указаны неизвестные настройки.",
  );
  valid(
    Array.isArray(s.dnsPolicies) && s.dnsPolicies.length <= 5000,
    "В копии повреждена карта DNS.",
  );
  const dnsPolicies = s.dnsPolicies.map((p) => {
    valid(
      object(p) &&
        string(p.id) &&
        p.id &&
        string(p.name) &&
        typeof p.enabled === "boolean" &&
        Array.isArray(p.domains) &&
        p.domains.length > 0 &&
        p.domains.every((d) => string(d) && validateDnsPattern(d)) &&
        Array.isArray(p.servers) &&
        p.servers.length > 0 &&
        p.servers.every((d) => string(d) && validateDnsServer(d)),
      "В копии повреждено соответствие доменов и DNS.",
    );
    return {
      id: p.id,
      name: p.name,
      domains: p.domains,
      servers: p.servers,
      enabled: p.enabled,
    } as DnsPolicy;
  });
  valid(
    new Set(dnsPolicies.map((p) => p.id)).size === dnsPolicies.length,
    "В копии повторяются идентификаторы DNS.",
  );
  const patterns = dnsPolicies.flatMap((p) =>
    p.domains.map((d) => d.toLowerCase()),
  );
  valid(
    new Set(patterns).size === patterns.length,
    "Один домен указан в нескольких соответствиях DNS.",
  );
  const settings = Object.fromEntries(
    Object.keys(initialSettings).map((key) => [key, s[key]]),
  ) as typeof initialSettings;
  settings.dnsPolicies = dnsPolicies;
  return {
    format: "tiho-backup",
    version: 2,
    scope: "design-preview",
    createdAt: data.createdAt as string,
    configuration: { servers, rules, settings, selected: c.selected },
  };
}
export function downloadBackup(configuration: Configuration) {
  const backup = createBackup(configuration);
  const url = URL.createObjectURL(
    new Blob([JSON.stringify(backup, null, 2)], { type: "application/json" }),
  );
  const link = document.createElement("a");
  link.href = url;
  link.download = `tiho-backup-${backup.createdAt.slice(0, 10)}.json`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
