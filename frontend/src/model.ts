export type Route = "vpn" | "direct" | "block";
export type Server = {
  id: string;
  name: string;
  country: string;
  code: string;
  protocol: string;
  latency: number | null;
  host: string;
};
export type Rule = {
  id: string;
  kind: string;
  value: string;
  route: Route;
  enabled: boolean;
  system?: boolean;
};
export type Connection = {
  id: number;
  app: string;
  icon: string;
  host: string;
  route: Route;
  rule: string;
  down: string;
  up: string;
};
export const routeNames: Record<Route, string> = {
  vpn: "Через VPN",
  direct: "Напрямую",
  block: "Блокировать",
};
export const ruleKinds: Record<string, string> = {
  "DOMAIN-SUFFIX": "Домен и поддомены",
  DOMAIN: "Точный домен",
  "DOMAIN-KEYWORD": "Домен содержит",
  "IP-CIDR": "Подсеть IPv4",
  "IP-CIDR6": "Подсеть IPv6",
  "PROCESS-NAME": "Приложение",
  "PROCESS-PATH": "Путь к программе",
  "DST-PORT": "Порт назначения",
  GEOIP: "Страна IP",
  GEOSITE: "Категория сайтов",
};
export const protocols = [
  "VLESS",
  "VMess",
  "Trojan",
  "Shadowsocks",
  "Hysteria2",
  "Hysteria",
  "TUIC",
  "WireGuard",
  "SOCKS5",
  "HTTP",
  "Snell",
  "SSH",
  "Mieru",
  "AnyTLS",
];
export const initialServers: Server[] = [
  {
    id: "ams",
    name: "Amsterdam",
    country: "Нидерланды",
    code: "nl",
    protocol: "VLESS",
    latency: 34,
    host: "nl-01.example.net",
  },
  {
    id: "fra",
    name: "Frankfurt",
    country: "Германия",
    code: "de",
    protocol: "Trojan",
    latency: 48,
    host: "de-01.example.net",
  },
  {
    id: "hel",
    name: "Helsinki",
    country: "Финляндия",
    code: "fi",
    protocol: "Hysteria2",
    latency: 62,
    host: "fi-01.example.net",
  },
  {
    id: "sto",
    name: "Stockholm",
    country: "Швеция",
    code: "se",
    protocol: "Shadowsocks",
    latency: null,
    host: "se-01.example.net",
  },
];
export const initialRules: Rule[] = [
  {
    id: "corp",
    kind: "DOMAIN-SUFFIX",
    value: "company.example",
    route: "direct",
    enabled: true,
    system: true,
  },
  {
    id: "corp2",
    kind: "DOMAIN-SUFFIX",
    value: "dev.internal.example",
    route: "direct",
    enabled: true,
    system: true,
  },
  {
    id: "lan",
    kind: "IP-CIDR",
    value: "192.168.0.0/16",
    route: "direct",
    enabled: true,
  },
  {
    id: "openai",
    kind: "DOMAIN-KEYWORD",
    value: "openai",
    route: "vpn",
    enabled: true,
  },
  {
    id: "youtube",
    kind: "DOMAIN-SUFFIX",
    value: "youtube.com",
    route: "vpn",
    enabled: true,
  },
  {
    id: "telegram",
    kind: "PROCESS-NAME",
    value: "Telegram.exe",
    route: "vpn",
    enabled: true,
  },
  {
    id: "steam",
    kind: "PROCESS-NAME",
    value: "steam.exe",
    route: "direct",
    enabled: true,
  },
  {
    id: "ads",
    kind: "DOMAIN-SUFFIX",
    value: "doubleclick.net",
    route: "block",
    enabled: false,
  },
];
export const connections: Connection[] = [
  {
    id: 1,
    app: "Google Chrome",
    icon: "chrome",
    host: "youtube.com",
    route: "vpn",
    rule: "DOMAIN-SUFFIX · youtube.com",
    down: "248,6 МБ",
    up: "2,4 МБ",
  },
  {
    id: 2,
    app: "Telegram",
    icon: "telegram",
    host: "149.154.167.99",
    route: "vpn",
    rule: "PROCESS-NAME · Telegram.exe",
    down: "18,2 МБ",
    up: "864 КБ",
  },
  {
    id: 3,
    app: "Google Chrome",
    icon: "chrome",
    host: "jira.company.example",
    route: "direct",
    rule: "DOMAIN-SUFFIX · company.example",
    down: "4,8 МБ",
    up: "312 КБ",
  },
  {
    id: 4,
    app: "Visual Studio Code",
    icon: "code",
    host: "api.github.com",
    route: "vpn",
    rule: "MATCH · правило по умолчанию",
    down: "2,1 МБ",
    up: "148 КБ",
  },
  {
    id: 5,
    app: "Slack",
    icon: "slack",
    host: "slack.com",
    route: "vpn",
    rule: "MATCH · правило по умолчанию",
    down: "1,4 МБ",
    up: "92 КБ",
  },
  {
    id: 6,
    app: "Steam",
    icon: "steam",
    host: "cdn.steampowered.com",
    route: "direct",
    rule: "PROCESS-NAME · steam.exe",
    down: "86,3 МБ",
    up: "1,1 МБ",
  },
];
export const apps = [
  { name: "Google Chrome", exe: "chrome.exe", icon: "chrome" },
  { name: "Telegram", exe: "Telegram.exe", icon: "telegram" },
  { name: "Visual Studio Code", exe: "Code.exe", icon: "code" },
  { name: "Slack", exe: "slack.exe", icon: "slack" },
  { name: "Steam", exe: "steam.exe", icon: "steam" },
  { name: "Discord", exe: "Discord.exe", icon: "discord" },
];
export type DnsPolicy = {
  id: string;
  name: string;
  domains: string[];
  servers: string[];
  enabled: boolean;
};
export const splitDnsEntries = (text: string) =>
  text
    .split(/[,\n\r]+/)
    .map((s) => s.trim())
    .filter(Boolean);
export function validateDnsPattern(pattern: string): boolean {
  const name = pattern.replace(/^(\+\.|\*\.)/, "");
  return (
    name.length > 0 &&
    name.length <= 253 &&
    name
      .split(".")
      .every((part) => /^[a-z0-9_](?:[a-z0-9_-]*[a-z0-9_])?$/i.test(part))
  );
}
export function validateDnsServer(server: string): boolean {
  if (!server || /\s/.test(server)) return false;
  try {
    const url = new URL(server.includes("://") ? server : "udp://" + server);
    if (
      !["udp:", "tcp:", "tls:", "https:", "quic:", "h3:"].includes(
        url.protocol,
      ) ||
      !url.hostname ||
      url.username ||
      url.password
    )
      return false;
    if (
      url.port &&
      (!/^\d+$/.test(url.port) || +url.port < 1 || +url.port > 65535)
    )
      return false;
    const host = url.hostname;
    if (/^[\d.]+$/.test(host))
      return (
        /^(\d{1,3}\.){3}\d{1,3}$/.test(host) &&
        host.split(".").every((part) => +part <= 255)
      );
    if (host.startsWith("[")) return true; // URL checks bracketed IPv6 syntax.
    return (
      validateDnsPattern(host) &&
      !host.startsWith("+.") &&
      !host.startsWith("*.")
    );
  } catch {
    return false;
  }
}
export function migrateDnsSettings(settings: Record<string, unknown>) {
  const { domainDns, dnsDomains, corporateDns, ...rest } = settings;
  if (Array.isArray(rest.dnsPolicies)) return rest;
  const domains =
    typeof dnsDomains === "string"
      ? splitDnsEntries(dnsDomains).map((domain) =>
          /^[+*]\./.test(domain) ? domain : "+." + domain,
        )
      : ["+.company.example", "+.internal.example", "+.service.example", "+.local"];
  const servers = splitDnsEntries(
    typeof domainDns === "string"
      ? domainDns
      : typeof corporateDns === "string" && corporateDns !== "127.0.0.1:1053"
        ? corporateDns
        : "10.20.0.53, 10.20.0.54",
  );
  return {
    ...rest,
    dnsPolicies:
      domains.length && servers.length
        ? [
            {
              id: "dns-migrated",
              name: "Из прежних настроек",
              domains,
              servers,
              enabled: true,
            },
          ]
        : [],
  };
}
export const initialSettings = {
  autoStart: true,
  minimized: true,
  notify: true,
  failover: true,
  theme: "system",
  dns: "1.1.1.1",
  dnsPolicies: [
    {
      id: "dns-vpn",
      name: "Адрес VPN-сервера",
      domains: ["vpn.company.example"],
      servers: [
        "https://dns.mullvad.net/dns-query",
        "https://dns10.quad9.net/dns-query",
      ],
      enabled: true,
    },
    {
      id: "dns-internal",
      name: "Внутренние домены",
      domains: ["+.company.example", "+.internal.example", "+.service.example", "+.local"],
      servers: [
        "udp://10.20.0.53:53#DIRECT",
        "udp://10.20.0.54:53#DIRECT",
      ],
      enabled: true,
    },
  ] as DnsPolicy[],
  exclusions: "localhost, *.local, *.company.example",
  defaultRoute: "vpn" as Route,
};
// A small UI simulator, not a replacement for Mihomo's rule engine.
export function previewConnections(
  rules: Rule[],
  fallback: Route,
): Connection[] {
  return connections.map((connection) => {
    const process =
      apps.find((app) => app.name === connection.app)?.exe.toLowerCase() || "";
    const matched = rules.find((rule) => {
      if (!rule.enabled) return false;
      const value = rule.value.toLowerCase();
      const host = connection.host.toLowerCase();
      switch (rule.kind) {
        case "DOMAIN":
          return host === value;
        case "DOMAIN-SUFFIX":
          return host === value || host.endsWith("." + value);
        case "DOMAIN-KEYWORD":
          return host.includes(value);
        case "PROCESS-NAME":
          return process === value;
        case "DST-PORT":
          return value === "443";
        case "IP-CIDR": {
          if (!/^(\d+\.){3}\d+$/.test(host) || validateRule(rule.kind, value))
            return false;
          const [network, bits] = value.split("/");
          const ip = (v: string) =>
            v.split(".").reduce((acc, octet) => ((acc << 8) | +octet) >>> 0, 0);
          const mask = +bits === 0 ? 0 : (0xffffffff << (32 - +bits)) >>> 0;
          return (ip(host) & mask) === (ip(network) & mask);
        }
        default:
          return false;
      }
    });
    const route = matched?.route || fallback;
    return {
      ...connection,
      route,
      rule: matched
        ? `${matched.kind} · ${matched.value}`
        : "MATCH · правило по умолчанию",
      down: route === "block" ? "0 Б" : connection.down,
      up: route === "block" ? "0 Б" : connection.up,
    };
  });
}
export const wait = (ms = 650) =>
  new Promise<void>((resolve) => setTimeout(resolve, ms));
export function readSaved<T>(key: string, fallback: T): T {
  try {
    const configuration = JSON.parse(
      localStorage.getItem("tiho.design.v2.configuration") || "{}",
    );
    const raw = Object.prototype.hasOwnProperty.call(configuration, key)
      ? JSON.stringify(configuration[key])
      : localStorage.getItem("tiho.design.v2." + key);
    if (!raw) return fallback;
    const parsed = JSON.parse(raw);
    // Preserve existing previews while migrating the removed route.
    if (key === "rules" && Array.isArray(parsed))
      return parsed.map((rule) => ({
        ...rule,
        route: rule.route === "work" ? "direct" : rule.route,
      })) as T;
    if (key === "settings" && parsed && typeof parsed === "object") {
      const rest = migrateDnsSettings(parsed);
      return {
        ...fallback,
        ...rest,
        defaultRoute:
          rest.defaultRoute === "work"
            ? "direct"
            : (rest.defaultRoute ?? initialSettings.defaultRoute),
      } as T;
    }
    return parsed;
  } catch {
    return fallback;
  }
}
export function save(key: string, value: unknown) {
  try {
    const configuration = JSON.parse(
      localStorage.getItem("tiho.design.v2.configuration") || "{}",
    );
    saveConfiguration({ ...configuration, [key]: value });
  } catch {
    /* Preview remains usable without storage. */
  }
}
export function saveConfiguration(configuration: Record<string, unknown>) {
  // One write: a failed restore leaves the previous configuration intact.
  localStorage.setItem(
    "tiho.design.v2.configuration",
    JSON.stringify(configuration),
  );
}
export function validateRule(kind: string, raw: string): string {
  const value = raw.trim();
  if (!value) return "Укажите значение правила.";
  if (
    kind === "IP-CIDR" &&
    (!/^(\d{1,3}\.){3}\d{1,3}\/\d{1,2}$/.test(value) ||
      value
        .split("/")[0]
        .split(".")
        .some((n) => +n > 255) ||
      +value.split("/")[1] > 32)
  )
    return "Введите подсеть, например 192.168.0.0/16.";
  if (
    kind === "IP-CIDR6" &&
    (!/^[\da-f:]+\/\d{1,3}$/i.test(value) || +value.split("/")[1] > 128)
  )
    return "Введите IPv6-подсеть, например fd00::/8.";
  if (kind.startsWith("DOMAIN") && /[\s/:]/.test(value))
    return "Укажите домен или слово без https://, пробелов и пути.";
  if (
    kind === "DST-PORT" &&
    (!/^\d+$/.test(value) || +value < 1 || +value > 65535)
  )
    return "Порт должен быть числом от 1 до 65535.";
  if (kind === "GEOIP" && !/^[a-z]{2}$/i.test(value))
    return "Укажите двухбуквенный код страны, например DE.";
  return "";
}
