import "./hero.css";
import { Button } from "./controls";
import { TitleBar } from "./TitleBar";
import { useAppearance } from "./appearance";
import { Select } from "./Select";
import { useEffect, useRef, useState } from "react";
import {
  Activity,
  ArrowDown,
  ArrowUp,
  ArrowDownUp,
  ArrowRight,
  ArrowUpRight,
  Bell,
  Building2,
  Check,
  ChevronDown,
  ChevronRight,
  Download,
  Upload,
  Ellipsis,
  Globe2,
  Info,
  Layers,
  Loader2,
  Monitor,
  Network,
  Plus,
  Power,
  RefreshCw,
  Settings2,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
  X,
} from "lucide-react";
import {
  AppIcon,
  Checkmark,
  DeviceIcon,
  Empty,
  Flag,
  RouteBadge,
  SearchBox,
  Modal,
  Toggle,
  TrafficChart,
} from "./ui";
import {
  connections,
  initialRules,
  initialServers,
  initialSettings,
  previewConnections,
  readSaved,
  ruleKinds,
  save,
  saveConfiguration,
  wait,
  type Connection,
  type Route,
  type Rule,
  type Server,
} from "./model";
import { RuleForm, ServerForm } from "./forms";
import { DnsPage } from "./DnsPage";
import { BackupModal } from "./BackupModal";
import { downloadBackup } from "./backup";





type Page = "overview" | "servers" | "rules" | "dns" | "settings";
const nav = [
  { id: "overview", label: "Подключение", icon: Activity },
  { id: "servers", label: "Серверы", icon: Globe2 },
  { id: "rules", label: "Правила", icon: SlidersHorizontal },
  { id: "dns", label: "DNS", icon: Network },
  { id: "settings", label: "Настройки", icon: Settings2 },
] as const;

export default function App() {
  const appearance = useAppearance();
  const [page, setPage] = useState<Page>("overview");
  const [servers, setServers] = useState<Server[]>(() =>
    readSaved("servers", initialServers),
  );
  const [rules, setRules] = useState<Rule[]>(() =>
    readSaved("rules", initialRules),
  );
  const [settings, setSettings] = useState(() =>
    readSaved("settings", initialSettings),
  );
  const [selected, setSelected] = useState(() => readSaved("selected", "ams"));
  const [status, setStatus] = useState<
    "connected" | "disconnected" | "connecting"
  >("connected");
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState("all");
  const [period, setPeriod] = useState("1 мин");
  const [checking, setChecking] = useState(false);
  const [modal, setModal] = useState<
    "server" | "rule" | "connection" | "help" | "backup" | null
  >(null);
  const [editingRule, setEditingRule] = useState<Rule | null>(null);
  const [ruleSeed, setRuleSeed] = useState<{
    kind: string;
    value: string;
    route: Route;
  } | null>(null);
  const [detail, setDetail] = useState<Connection | null>(null);
  const [toast, setToast] = useState("");
  const [serverMenu, setServerMenu] = useState<string | null>(null);
  const [sessionSeconds, setSessionSeconds] = useState(2528);
  const toastTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const active = status === "connected";
  const current = servers.find((s) => s.id === selected) || servers[0];
  const backup = servers.find(
    (s) => s.id !== current?.id && s.latency !== null,
  );
  const available = servers.filter((s) => s.latency !== null);
  useEffect(() => {
    save("servers", servers);
  }, [servers]);
  useEffect(() => {
    save("rules", rules);
  }, [rules]);
  useEffect(() => {
    save("settings", settings);
  }, [settings]);
  useEffect(() => {
    setSettings((current) => current.theme === appearance.mode ? current : { ...current, theme: appearance.mode });
  }, [appearance.mode]);
  useEffect(() => {
    save("selected", selected);
  }, [selected]);
  useEffect(() => {
    if (!active) return;
    const t = setInterval(() => setSessionSeconds((s) => s + 1), 1000);
    return () => clearInterval(t);
  }, [active]);
  useEffect(() => () => clearTimeout(toastTimer.current), []);
  function announce(message: string) {
    setToast(message);
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(""), 4200);
  }
  function go(p: Page) {
    setPage(p);
    setQuery("");
    setFilter("all");
    setServerMenu(null);
    document.getElementById("main")?.scrollTo({ top: 0, behavior: "instant" });
  }
  async function connect() {
    if (active) {
      setStatus("disconnected");
      setSessionSeconds(0);
      announce("Демо-подключение отключено");
      return;
    }
    if (!current || current.latency === null) {
      announce("Выберите доступный сервер");
      go("servers");
      return;
    }
    setStatus("connecting");
    await wait(1100);
    setStatus("connected");
    announce("Демо-подключение установлено");
  }
  async function selectServer(server: Server) {
    if (server.latency === null || status === "connecting") return;
    setSelected(server.id);
    setServerMenu(null);
    if (active) {
      setStatus("connecting");
      await wait(750);
      setStatus("connected");
    }
    announce("Выбран " + server.name);
  }
  async function checkServers() {
    setChecking(true);
    await wait(900);
    setChecking(false);
    announce(
      "Демо-проверка: " +
        available.length +
        " из " +
        servers.length +
        " серверов доступны",
    );
  }
  async function simulateFailover() {
    if (!backup || !settings.failover || !active) return;
    setStatus("connecting");
    await wait(1100);
    setSelected(backup.id);
    setStatus("connected");
    if (settings.notify) announce("Демо: переключились на " + backup.name);
  }

  function exportDiagnostics() {
    const data = {
      demo: true,
      application: "Yoru · design preview",
      status,
      server: current?.name,
      rules: rules.map((r) => ({
        type: r.kind,
        enabled: r.enabled,
        route: r.route,
      })),
      notice: "Синтетические данные. Проверка сети и Mihomo не выполнялась.",
    };
    const url = URL.createObjectURL(
      new Blob([JSON.stringify(data, null, 2)], { type: "application/json" }),
    );
    const link = document.createElement("a");
    link.href = url;
    link.download = "tiho-demo-diagnostics.json";
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
    announce("Демо-отчёт сохранён");
  }
  const connectionRows = previewConnections(
    rules,
    settings.defaultRoute,
  ).filter(
    (c) =>
      (filter === "all" || c.route === filter) &&
      `${c.app} ${c.host}`.toLowerCase().includes(query.toLowerCase()),
  );
  const ruleRows = rules.filter(
    (r) =>
      (filter === "all" ||
        (filter === "apps"
          ? r.kind.startsWith("PROCESS")
          : filter === "domains"
            ? r.kind.startsWith("DOMAIN")
            : r.kind.startsWith("IP"))) &&
      `${r.value} ${ruleKinds[r.kind]}`
        .toLowerCase()
        .includes(query.toLowerCase()),
  );
  const serverRows = servers.filter((s) =>
    `${s.name} ${s.country} ${s.protocol}`
      .toLowerCase()
      .includes(query.toLowerCase()),
  );
  function editRule(rule: Rule | null = null) {
    setEditingRule(rule);
    setRuleSeed(null);
    setModal("rule");
  }
  function moveRule(id: string, delta: number) {
    setRules((list) => {
      const next = [...list];
      const index = next.findIndex((r) => r.id === id);
      if (index + delta < 0 || index + delta >= next.length) return list;
      [next[index], next[index + delta]] = [next[index + delta], next[index]];
      return next;
    });
    announce("Приоритет правила изменён");
  }
  const elapsed = [
    Math.floor(sessionSeconds / 3600),
    Math.floor(sessionSeconds / 60) % 60,
    sessionSeconds % 60,
  ]
    .map((n) => String(n).padStart(2, "0"))
    .join(":");

  return (
    <div className="app-shell">
      <TitleBar platform={appearance.platform} page={page} onNavigate={go} />
      <div className="workspace">
        <main id="main" className={"main-content page-" + page}>
          <div className="page-heading">
            <div>
              <h1>{nav.find((n) => n.id === page)?.label}</h1>
            </div>
            {page === "overview" ? (
              <div className="subtle-tag">
                <ShieldCheck size={14} />
                Режим TUN
              </div>
            ) : page === "servers" ? (
              <Button
                className="button primary"
                onClick={() => setModal("server")}
              >
                <Plus size={16} />
                Добавить сервер
              </Button>
            ) : page === "rules" ? (
              <Button className="button primary" onClick={() => editRule()}>
                <Plus size={16} />
                Добавить правило
              </Button>
            ) : (
              <span className="saved-label">
                <Check size={14} />
                Сохранено на устройстве
              </span>
            )}
          </div>
          {appearance.error && <p className="form-error" role="alert">{appearance.error}</p>}
          {page === "overview" && (
            <>
              <section
                className={"connection-panel " + (!active ? "inactive" : "")}
                aria-label="Состояние подключения"
              >
                <div className="connection-primary">
                  <div className="connection-title">
                    <span className="connection-emblem">
                      {status === "connecting" ? (
                        <Loader2 size={27} className="spin" />
                      ) : active ? (
                        <ShieldCheck size={28} strokeWidth={1.6} />
                      ) : (
                        <Power size={28} />
                      )}
                    </span>
                    <div>
                      <h2>
                        {active
                          ? "Подключено"
                          : status === "connecting"
                            ? "Подключение…"
                            : "Отключено"}
                      </h2>
                      <p>
                        {active
                          ? "TUN · маршрутизация по правилам"
                          : "TUN · подключение не активно"}
                      </p>
                    </div>
                  </div>
                  <Button
                    className={
                      "button connect-button " + (!active ? "primary" : "")
                    }
                    disabled={status === "connecting"}
                    onClick={connect}
                  >
                    <Power size={15} />
                    {active
                      ? "Отключить"
                      : status === "connecting"
                        ? "Подключение…"
                        : "Подключить"}
                  </Button>
                </div>
                <div className="connection-bottom">
                  <Button
                    className="server-choice"
                    onClick={() => go("servers")}
                  >
                    <Flag code={current?.code || ""} />
                    <span>
                      <strong>{current?.name || "Выбрать сервер"}</strong>
                      <small>
                        {current
                          ? `${current.country} · ${current.protocol}`
                          : "Добавьте свой сервер"}
                      </small>
                    </span>
                    <ChevronDown size={15} />
                  </Button>
                  <div className="connection-facts">
                    <div>
                      <span className="tiny-label">Задержка</span>
                      <strong>
                        <span className="latency-bars">
                          <i />
                          <i />
                          <i />
                        </span>
                        {active ? current?.latency : "—"}
                        <small>мс</small>
                      </strong>
                    </div>
                    <div>
                      <span className="tiny-label">Время в сети</span>
                      <strong className="tabular">
                        {active ? elapsed : "00:00:00"}
                      </strong>
                    </div>
                    <div className="failover-fact">
                      <span className="tiny-label">Резервирование</span>
                      <strong>
                        <span
                          className={
                            "status-dot " + (!settings.failover ? "off" : "")
                          }
                        />
                        {settings.failover ? "Автоматически" : "Выключено"}
                      </strong>
                    </div>
                  </div>
                </div>
              </section>
              <div className="route-summary">
                <DeviceIcon />
                <span className="route-wire" />
                <span className="route-summary-item">
                  <span className="route-node vpn">
                    <Globe2 size={14} />
                  </span>
                  Интернет<small>через VPN</small>
                </span>
                <span className="route-divider" />
                <span className="route-summary-item">
                  <span className="route-node">
                    <Globe2 size={14} />
                  </span>
                  Исключения<small>напрямую</small>
                </span>
                <Button className="text-button" onClick={() => go("rules")}>
                  Настроить правила
                  <ArrowUpRight size={14} />
                </Button>
              </div>
              <section className="traffic-section">
                <div className="section-heading">
                  <h2>Активность сети</h2>
                  <div
                    className="segmented compact"
                    aria-label="Период графика"
                  >
                    {["1 мин", "5 мин"].map((p) => (
                      <Button
                        key={p}
                        aria-pressed={period === p}
                        className={period === p ? "active" : ""}
                        onClick={() => setPeriod(p)}
                      >
                        {p}
                      </Button>
                    ))}
                  </div>
                </div>
                <div className="traffic-summary">
                  <div className="traffic-rate">
                    <span className="rate-label">
                      <ArrowDown size={13} />
                      Получено
                    </span>
                    <strong>
                      {active ? "12,4" : "0"}
                      <small>МБ/с</small>
                    </strong>
                  </div>
                  <div className="traffic-rate upload">
                    <span className="rate-label">
                      <ArrowUp size={13} />
                      Отправлено
                    </span>
                    <strong>
                      {active ? "1,2" : "0"}
                      <small>МБ/с</small>
                    </strong>
                  </div>
                  <div className="session-total">
                    <span>За демосессию</span>
                    <strong>
                      ↓ {active ? "1,84 ГБ" : "0 Б"}{" "}
                      <span>↑ {active ? "126 МБ" : "0 Б"}</span>
                    </strong>
                  </div>
                </div>
                <TrafficChart active={active} period={period} />
              </section>
              <section className="connections-section">
                <div className="section-heading">
                  <h2>
                    Соединения
                    <span className="count">
                      {active ? connections.length : 0}
                    </span>
                  </h2>
                  <SearchBox
                    value={query}
                    onChange={setQuery}
                    placeholder="Приложение или домен"
                  />
                </div>
                <div className="tabs" aria-label="Маршрут соединений">
                  {[
                    ["all", "Все"],
                    ["vpn", "Через VPN"],
                    ["direct", "Напрямую"],
                    ["block", "Блокировано"],
                  ].map(([id, label]) => (
                    <Button
                      className={filter === id ? "active" : ""}
                      aria-pressed={filter === id}
                      onClick={() => setFilter(id)}
                      key={id}
                    >
                      {label}
                    </Button>
                  ))}
                </div>
                {!active ? (
                  <Empty
                    title={
                      status === "connecting"
                        ? "Устанавливаем соединение"
                        : "Сейчас соединений нет"
                    }
                  >
                    Подключите VPN, чтобы увидеть демонстрационный трафик
                    приложений.
                  </Empty>
                ) : !connectionRows.length ? (
                  <Empty title="Соединения не найдены">
                    Попробуйте другое приложение, домен или маршрут.
                  </Empty>
                ) : (
                  <div className="table-scroll">
                    <table className="connections-table">
                      <thead>
                        <tr>
                          <th>Приложение / назначение</th>
                          <th>Маршрут</th>
                          <th className="numeric">
                            Получено <ArrowDown size={11} />
                          </th>
                          <th className="numeric">Отправлено</th>
                          <th>
                            <span className="sr-only">Подробности</span>
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {connectionRows.map((c) => (
                          <tr key={c.id}>
                            <td>
                              <Button
                                className="connection-link"
                                onClick={() => {
                                  setDetail(c);
                                  setModal("connection");
                                }}
                              >
                                <AppIcon type={c.icon} />
                                <span>
                                  <strong>{c.app}</strong>
                                  <small>{c.host}</small>
                                </span>
                              </Button>
                            </td>
                            <td>
                              <RouteBadge route={c.route} />
                            </td>
                            <td className="numeric">{c.down}</td>
                            <td className="numeric muted">{c.up}</td>
                            <td>
                              <Button
                                className="icon-button"
                                aria-label={"Подробности соединения " + c.host}
                                onClick={() => {
                                  setDetail(c);
                                  setModal("connection");
                                }}
                              >
                                <ChevronRight size={15} />
                              </Button>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                )}
              </section>
            </>
          )}
          {page === "servers" && (
            <>
              <section className="failover-banner">
                <span className="feature-icon">
                  <Layers size={21} />
                </span>
                <div>
                  <h2>Автоматическое резервирование</h2>
                  <p>
                    {settings.failover
                      ? "Если основной сервер недоступен, подключим следующий из списка."
                      : "Сервер выбирается вручную. Включите резервирование для автоматической смены."}
                  </p>
                </div>
                <Toggle
                  checked={settings.failover}
                  label="Автоматическое резервирование"
                  onChange={() =>
                    setSettings((s) => ({ ...s, failover: !s.failover }))
                  }
                />
              </section>
              <div className="server-layout">
                <section className="server-list">
                  <div className="list-toolbar">
                    <SearchBox
                      value={query}
                      onChange={setQuery}
                      placeholder="Название, страна, протокол"
                    />
                    <Button
                      className="button"
                      disabled={checking}
                      onClick={checkServers}
                    >
                      <RefreshCw size={14} className={checking ? "spin" : ""} />
                      {checking ? "Проверяем…" : "Проверить"}
                    </Button>
                  </div>
                  <div className="list-caption">
                    <span>
                      МОИ СЕРВЕРЫ <b>{servers.length}</b>
                    </span>
                    <span>ЗАДЕРЖКА</span>
                  </div>
                  {serverRows.length ? (
                    serverRows.map((server) => (
                      <article
                        key={server.id}
                        className={
                          "server-row " +
                          (current?.id === server.id ? "current" : "")
                        }
                      >
                        <Button
                          className="server-select"
                          disabled={
                            server.latency === null || status === "connecting"
                          }
                          onClick={() => selectServer(server)}
                        >
                          <span className="server-radio">
                            {current?.id === server.id && <Check size={12} />}
                          </span>
                          <Flag code={server.code} />
                          <span className="server-info">
                            <strong>
                              {server.name}
                              {current?.id === server.id && (
                                <span className="small-pill">Выбран</span>
                              )}
                            </strong>
                            <small>
                              {server.country} · {server.protocol}
                            </small>
                          </span>
                          <span
                            className={
                              "server-latency " +
                              (server.latency === null ? "unavailable" : "")
                            }
                          >
                            {checking ? (
                              <Loader2 size={16} className="spin" />
                            ) : server.latency === null ? (
                              "Нет ответа"
                            ) : (
                              <>
                                <span className="latency-bars">
                                  <i />
                                  <i />
                                  <i />
                                </span>
                                {server.latency}
                                <small>мс</small>
                              </>
                            )}
                          </span>
                        </Button>
                        <Button
                          className="icon-button"
                          aria-label={"Действия с сервером " + server.name}
                          aria-expanded={serverMenu === server.id}
                          onClick={() =>
                            setServerMenu(
                              serverMenu === server.id ? null : server.id,
                            )
                          }
                        >
                          <Ellipsis size={18} />
                        </Button>
                        {serverMenu === server.id && (
                          <div className="server-menu">
                            <code>{server.host}</code>
                            <Button
                              onClick={() => {
                                setServers((ss) => [
                                  server,
                                  ...ss.filter((s) => s.id !== server.id),
                                ]);
                                setServerMenu(null);
                                announce(
                                  "Сервер поставлен первым в очередь резервирования",
                                );
                              }}
                            >
                              <ArrowUp size={14} />
                              Первым в резерве
                            </Button>
                            <Button
                              disabled={current?.id === server.id}
                              onClick={() => {
                                setServers((ss) =>
                                  ss.filter((s) => s.id !== server.id),
                                );
                                setServerMenu(null);
                                announce("Сервер удалён");
                              }}
                            >
                              <Trash2 size={14} />
                              Удалить сервер
                            </Button>
                          </div>
                        )}
                      </article>
                    ))
                  ) : (
                    <Empty title="Серверы не найдены">
                      Измените поиск или добавьте новый сервер.
                    </Empty>
                  )}
                  <Button
                    className="add-list-button"
                    onClick={() => setModal("server")}
                  >
                    <Plus size={17} />
                    Добавить по ссылке или вручную
                  </Button>
                  <div className="list-footnote">
                    <span className="status-dot" />
                    {available.length} из {servers.length} доступны
                    <span>Демо-проверка</span>
                  </div>
                </section>
                <aside className="context-panel">
                  <div className="eyebrow">МАРШРУТ ПОДКЛЮЧЕНИЯ</div>
                  <div className="priority-item">
                    <span className="priority-number">1</span>
                    <div>
                      <small>Основной сервер</small>
                      <strong>{current?.name || "Не выбран"}</strong>
                      <span>{current?.protocol || "Добавьте сервер"}</span>
                    </div>
                    {current && <Flag code={current.code} />}
                  </div>
                  <div className="priority-connector" />
                  <div
                    className={
                      "priority-item " + (!settings.failover ? "dimmed" : "")
                    }
                  >
                    <span className="priority-number">2</span>
                    <div>
                      <small>Следующий в резерве</small>
                      <strong>
                        {settings.failover
                          ? backup?.name || "Нет доступных"
                          : "Резерв отключён"}
                      </strong>
                      <span>
                        {settings.failover ? backup?.protocol : "Выбор вручную"}
                      </span>
                    </div>
                    {settings.failover && backup && <Flag code={backup.code} />}
                  </div>
                  <div className="context-note">
                    <Bell size={16} />
                    <p>Сообщим, если пришлось переключить сервер.</p>
                  </div>
                  <Button
                    className="button full"
                    disabled={!active || !settings.failover || !backup}
                    onClick={simulateFailover}
                  >
                    <ArrowDownUp size={14} />
                    Проверить демопереключение
                  </Button>
                  <p className="helper">
                    Симуляция сбоя для проверки интерфейса.
                  </p>
                </aside>
              </div>
            </>
          )}
          {page === "rules" && (
            <>
              <div className="rules-note">
                <Info size={16} />
                <span>
                  Правила применяются сверху вниз. Срабатывает первое
                  совпадение.
                </span>
                <span className="muted">
                  {rules.filter((r) => r.enabled).length} активны
                </span>
              </div>
              <div className="rules-toolbar">
                <div className="tabs">
                  {[
                    ["all", "Все правила"],
                    ["apps", "Приложения"],
                    ["domains", "Домены"],
                    ["ips", "Подсети"],
                  ].map(([id, label]) => (
                    <Button
                      key={id}
                      className={filter === id ? "active" : ""}
                      aria-pressed={filter === id}
                      onClick={() => setFilter(id)}
                    >
                      {label}
                    </Button>
                  ))}
                </div>
                <SearchBox
                  value={query}
                  onChange={setQuery}
                  placeholder="Найти правило"
                />
              </div>
              <div className="table-scroll">
                {ruleRows.length ? (
                  <table className="rules-table">
                    <thead>
                      <tr>
                        <th className="order-column">№</th>
                        <th>Условие</th>
                        <th>Маршрут</th>
                        <th>Включено</th>
                        <th className="numeric">Приоритет</th>
                      </tr>
                    </thead>
                    <tbody>
                      {ruleRows.map((rule) => (
                        <tr
                          key={rule.id}
                          className={!rule.enabled ? "disabled-row" : ""}
                        >
                          <td className="rule-number">
                            {String(rules.indexOf(rule) + 1).padStart(2, "0")}
                          </td>
                          <td>
                            <Button
                              className="rule-edit"
                              onClick={() => editRule(rule)}
                            >
                              <span className={"rule-icon"}>
                                {rule.system ? (
                                  <Building2 size={18} />
                                ) : rule.kind.startsWith("PROCESS") ? (
                                  <Monitor size={18} />
                                ) : rule.kind.startsWith("IP") ? (
                                  <Network size={18} />
                                ) : (
                                  <Globe2 size={18} />
                                )}
                              </span>
                              <span>
                                <strong>
                                  {rule.value}
                                  {rule.system && (
                                    <span className="system-label">Пример</span>
                                  )}
                                </strong>
                                <small>{ruleKinds[rule.kind]}</small>
                              </span>
                            </Button>
                          </td>
                          <td>
                            <RouteBadge route={rule.route} />
                          </td>
                          <td>
                            <Toggle
                              checked={rule.enabled}
                              label={"Правило " + rule.value}
                              onChange={() =>
                                setRules((rr) =>
                                  rr.map((r) =>
                                    r.id === rule.id
                                      ? { ...r, enabled: !r.enabled }
                                      : r,
                                  ),
                                )
                              }
                            />
                          </td>
                          <td>
                            <div className="row-actions">
                              <Button
                                className="icon-button"
                                disabled={rules.indexOf(rule) === 0}
                                aria-label={"Поднять " + rule.value}
                                onClick={() => moveRule(rule.id, -1)}
                              >
                                <ArrowUp size={14} />
                              </Button>
                              <Button
                                className="icon-button"
                                disabled={
                                  rules.indexOf(rule) === rules.length - 1
                                }
                                aria-label={"Опустить " + rule.value}
                                onClick={() => moveRule(rule.id, 1)}
                              >
                                <ArrowDown size={14} />
                              </Button>
                              <Button
                                className="icon-button"
                                aria-label={"Редактировать " + rule.value}
                                onClick={() => editRule(rule)}
                              >
                                <ChevronRight size={15} />
                              </Button>
                            </div>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <Empty title="Таких правил пока нет">
                    Добавьте правило или измените условия поиска.
                  </Empty>
                )}
              </div>
              <div className="default-route">
                <span className="rule-icon">
                  <ArrowRight size={18} />
                </span>
                <div>
                  <strong>Всё остальное</strong>
                  <p>Если ни одно правило не подошло</p>
                </div>
                <Select
                  aria-label="Маршрут по умолчанию"
                  value={settings.defaultRoute}
                  onValueChange={(value) =>
                    setSettings((s) => ({
                      ...s,
                      defaultRoute: value as Route,
                    }))
                  }
                >
                  <option value="vpn">Через VPN</option>
                  <option value="direct">Напрямую</option>
                  <option value="block">Блокировать</option>
                </Select>
              </div>
            </>
          )}
          {page === "dns" && (
            <DnsPage settings={settings} onChange={setSettings} />
          )}
          {page === "settings" && (
            <div className="settings-layout">
              <div className="settings-main">
                <section className="settings-section">
                  <h2>Запуск и уведомления</h2>
                  {(
                    [
                      {
                        key: "autoStart",
                        name: "Запускать вместе с системой",
                        help: "Запускать при входе в систему.",
                      },
                      {
                        key: "minimized",
                        name: "Начинать в свёрнутом виде",
                        help: "Запускать в трее.",
                      },
                      {
                        key: "notify",
                        name: "Уведомлять о смене сервера",
                        help: "Системные уведомления о резервировании и потере соединения.",
                      },
                    ] as const
                  ).map((row) => (
                    <div className="setting-row" key={row.key}>
                      <div>
                        <strong>{row.name}</strong>
                        <p>{row.help}</p>
                      </div>
                      <Toggle
                        label={row.name}
                        checked={settings[row.key]}
                        onChange={() =>
                          setSettings((s) => ({ ...s, [row.key]: !s[row.key] }))
                        }
                      />
                    </div>
                  ))}
                </section>
                <section className="settings-section">
                  <h2>Внешний вид</h2>
                  <div className="setting-row">
                    <div>
                      <strong>Тема оформления</strong>
                    </div>
                    <Select aria-label="Тема оформления" value={appearance.mode} onValueChange={appearance.change}>
                      <option value="dark">Тёмная</option><option value="light">Светлая</option><option value="system">Как в системе</option>
                    </Select>
                  </div>
                </section>
                <section className="settings-section">
                  <h2>Подключение</h2>
                  <div className="setting-row">
                    <div>
                      <strong>Режим TUN</strong>
                      <p>Маршрутизация трафика всех приложений.</p>
                    </div>
                    <span className="subtle-tag">
                      <Check size={13} />
                      По умолчанию
                    </span>
                  </div>
                  <Button className="text-button" onClick={() => go("dns")}>
                    <Network size={15} />
                    Настроить DNS
                    <ChevronRight size={14} />
                  </Button>
                </section>
                <section className="settings-section">
                  <h2>Резервная копия</h2>
                  <div className="setting-row backup-setting">
                    <div>
                      <strong>Вся конфигурация в одном файле</strong>
                      <p>
                        Серверы, правила, DNS и настройки. Для переноса на
                        другой компьютер или восстановления.
                      </p>
                    </div>
                    <div className="backup-actions">
                      <Button
                        className="button"
                        onClick={() => {
                          downloadBackup({
                            servers,
                            rules,
                            settings,
                            selected: current?.id || "",
                          });
                          announce("Резервная копия подготовлена");
                        }}
                      >
                        <Download size={15} />
                        Экспортировать
                      </Button>
                      <Button
                        className="button"
                        onClick={() => setModal("backup")}
                      >
                        <Upload size={15} />
                        Импортировать
                      </Button>
                    </div>
                  </div>
                  <p className="helper">
                    В демоверсии сохраняются все данные интерфейса. Ключи
                    подключения пока не хранятся.
                  </p>
                </section>
                <section className="settings-section">
                  <h2>Диагностика</h2>
                  <div className="setting-row">
                    <div>
                      <strong>Отчёт о работе</strong>
                      <p>Состояние подключения и правил в одном файле.</p>
                    </div>
                    <Button className="button" onClick={exportDiagnostics}>
                      <Download size={15} />
                      Выгрузить
                    </Button>
                  </div>
                </section>
              </div>
              <aside className="about-panel">
                <h2>О приложении</h2>
                <div className="about-version">
                  <span>Приложение</span>
                  <strong>
                    0.1.0 <span className="demo-label">ДЕМО</span>
                  </strong>
                </div>
                <div className="about-version">
                  <span>Ядро Mihomo</span>
                  <strong>Не подключено</strong>
                </div>
                <Button
                  className="button full"
                  onClick={async () => {
                    setChecking(true);
                    await wait(700);
                    setChecking(false);
                    announce(
                      "В дизайн-версии обновления недоступны. Ядро Mihomo пока не подключено.",
                    );
                  }}
                  disabled={checking}
                >
                  <RefreshCw size={14} className={checking ? "spin" : ""} />
                  {checking ? "Проверяем…" : "Проверить обновления"}
                </Button>
                <p className="helper">
                  Демонстрационные данные.
                  <br />
                  Сетевые функции работают на моках.
                </p>
              </aside>
            </div>
          )}
        </main>
        <footer className="app-footer">
          <span>
            <span className="status-dot neutral" />
            Демо · Mihomo не подключено
          </span>
          <span>Системная сеть не изменяется</span>
        </footer>
      </div>
      {toast && (
        <div className="toast" role="status">
          <Checkmark />
          <span>{toast}</span>
          <Button aria-label="Скрыть сообщение" onClick={() => setToast("")}>
            <X size={14} />
          </Button>
        </div>
      )}
      {modal === "backup" && (
        <BackupModal
          onClose={() => setModal(null)}
          onRestore={(configuration) => {
            saveConfiguration(configuration);
            setServers(configuration.servers);
            setRules(configuration.rules);
              setSettings(configuration.settings);
              void appearance.change(configuration.settings.theme);
            setSelected(configuration.selected);
            setStatus("disconnected");
            setSessionSeconds(0);
            setModal(null);
            setServerMenu(null);
            announce("Конфигурация восстановлена из копии");
          }}
        />
      )}
      {modal === "server" && (
        <ServerForm
          onClose={() => setModal(null)}
          onSave={(server) => {
            setServers((s) => [...s, server]);
            if (!current) setSelected(server.id);
            setModal(null);
            announce("Сервер добавлен в демопул");
          }}
        />
      )}
      {modal === "rule" && (
        <RuleForm
          rule={editingRule}
          seed={ruleSeed}
          onClose={() => setModal(null)}
          onSave={(rule) => {
            setRules((rs) =>
              editingRule
                ? rs.map((r) => (r.id === rule.id ? rule : r))
                : [...rs, rule],
            );
            setModal(null);
            announce(editingRule ? "Правило сохранено" : "Правило добавлено");
          }}
          onDelete={(id) => {
            setRules((rs) => rs.filter((r) => r.id !== id));
            setModal(null);
            announce("Правило удалено");
          }}
        />
      )}
      {modal === "connection" && detail && (
        <Modal
          title="Детали соединения"
          subtitle="Демонстрационные данные"
          onClose={() => setModal(null)}
        >
          <div className="modal-body">
            <div className="detail-app">
              <AppIcon type={detail.icon} />
              <div>
                <h3>{detail.app}</h3>
                <p>{detail.host}</p>
              </div>
            </div>
            <dl className="detail-list">
              <div>
                <dt>Маршрут</dt>
                <dd>
                  <RouteBadge route={detail.route} />
                </dd>
              </div>
              <div>
                <dt>Сервер</dt>
                <dd>
                  {detail.route === "vpn"
                    ? current?.name
                    : detail.route === "block"
                      ? "Соединение запрещено"
                      : "Маршрут операционной системы"}
                </dd>
              </div>
              <div>
                <dt>Протокол</dt>
                <dd>TCP · TLS</dd>
              </div>
              <div>
                <dt>Получено</dt>
                <dd>{detail.down}</dd>
              </div>
              <div>
                <dt>Отправлено</dt>
                <dd>{detail.up}</dd>
              </div>
            </dl>
            <div className="field-section">
              <h3>Почему выбран этот маршрут</h3>
              <code className="rule-code">{detail.rule}</code>
              <p className="helper">
                Первое совпавшее правило определяет маршрут соединения.
              </p>
            </div>
            <Button
              className="button full"
              onClick={() => {
                setEditingRule(null);
                setRuleSeed({
                  kind: /^\d+\./.test(detail.host) ? "IP-CIDR" : "DOMAIN",
                  value: /^\d+\./.test(detail.host)
                    ? detail.host + "/32"
                    : detail.host,
                  route: detail.route,
                });
                setModal("rule");
              }}
            >
              Создать своё правило
              <Plus size={15} />
            </Button>
          </div>
        </Modal>
      )}
      {modal === "help" && (
        <Modal
          title="Всё под контролем"
          subtitle="Помощь и диагностика"
          onClose={() => setModal(null)}
        >
          <div className="modal-body">
            <div className="help-intro">
              <ShieldCheck size={28} />
              <h3>Дизайн-версия приложения</h3>
              <p>
                Можно подключаться, выбирать серверы и собирать правила. Все
                сетевые состояния демонстрационные, реальный VPN не запускается.
              </p>
            </div>
            <h3>Что происходит с трафиком</h3>
            <div className="help-route">
              <RouteBadge route="vpn" />
              <p>Трафик через выбранный сервер Mihomo.</p>
            </div>
            <div className="help-route">
              <RouteBadge route="block" />
              <p>Соединение запрещено правилом.</p>
            </div>
            <div className="help-route">
              <RouteBadge route="direct" />
              <p>
                Подключение без личного прокси. Дальнейший маршрут выбирает ОС.
              </p>
            </div>
            <Button className="button full" onClick={exportDiagnostics}>
              <Download size={15} />
              Скачать демо-диагностику
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}
