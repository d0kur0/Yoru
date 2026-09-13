import { Input } from "@heroui/react";
import { Button } from "./controls";
import { useState } from "react";
import {
  ArrowRight,
  Check,
  ChevronRight,
  Info,
  Plus,
  Trash2,
} from "lucide-react";
import { Modal, Toggle } from "./ui";
import { DnsEntries, DnsGuide } from "./DnsEntries";
import {
  initialSettings,
  splitDnsEntries,
  validateDnsPattern,
  validateDnsServer,
  type DnsPolicy,
} from "./model";

type Settings = typeof initialSettings;
export function DnsPage({
  settings,
  onChange,
}: {
  settings: Settings;
  onChange: (next: Settings) => void;
}) {
  const [editing, setEditing] = useState<DnsPolicy | null | undefined>(
    undefined,
  );
  const [general, setGeneral] = useState<"dns" | "exclusions" | null>(null);
  const [draft, setDraft] = useState<string[]>([]);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  return (
    <div className="dns-page">
      <section className="dns-default">
        <div>
          <span className="eyebrow">ДЛЯ ОСТАЛЬНЫХ ДОМЕНОВ</span>
          <h2>DNS по умолчанию</h2>
          <p>Используется, если для домена нет отдельного соответствия.</p>
        </div>
        <div className="dns-default-addresses">
          {splitDnsEntries(settings.dns).map((address, i) => (
            <code key={i}>{address}</code>
          ))}
        </div>
        <Button
          className="button"
          onClick={() => {
            setGeneral("dns");
            setDraft(splitDnsEntries(settings.dns));
            setError("");
          }}
        >
          Изменить
        </Button>
      </section>
      <section className="dns-mappings">
        <div className="dns-section-heading">
          <div>
            <h2>Домены → DNS</h2>
            <p>У каждой записи свои домены и серверы разрешения имён.</p>
          </div>
          <Button className="button primary" onClick={() => setEditing(null)}>
            <Plus size={16} />
            Добавить соответствие
          </Button>
        </div>
        {settings.dnsPolicies.length ? (
          <div className="dns-list">
            <div className="dns-list-heading">
              <span>Домены и маски</span>
              <span>DNS-серверы</span>
              <span className="sr-only">Действия</span>
            </div>
            {settings.dnsPolicies.map((policy) => (
              <div
                className={"dns-row " + (!policy.enabled ? "dns-disabled" : "")}
                key={policy.id}
              >
                <div>
                  <strong>{policy.name || "Без названия"}</strong>
                  <div className="dns-domains">
                    {policy.domains.map((domain) => (
                      <code key={domain}>{domain}</code>
                    ))}
                  </div>
                </div>
                <div className="dns-addresses">
                  {policy.servers.map((server) => (
                    <code key={server}>{server}</code>
                  ))}
                </div>
                <div className="dns-row-actions">
                  <Toggle
                    label={
                      "Использовать DNS: " + (policy.name || policy.domains[0])
                    }
                    checked={policy.enabled}
                    onChange={() =>
                      onChange({
                        ...settings,
                        dnsPolicies: settings.dnsPolicies.map((p) =>
                          p.id === policy.id
                            ? { ...p, enabled: !p.enabled }
                            : p,
                        ),
                      })
                    }
                  />
                  <Button
                    className="icon-button"
                    aria-label={
                      "Изменить DNS: " + (policy.name || policy.domains[0])
                    }
                    onClick={() => setEditing(policy)}
                  >
                    <ChevronRight size={17} />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="dns-empty">
            <h3>Все домены используют DNS по умолчанию</h3>
            <p>
              Добавьте соответствие, чтобы направить запросы для отдельных
              доменов на другие DNS-серверы.
            </p>
          </div>
        )}
        <DnsGuide />
      </section>
      <section className="dns-exclusions">
        <div>
          <h2>Исключения fake IP</h2>
          <p>Домены, которым нужен настоящий IP-адрес в ответе DNS.</p>
          <div className="dns-domains">
            {splitDnsEntries(settings.exclusions).map((domain, i) => (
              <code key={i}>{domain}</code>
            ))}
            {!settings.exclusions.trim() && (
              <span className="helper">Исключений нет</span>
            )}
          </div>
        </div>
        <Button
          className="button"
          onClick={() => {
            setGeneral("exclusions");
            setDraft(splitDnsEntries(settings.exclusions));
            setError("");
          }}
        >
          Настроить исключения
        </Button>
      </section>
      <p className="dns-explanation">
        <Info size={15} />
        <span>
          Здесь выбирается, кто разрешает имя домена. Путь самого соединения
          задаётся в разделе «Правила». Настройки сохраняются в дизайн-версии;
          ядро Mihomo пока не подключено.
        </span>
      </p>
      {editing !== undefined && (
        <DnsPolicyForm
          policy={editing}
          policies={settings.dnsPolicies}
          onClose={() => setEditing(undefined)}
          onSave={(policy) => {
            onChange({
              ...settings,
              dnsPolicies: editing
                ? settings.dnsPolicies.map((p) =>
                    p.id === policy.id ? policy : p,
                  )
                : [...settings.dnsPolicies, policy],
            });
            setEditing(undefined);
          }}
          onDelete={() => {
            onChange({
              ...settings,
              dnsPolicies: settings.dnsPolicies.filter(
                (p) => p.id !== editing?.id,
              ),
            });
            setEditing(undefined);
          }}
        />
      )}
      {general && (
        <Modal
          wide
          title={general === "dns" ? "DNS по умолчанию" : "Исключения fake IP"}
          onClose={() => setGeneral(null)}
        >
          <form
            className="modal-form"
            onSubmit={(e) => {
              e.preventDefault();
              if (pending) {
                setError(
                  "Сначала добавьте введённый адрес в список или очистите поле.",
                );
                return;
              }
              const entries = draft;
              if (
                general === "dns" &&
                (!entries.length || entries.some((v) => !validateDnsServer(v)))
              ) {
                setError(
                  "Укажите корректные IP-адреса или DNS URL, по одному на строке.",
                );
                return;
              }
              if (
                general === "exclusions" &&
                entries.some((v) => !validateDnsPattern(v))
              ) {
                setError("Укажите домены или маски, например +.example.com.");
                return;
              }
              onChange({
                ...settings,
                [general]: [...new Set(entries)].join("\n"),
              });
              setGeneral(null);
            }}
          >
            <div className="modal-body">
              <DnsEntries
                kind={general === "dns" ? "server" : "domain"}
                values={draft}
                onChange={(values) => {
                  setDraft(values);
                  setError("");
                }}
                onPendingChange={setPending}
              />
              {error && (
                <p className="form-error" role="alert">
                  {error}
                </p>
              )}
            </div>
            <footer className="modal-footer">
              <Button
                type="button"
                className="button"
                onClick={() => setGeneral(null)}
              >
                Отмена
              </Button>
              <Button className="button primary">Сохранить</Button>
            </footer>
          </form>
        </Modal>
      )}
    </div>
  );
}

function DnsPolicyForm({
  policy,
  policies,
  onClose,
  onSave,
  onDelete,
}: {
  policy: DnsPolicy | null;
  policies: DnsPolicy[];
  onClose: () => void;
  onSave: (policy: DnsPolicy) => void;
  onDelete: () => void;
}) {
  const [name, setName] = useState(policy?.name || "");
  const [domains, setDomains] = useState<string[]>(policy?.domains || []);
  const [domainPending, setDomainPending] = useState(false);
  const [servers, setServers] = useState<string[]>(policy?.servers || []);
  const [serverPending, setServerPending] = useState(false);
  const [error, setError] = useState("");
  const [deleting, setDeleting] = useState(false);
  return (
    <Modal
      wide
      title={policy ? "Изменить соответствие DNS" : "Новое соответствие DNS"}
      subtitle="Несколько доменов могут использовать один набор DNS-серверов."
      onClose={onClose}
    >
      <form
        className="modal-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (domainPending || serverPending) {
            setError(
              "В поле остался адрес: добавьте его в список или очистите поле перед сохранением.",
            );
            return;
          }
          const patterns = domains;
          const addresses = servers;
          if (
            !patterns.length ||
            patterns.some((d) => !validateDnsPattern(d))
          ) {
            setError("Добавьте хотя бы один домен в список.");
            return;
          }
          if (
            !addresses.length ||
            addresses.some((s) => !validateDnsServer(s))
          ) {
            setError("Добавьте хотя бы один DNS-сервер в список.");
            return;
          }
          const duplicate = patterns.find((d) =>
            policies.some(
              (p) =>
                p.id !== policy?.id &&
                p.domains.some((value) => value.toLowerCase() === d),
            ),
          );
          if (duplicate) {
            setError(
              "Для " +
                duplicate +
                " уже есть соответствие. Измените существующую запись.",
            );
            return;
          }
          onSave({
            id: policy?.id || crypto.randomUUID(),
            name: name.trim(),
            domains: [...new Set(patterns)],
            servers: [...new Set(addresses)],
            enabled: policy?.enabled ?? true,
          });
        }}
      >
        <div className="modal-body">
          <label className="form-label">
            Название<span className="optional">необязательно</span>
            <Input variant="secondary"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Например, внутренние сервисы"
            />
          </label>
          <DnsEntries
            kind="domain"
            values={domains}
            onChange={(values) => {
              setDomains(values);
              setError("");
            }}
            onPendingChange={setDomainPending}
          />
          <div className="dns-direction">
            <ArrowRight size={16} />
            <span>Разрешать через</span>
          </div>
          <DnsEntries
            kind="server"
            values={servers}
            onChange={(values) => {
              setServers(values);
              setError("");
            }}
            onPendingChange={setServerPending}
          />
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          {policy && (
            <div className="dns-delete">
              {deleting ? (
                <>
                  <p>
                    Удалить это соответствие? Домены будут использовать другие
                    подходящие записи или DNS по умолчанию.
                  </p>
                  <Button
                    type="button"
                    className="button"
                    onClick={() => setDeleting(false)}
                  >
                    Оставить
                  </Button>
                  <Button
                    type="button"
                    className="button danger"
                    onClick={onDelete}
                  >
                    Удалить соответствие
                  </Button>
                </>
              ) : (
                <Button
                  type="button"
                  className="text-button danger"
                  onClick={() => setDeleting(true)}
                >
                  <Trash2 size={14} />
                  Удалить соответствие
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
            Сохранить соответствие
          </Button>
        </footer>
      </form>
    </Modal>
  );
}
