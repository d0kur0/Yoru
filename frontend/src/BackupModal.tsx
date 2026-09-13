import { Input } from "@heroui/react";
import { Button } from "./controls";
import { useRef, useState } from "react";
import { FileJson, Upload } from "lucide-react";
import { Modal } from "./ui";
import { parseBackup, type Backup, type Configuration } from "./backup";

export function BackupModal({
  onClose,
  onRestore,
}: {
  onClose: () => void;
  onRestore: (configuration: Configuration) => void;
}) {
  const [backup, setBackup] = useState<Backup | null>(null);
  const [filename, setFilename] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const request = useRef(0);
  const fileInput = useRef<HTMLInputElement>(null);
  async function readFile(file?: File) {
    const id = ++request.current;
    setBackup(null);
    setError("");
    setFilename(file?.name || "");
    if (!file) {
      setLoading(false);
      return;
    }
    setLoading(true);
    try {
      if (file.size > 2 * 1024 * 1024)
        throw new Error("Файл слишком большой. Максимальный размер: 2 МБ.");
      const parsed = parseBackup(await file.text());
      if (id === request.current) setBackup(parsed);
    } catch (e) {
      if (id === request.current)
        setError(e instanceof Error ? e.message : "Не удалось прочитать файл.");
    } finally {
      if (id === request.current) setLoading(false);
    }
  }
  return (
    <Modal
      title="Восстановить из копии"
      subtitle="Перенесите конфигурацию с другого компьютера."
      onClose={onClose}
    >
      <form
        className="modal-form"
        onSubmit={(e) => {
          e.preventDefault();
          if (!backup || loading) return;
          try {
            onRestore(backup.configuration);
          } catch {
            setError(
              "Не удалось сохранить копию на устройстве. Текущие настройки не изменены.",
            );
          }
        }}
      >
        <div className="modal-body">
          <div className="backup-file">
            <FileJson size={26} />
            <strong>Файл резервной копии</strong>
            <span>JSON, до 2 МБ</span>
            <Input variant="secondary"
              ref={fileInput}
              hidden
              type="file"
              accept=".json,application/json"
              onChange={(e) => void readFile(e.target.files?.[0])}
            />
            <Button
              type="button"
              className="button"
              onClick={() => fileInput.current?.click()}
            >
              {filename ? "Выбрать другой файл" : "Выбрать файл"}
            </Button>
          </div>
          {loading && (
            <p className="helper" role="status">
              Проверяем файл…
            </p>
          )}
          {error && (
            <p className="form-error" role="alert">
              {error}
            </p>
          )}
          {backup && (
            <section
              className="backup-preview"
              aria-label="Содержимое резервной копии"
            >
              <h3>{filename}</h3>
              <p>
                Создана {new Date(backup.createdAt).toLocaleString("ru-RU")}
              </p>
              <dl>
                <div>
                  <dt>Серверы</dt>
                  <dd>{backup.configuration.servers.length}</dd>
                </div>
                <div>
                  <dt>Правила</dt>
                  <dd>{backup.configuration.rules.length}</dd>
                </div>
                <div>
                  <dt>Выбранный сервер</dt>
                  <dd>
                    {backup.configuration.servers.find(
                      (s) => s.id === backup.configuration.selected,
                    )?.name || "Не выбран"}
                  </dd>
                </div>
                <div>
                  <dt>Соответствия DNS</dt>
                  <dd>{backup.configuration.settings.dnsPolicies.length}</dd>
                </div>
                <div>
                  <dt>Настройки приложения и DNS</dt>
                  <dd>Включены</dd>
                </div>
              </dl>
            </section>
          )}
          <p className="helper backup-note">
            Копия дизайн-версии. Ключи подключения пока не хранятся в приложении
            и в файл не входят.
          </p>
        </div>
        {backup && (
          <p className="backup-confirm">
            Текущие серверы, правила и настройки будут полностью заменены
            данными из файла.
          </p>
        )}
        <footer className="modal-footer">
          <Button type="button" className="button" onClick={onClose}>
            Отмена
          </Button>
          <Button className="button primary" disabled={!backup || loading}>
            <Upload size={15} />
            Восстановить настройки
          </Button>
        </footer>
      </form>
    </Modal>
  );
}
