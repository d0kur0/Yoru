import { TextArea } from "@heroui/react";
import { Input } from "@heroui/react";
import { Button } from "./controls";
import { Select } from "./Select";
import { useState } from "react";
import {
  ArrowRight,
  Check,
  FolderOpen,
  Info,
  Loader2,
  LockKeyhole,
  Plus,
  Trash2,
} from "lucide-react";
import { AppIcon, Empty, RouteBadge, SearchBox, Modal } from "./ui";
import {
  apps,
  protocols,
  routeNames,
  ruleKinds,
  validateRule,
  wait,
  type Route,
  type Rule,
  type Server,
} from "./model";

export function ServerForm({
  onClose,
  onSave,
}: {
  onClose: () => void;
  onSave: (server: Server) => void;
}) {
  const [mode, setMode] = useState("link");
  const [name, setName] = useState("");
  const [link, setLink] = useState("");
  const [host, setHost] = useState("");
  const [protocol, setProtocol] = useState("VLESS");
  const [port, setPort] = useState("443");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [transport, setTransport] = useState(false);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    let serverHost = host.trim();
    let serverName = name.trim();
    let serverProtocol = protocol;
    if (mode === "link") {
      try {
        const url = new URL(link.trim());
        const mapping: Record<string, string> = {
          vless: "VLESS",
          vmess: "VMess",
          trojan: "Trojan",
          ss: "Shadowsocks",
          hysteria2: "Hysteria2",
          hy2: "Hysteria2",
          hysteria: "Hysteria",
          tuic: "TUIC",
          wireguard: "WireGuard",
          socks5: "SOCKS5",
          http: "HTTP",
          https: "HTTP",
          snell: "Snell",
          ssh: "SSH",
          mieru: "Mieru",
          anytls: "AnyTLS",
        };
        serverProtocol = mapping[url.protocol.replace(":", "")];
        if (!serverProtocol) throw new Error();
        // Opaque VMess/SS payloads can contain credentials, so never save them as hosts.
        serverHost = /^[a-z0-9-]+(\.[a-z0-9-]+)+$/i.test(url.hostname)
          ? url.hostname
          : "imported.example.net";
        serverName =
          serverName || decodeURIComponent(url.hash.slice(1)) || "Новый сервер";
      } catch {
        setError("Вставьте ссылку сервера, например vless://… или trojan://…");
        return;
      }
    } else if (
      !serverName ||
      !serverHost ||
      /[\s/]/.test(serverHost) ||
      !/^\d+$/.test(port) ||
      +port < 1 ||
      +port > 65535
    ) {
      setError(
        "Укажите имя, адрес сервера без протокола и порт от 1 до 65535.",
      );
      return;
    }
    setBusy(true);
    await wait(600);
    onSave({
      id: crypto.randomUUID(),
      name: serverName,
      host: serverHost,
      code: "",
      country: "Личный сервер",
      protocol: serverProtocol,
      latency: 52,
    });
  }
  return (
    <Modal
      title="Новый сервер"
      subtitle="Импорт ссылки или ручная настройка."
      onClose={onClose}
    >
      <form className="modal-form" onSubmit={submit}>
        <div className="modal-body">
          <div className="segmented wide">
            <Button
              type="button"
              className={mode === "link" ? "active" : ""}
              onClick={() => {
                setMode("link");
                setError("");
              }}
            >
              По ссылке
            </Button>
            <Button
              type="button"
              className={mode === "manual" ? "active" : ""}
              onClick={() => {
                setMode("manual");
                setError("");
              }}
            >
              Вручную
            </Button>
          </div>
          {mode === "link" ? (
            <>
              <label className="form-label">
                Ссылка подключения
                <TextArea variant="secondary"
                  autoComplete="off"
                  spellCheck={false}
                  rows={5}
                  value={link}
                  onChange={(e) => setLink(e.target.value)}
                  placeholder="vless://, trojan://, ss://…"
                />
              </label>
              <p className="helper">
                Демоимпорт: распознаём тип и название. Ключи подключения не
                сохраняются.
              </p>
              <label className="form-label">
                Название<span className="optional">необязательно</span>
                <Input variant="secondary"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Например, Amsterdam"
                />
              </label>
            </>
          ) : (
            <>
              <label className="form-label">
                Название
                <Input variant="secondary"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="Мой сервер"
                />
              </label>
              <label className="form-label">
                Протокол
                <Select
                  aria-label="Протокол"
                  value={protocol}
                  onValueChange={(value) => setProtocol(value)}
                >
                  {protocols.map((p) => (
                    <option key={p}>{p}</option>
                  ))}
                </Select>
              </label>
              <div className="form-columns">
                <label className="form-label">
                  Адрес
                  <Input variant="secondary"
                    value={host}
                    onChange={(e) => setHost(e.target.value)}
                    placeholder="vpn.example.net"
                  />
                </label>
                <label className="form-label">
                  Порт
                  <Input variant="secondary"
                    inputMode="numeric"
                    value={port}
                    onChange={(e) => setPort(e.target.value)}
                  />
                </label>
              </div>
              <div className="field-section">
                <h3>Авторизация</h3>
                {["VLESS", "VMess", "TUIC"].includes(protocol) ? (
                  <label className="form-label">
                    UUID
                    <Input variant="secondary"
                      autoComplete="off"
                      spellCheck={false}
                      placeholder="00000000-0000-0000-0000-000000000000"
                    />
                  </label>
                ) : protocol === "WireGuard" ? (
                  <>
                    <label className="form-label">
                      Приватный ключ
                      <Input variant="secondary"
                        type="password"
                        autoComplete="new-password"
                        placeholder="Ключ клиента"
                      />
                    </label>
                    <label className="form-label">
                      Публичный ключ сервера
                      <Input variant="secondary" autoComplete="off" placeholder="Ключ сервера" />
                    </label>
                    <label className="form-label">
                      Адрес клиента
                      <Input variant="secondary" placeholder="10.0.0.2/32" />
                    </label>
                  </>
                ) : (
                  <label className="form-label">
                    Пароль
                    <Input variant="secondary"
                      type="password"
                      autoComplete="new-password"
                      placeholder="Пароль подключения"
                    />
                  </label>
                )}
                {protocol === "Shadowsocks" && (
                  <label className="form-label">
                    Шифрование
                    <Select aria-label="Шифрование" defaultValue="aes-128-gcm">
                      <option>aes-128-gcm</option>
                      <option>aes-256-gcm</option>
                      <option>chacha20-ietf-poly1305</option>
                      <option>2022-blake3-aes-128-gcm</option>
                    </Select>
                  </label>
                )}
              </div>
              <div className="field-section">
                <Button
                  className="disclosure"
                  type="button"
                  aria-expanded={transport}
                  onClick={() => setTransport(!transport)}
                >
                  <span>Транспорт и TLS</span>
                  <span>{transport ? "−" : "+"}</span>
                </Button>
                {transport && (
                  <div className="advanced-settings">
                    <label>
                      Транспорт
                      <Select aria-label="Транспорт" defaultValue="tcp">
                        <option value="tcp">TCP</option>
                        <option value="ws">WebSocket</option>
                        <option value="grpc">gRPC</option>
                        <option value="http">HTTP</option>
                      </Select>
                    </label>
                    <label>
                      Защита соединения
                      <Select
                        aria-label="Защита соединения"
                        defaultValue={protocol === "VLESS" ? "reality" : "tls"}
                      >
                        <option value="tls">TLS</option>
                        <option value="reality">Reality</option>
                        <option value="none">Без TLS</option>
                      </Select>
                    </label>
                    <label>
                      Имя сервера (SNI)
                      <Input variant="secondary" placeholder="example.com" />
                    </label>
                    {protocol === "VLESS" && (
                      <>
                        <label>
                          Public key
                          <Input variant="secondary" placeholder="Публичный ключ Reality" />
                        </label>
                        <label>
                          Short ID
                          <Input variant="secondary" placeholder="Идентификатор Reality" />
                        </label>
                      </>
                    )}
                  </div>
                )}
              </div>
              <div className="inline-info">
                <Info size={16} />
                <p>
                  Поля показывают будущий сценарий настройки. В демопул
                  сохраняются имя, адрес и протокол; ключи и пароли не
                  сохраняются.
                </p>
              </div>
            </>
          )}
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          <div className="import-support">
            <LockKeyhole size={16} />
            <span>Сохраняется только в этой дизайн-версии.</span>
          </div>
        </div>
        <footer className="modal-footer">
          <Button className="button" type="button" onClick={onClose}>
            Отмена
          </Button>
          <Button className="button primary" disabled={busy}>
            {busy ? <Loader2 size={15} className="spin" /> : <Plus size={15} />}
            Добавить сервер
          </Button>
        </footer>
      </form>
    </Modal>
  );
}

export function RuleForm({
  rule,
  seed,
  onClose,
  onSave,
  onDelete,
}: {
  rule: Rule | null;
  seed?: { kind: string; value: string; route: Route } | null;
  onClose: () => void;
  onSave: (rule: Rule) => void;
  onDelete: (id: string) => void;
}) {
  const [kind, setKind] = useState(rule?.kind || seed?.kind || "DOMAIN-SUFFIX");
  const [value, setValue] = useState(rule?.value || seed?.value || "");
  const [route, setRoute] = useState<Route>(
    rule?.route || seed?.route || "vpn",
  );
  const [error, setError] = useState("");
  const [processQuery, setProcessQuery] = useState("");
  const [confirmDelete, setConfirmDelete] = useState(false);
  const picker = kind === "PROCESS-NAME";
  function submit(e: React.FormEvent) {
    e.preventDefault();
    const issue = validateRule(kind, value);
    if (issue) {
      setError(issue);
      return;
    }
    onSave({
      id: rule?.id || crypto.randomUUID(),
      kind,
      value: value.trim(),
      route,
      enabled: rule?.enabled ?? true,
      system: rule?.system,
    });
  }
  const filteredApps = apps.filter((a) =>
    `${a.name} ${a.exe}`.toLowerCase().includes(processQuery.toLowerCase()),
  );
  return (
    <Modal
      title={rule ? "Редактировать правило" : "Новое правило"}
      subtitle="Условие и действие для совпавшего трафика."
      onClose={onClose}
    >
      <form className="modal-form" onSubmit={submit}>
        <div className="modal-body">
          <div className="step-label">
            <span>1</span>Что направляем
          </div>
          <label className="form-label">
            Тип условия
            <Select
              aria-label="Тип условия"
              value={kind}
              onValueChange={(value) => {
                setKind(value);
                setValue("");
                setError("");
              }}
            >
              {Object.entries(ruleKinds).map(([key, label]) => (
                <option key={key} value={key}>
                  {label}
                </option>
              ))}
            </Select>
          </label>
          {picker ? (
            <div className="process-picker">
              <div className="process-caption">
                <span>Приложения</span>
                <span className="demo-label">ДЕМОСПИСОК</span>
              </div>
              <SearchBox
                value={processQuery}
                onChange={setProcessQuery}
                placeholder="Найти приложение"
              />
              <div className="process-list">
                {filteredApps.map((app) => (
                  <Button
                    type="button"
                    key={app.exe}
                    className={
                      "process-option " + (value === app.exe ? "selected" : "")
                    }
                    onClick={() => {
                      setValue(app.exe);
                      setError("");
                    }}
                  >
                    <AppIcon type={app.icon} />
                    <span>
                      <strong>{app.name}</strong>
                      <small>{app.exe}</small>
                    </span>
                    {value === app.exe && <Check size={16} />}
                  </Button>
                ))}
                {!filteredApps.length && (
                  <Empty title="Не нашли приложение">
                    Укажите имя процесса ниже.
                  </Empty>
                )}
              </div>
              <label className="form-label">
                Или имя процесса
                <Input variant="secondary"
                  value={value}
                  onChange={(e) => setValue(e.target.value)}
                  placeholder="application.exe"
                />
              </label>
              <Button
                type="button"
                className="text-button"
                onClick={() => {
                  setKind("PROCESS-PATH");
                  setValue("");
                }}
              >
                <FolderOpen size={15} />
                Указать путь к программе
              </Button>
            </div>
          ) : (
            <label className="form-label">
              {kind.startsWith("DOMAIN") ? "Домен или значение" : "Значение"}
              <Input variant="secondary"
                value={value}
                onChange={(e) => {
                  setValue(e.target.value);
                  setError("");
                }}
                placeholder={
                  kind === "IP-CIDR"
                    ? "192.168.0.0/16"
                    : kind === "IP-CIDR6"
                      ? "fd00::/8"
                      : kind === "PROCESS-PATH"
                        ? "C:\\Program Files\\App\\app.exe"
                        : kind === "DOMAIN-KEYWORD"
                          ? "openai"
                          : kind === "DST-PORT"
                            ? "443"
                            : kind === "GEOIP"
                              ? "DE"
                              : kind === "GEOSITE"
                                ? "youtube"
                                : "example.com"
                }
                autoComplete="off"
                spellCheck={false}
              />
            </label>
          )}
          <div className="field-section">
            <div className="step-label">
              <span>2</span>Куда направляем
            </div>
            <div className="route-options">
              {(Object.keys(routeNames) as Route[]).map((r) => (
                <Button
                  key={r}
                  type="button"
                  className={route === r ? "selected" : ""}
                  aria-pressed={route === r}
                  onClick={() => setRoute(r)}
                >
                  <RouteBadge route={r} />
                  <span className="option-radio">
                    {route === r && <Check size={11} />}
                  </span>
                </Button>
              ))}
            </div>
          </div>
          <div className="rule-preview">
            <span>ПОЛУЧИТСЯ ТАК</span>
            <p>
              <strong>{value || "Условие"}</strong>
              <ArrowRight size={14} />
              {routeNames[route].toLowerCase()}
            </p>
            <code>
              {kind},{value || "…"},
              {route === "vpn"
                ? "Proxy"
                : route === "block"
                  ? "REJECT"
                  : "DIRECT"}
            </code>
          </div>
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          {rule && (
            <div className="delete-rule">
              {confirmDelete ? (
                <>
                  <span>Удалить это правило?</span>
                  <Button
                    type="button"
                    className="text-button danger"
                    onClick={() => onDelete(rule.id)}
                  >
                    Да, удалить
                  </Button>
                  <Button
                    type="button"
                    className="text-button"
                    onClick={() => setConfirmDelete(false)}
                  >
                    Отмена
                  </Button>
                </>
              ) : (
                <Button
                  type="button"
                  className="text-button danger"
                  onClick={() => setConfirmDelete(true)}
                >
                  <Trash2 size={14} />
                  Удалить правило
                </Button>
              )}
            </div>
          )}
        </div>
        <footer className="modal-footer">
          <Button type="button" className="button" onClick={onClose}>
            Отмена
          </Button>
          <Button className="button primary">
            <Check size={15} />
            Сохранить правило
          </Button>
        </footer>
      </form>
    </Modal>
  );
}
