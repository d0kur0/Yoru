import { Input } from "@heroui/react";
import { Button } from "./controls";
import { Select } from "./Select";
import { useEffect, useId, useRef, useState } from "react";
import { BookOpen, Check, Globe2, Pencil, Plus, Server, X } from "lucide-react";
import { validateDnsPattern, validateDnsServer } from "./model";

import {
  domainInfo,
  domainPattern,
  domainScopes,
  type DomainScope,
} from "./dnsEntryModel";

export function DnsGuide({
  kind = "all",
}: {
  kind?: "domain" | "server" | "all";
}) {
  return (
    <details className="dns-guide">
      <summary>
        <BookOpen size={15} />
        {kind === "all"
          ? "Как настроить DNS: примеры и обозначения"
          : kind === "domain"
            ? "Как выбрать охват домена"
            : "Какие адреса DNS можно добавить"}
      </summary>
      <div className="dns-guide-content">
        {kind !== "server" && (
          <>
            <p>
              Введите имя без https:// и пути. Охват выбирается в списке;
              обозначение для Mihomo формируется автоматически.
            </p>
            <dl>
              <div>
                <dt>
                  Только этот домен <code>example.com</code>
                </dt>
                <dd>
                  Совпадает только example.com. Адрес api.example.com сюда не
                  входит.
                </dd>
              </div>
              <div>
                <dt>
                  Домен и поддомены <code>+.example.com</code>
                </dt>
                <dd>
                  Совпадают example.com, api.example.com и любые более глубокие
                  поддомены.
                </dd>
              </div>
              <div>
                <dt>
                  Поддомены одного уровня <code>*.example.com</code>
                </dt>
                <dd>
                  Совпадает api.example.com, но не example.com и не
                  dev.api.example.com.
                </dd>
              </div>
            </dl>
            <p>
              Готовую маску из YAML тоже можно вставить: +. или *. будут
              распознаны.
            </p>
          </>
        )}
        {kind !== "domain" && (
          <>
            <p>
              Добавляйте адрес сервера, которому нужно отправлять DNS-запросы.
            </p>
            <dl>
              <div>
                <dt>IP или IP:порт</dt>
                <dd>
                  <code>10.20.0.53</code> или <code>10.20.0.53:53</code>
                </dd>
              </div>
              <div>
                <dt>Зашифрованный DNS</dt>
                <dd>
                  DoH: <code>https://dns.example.com/dns-query</code>
                  <br />
                  DoT: <code>tls://1.1.1.1:853</code>
                </dd>
              </div>
              <div>
                <dt>Явный прямой выход</dt>
                <dd>
                  <code>udp://10.20.0.53:53#DIRECT</code>
                  <br />
                  #DIRECT относится к запросам на этот DNS. Маршрут приложения
                  задаётся отдельно.
                </dd>
              </div>
            </dl>
          </>
        )}
        {kind === "all" && (
          <p>
            Исключения fake IP определяют, каким доменам возвращать настоящий
            IP. Они не выбирают DNS-сервер и не задают маршрут трафика.
          </p>
        )}
        <a
          href={
            kind === "domain"
              ? "https://wiki.metacubex.one/en/handbook/syntax/#domain-wildcards"
              : "https://wiki.metacubex.one/en/config/dns/"
          }
          target="_blank"
          rel="noreferrer"
        >
          Документация Mihomo ↗
        </a>
      </div>
    </details>
  );
}

export function DnsEntries({
  kind,
  values,
  onChange,
  onPendingChange,
}: {
  kind: "domain" | "server";
  values: string[];
  onChange: (values: string[]) => void;
  onPendingChange: (pending: boolean) => void;
}) {
  const [input, setInput] = useState("");
  const [scope, setScope] = useState<DomainScope>("all");
  const [editing, setEditing] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const field = useRef<HTMLInputElement>(null);
  const id = useId();
  const domain = kind === "domain";
  const title = domain ? "Домены" : "DNS-серверы";
  const pattern = domainPattern(input, scope);
  const effective = domainInfo(pattern);
  useEffect(() => {
    onPendingChange(!!input.trim() || editing !== null);
  }, [input, editing, onPendingChange]);
  function reset() {
    setInput("");
    setEditing(null);
    setError("");
  }
  function add() {
    const value = domain ? pattern : input.trim();
    if (
      !input.trim() ||
      !(domain ? validateDnsPattern(value) : validateDnsServer(value))
    ) {
      setError(
        domain
          ? "Введите домен, например example.com, без протокола и пути."
          : "Проверьте адрес DNS: например 10.20.0.53 или https://dns.example.com/dns-query.",
      );
      field.current?.focus();
      return;
    }
    if (
      values.some(
        (entry) =>
          (domain ? entry.toLowerCase() === value : entry === value) &&
          entry !== editing,
      )
    ) {
      setError("Этот адрес уже добавлен в список.");
      field.current?.focus();
      return;
    }
    onChange(
      editing === null
        ? [...values, value]
        : values.map((entry) => (entry === editing ? value : entry)),
    );
    setMessage(editing === null ? "Добавлено: " + value : "Изменено: " + value);
    reset();
    field.current?.focus();
  }
  return (
    <section className="dns-entry-field" aria-labelledby={id + "-title"}>
      <div className="dns-entry-heading">
        <h3 id={id + "-title"}>{title}</h3>
        <span>
          {values.length ? `${values.length} в списке` : "Список пока пуст"}
        </span>
      </div>
      {values.length > 0 && (
        <ul className="dns-entry-list">
          {values.map((value) => {
            const info = domainInfo(value);
            return (
              <li key={value} className={editing === value ? "is-editing" : ""}>
                {domain ? <Globe2 size={16} /> : <Server size={16} />}
                <div>
                  <strong>{domain ? info.name : value}</strong>
                  <small>
                    {domain
                      ? domainScopes[info.scope]
                      : value.startsWith("https://")
                        ? "DNS over HTTPS"
                        : value.startsWith("tls://")
                          ? "DNS over TLS"
                          : "DNS-сервер"}
                  </small>
                </div>
                <Button
                  type="button"
                  className="icon-button"
                  aria-label={"Изменить " + value}
                  onClick={() => {
                    setEditing(value);
                    setInput(domain ? info.name : value);
                    if (domain) setScope(info.scope);
                    setError("");
                    field.current?.focus();
                  }}
                >
                  <Pencil size={14} />
                </Button>
                <Button
                  type="button"
                  className="icon-button"
                  aria-label={"Убрать " + value}
                  onClick={() => {
                    onChange(values.filter((entry) => entry !== value));
                    if (editing === value) reset();
                    setMessage("Убрано: " + value);
                    field.current?.focus();
                  }}
                >
                  <X size={15} />
                </Button>
              </li>
            );
          })}
        </ul>
      )}
      <div className="dns-entry-composer">
        <div
          className={"dns-entry-input-row" + (domain ? " dns-domain-line" : "")}
        >
          {domain && (
            <label className="dns-scope-label">
              <span className="sr-only">Охват</span>
              <Select
                aria-label="Охват домена"
                value={scope}
                onValueChange={(value) => {
                  const next = value as DomainScope;
                  setScope(next);
                  if (/^[+*]\./.test(input.trim()))
                    setInput(domainInfo(input.trim()).name);
                  setError("");
                }}
              >
                <option value="all">Домен и поддомены</option>
                <option value="exact">Только этот домен</option>
                <option value="level">Поддомены одного уровня</option>
              </Select>
            </label>
          )}
          <Input variant="secondary"
            ref={field}
            aria-label={domain ? "Добавить домен" : "Добавить DNS-сервер"}
            aria-invalid={!!error}
            aria-describedby={id + (error ? "-error" : "-hint")}
            autoComplete="off"
            spellCheck={false}
            placeholder={domain ? "example.com" : "IP-адрес или DNS URL"}
            value={input}
            onChange={(e) => {
              const value = e.target.value;
              setInput(value);
              if (domain && /^[+*]\./.test(value.trim()))
                setScope(domainInfo(value.trim()).scope);
              setError("");
            }}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.nativeEvent.isComposing) {
                e.preventDefault();
                add();
              }
            }}
          />
          <Button type="button" className="button" onClick={add}>
            {editing ? <Check size={15} /> : <Plus size={15} />}
            {editing ? "Применить" : "Добавить"}
          </Button>
        </div>
        {editing && (
          <Button className="text-button" type="button" onClick={reset}>
            Отменить изменение
          </Button>
        )}
        <p className="dns-entry-hint" id={id + "-hint"}>
          {domain ? (
            <>
              {effective.scope === "all"
                ? "Сам домен и все его поддомены."
                : effective.scope === "exact"
                  ? "Только указанное имя, без поддоменов."
                  : "Один уровень поддоменов, без самого домена."}{" "}
              <span>
                В YAML:{" "}
                <code>
                  {input.trim() && validateDnsPattern(pattern)
                    ? pattern
                    : domainPattern("example.com", scope)}
                </code>
              </span>
            </>
          ) : (
            "Введите адрес и нажмите Enter или «Добавить»."
          )}
        </p>
        {error && (
          <p className="dns-entry-error" id={id + "-error"} role="alert">
            {error}
          </p>
        )}
      </div>
      <DnsGuide kind={kind} />
      <span className="sr-only" role="status">
        {message}
      </span>
    </section>
  );
}
