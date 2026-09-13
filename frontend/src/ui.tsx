import { Input } from "@heroui/react";
import { Button } from "./controls";
import { type ReactNode } from "react";
import { Modal as HeroModal, Switch, Label } from "@heroui/react";

import {
  ArrowUpRight,
  Check,
  Globe2,
  Laptop,
  Search,
  Send,
  Terminal,
  X,
  Ban,
} from "lucide-react";
import { routeNames, type Route } from "./model";
export function Logo() {
  return (
    <svg viewBox="0 0 36 36" fill="none" aria-hidden="true">
      <path
        d="M7 24V14a5 5 0 0 1 5-5h3v13a5 5 0 0 1-5 5H7v-3Z"
        fill="currentColor"
      />
      <path
        d="M21 9h3a5 5 0 0 1 5 5v13h-3a5 5 0 0 1-5-5V9Z"
        fill="currentColor"
        opacity=".55"
      />
    </svg>
  );
}
export function Flag({ code }: { code: string }) {
  return (
    <span className={"flag flag-" + code} aria-hidden="true">
      {!["nl", "de", "fi", "se"].includes(code) && <Globe2 size={17} />}
    </span>
  );
}
export function AppIcon({ type }: { type: string }) {
  return (
    <span className={"app-icon app-" + type} aria-hidden="true">
      {type === "chrome" ? (
        <span className="chrome-center" />
      ) : type === "telegram" ? (
        <Send size={17} fill="currentColor" />
      ) : type === "code" ? (
        <span className="code-symbol">〈</span>
      ) : type === "slack" ? (
        <span>✣</span>
      ) : type === "steam" ? (
        <svg
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        >
          <circle cx="16" cy="7" r="4" />
          <circle cx="7" cy="17" r="3" />
          <path d="m9 15 4-5m-10 5 5 2 7-7" />
        </svg>
      ) : (
        <Terminal size={17} />
      )}
    </span>
  );
}
export function RouteBadge({ route }: { route: Route }) {
  const Icon =
    route === "vpn" ? ArrowUpRight : route === "block" ? Ban : Globe2;
  return (
    <span className={"route-badge " + route}>
      <Icon size={12} />
      {routeNames[route]}
    </span>
  );
}
export function Toggle({
  checked,
  onChange,
  label,
  disabled = false,
}: {
  checked: boolean;
  onChange: () => void;
  label: string;
  disabled?: boolean;
}) {
  return <Switch aria-label={label} isSelected={checked} onChange={onChange} isDisabled={disabled}><Switch.Content><Switch.Control><Switch.Thumb /></Switch.Control><Label className="sr-only">{label}</Label></Switch.Content></Switch>;
}
export function SearchBox({
  value,
  onChange,
  placeholder = "Поиск",
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <div className="search">
      <Search size={15} />
      <Input variant="secondary"
        aria-label={placeholder}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
      />
      {value && (
        <Button
          type="button"
          aria-label="Очистить поиск"
          onClick={() => onChange("")}
        >
          <X size={13} />
        </Button>
      )}
    </div>
  );
}
export function Empty({
  title,
  children,
}: {
  title: string;
  children: ReactNode;
}) {
  return (
    <div className="empty">
      <Search size={27} />
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
export function Modal({
  title,
  subtitle,
  children,
  onClose,
  wide = false,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
  onClose: () => void;
  wide?: boolean;
}) {
  return <HeroModal.Backdrop isOpen onOpenChange={open => { if (!open) onClose(); }}>
    <HeroModal.Container size={wide ? 'lg' : 'md'} placement="center" scroll="inside">
      <HeroModal.Dialog className={"app-dialog" + (wide ? " app-dialog-wide" : "")}>
        <HeroModal.CloseTrigger aria-label="Закрыть окно" />
        <HeroModal.Header><HeroModal.Heading>{title}</HeroModal.Heading>
          {subtitle && <p className="text-muted text-sm">{subtitle}</p>}
        </HeroModal.Header>
        {children}
      </HeroModal.Dialog>
    </HeroModal.Container>
  </HeroModal.Backdrop>;
}

export function TrafficChart({
  active,
  period,
}: {
  active: boolean;
  period: string;
}) {
  const raw = [
    15, 14, 17, 16, 20, 19, 18, 27, 24, 21, 20, 23, 22, 31, 28, 24, 27, 26, 25,
    29, 43, 55, 52, 42, 46, 57, 54, 48, 39, 34, 32, 36, 34, 33, 40, 38, 44, 41,
    46, 61, 69, 61, 57, 61, 53, 48, 44, 42, 46, 42, 40, 43, 38, 34, 35, 33, 38,
    41, 35, 38, 34, 31, 35, 37, 38, 35, 40, 39, 42, 37, 39, 43,
  ];
  const values = raw.map((v, i) =>
    active ? (period === "5 мин" ? v * 0.8 + Math.sin(i) * 8 : v) : 0,
  );
  const points = values
    .map((v, i) => `${(i * 840) / (values.length - 1)},${112 - v * 1.3}`)
    .join(" ");
  const up = values
    .map((v, i) => `${(i * 840) / (values.length - 1)},${115 - v * 0.22}`)
    .join(" ");
  return (
    <div
      className="chart"
      role="img"
      aria-label={
        active
          ? "Демонстрационный график: скачивание 12,4 МБ/с, отправка 1,2 МБ/с"
          : "VPN отключён, трафик отсутствует"
      }
    >
      <div className="chart-scale">
        <span>20 МБ/с</span>
        <span>10 МБ/с</span>
        <span>0</span>
      </div>
      <svg viewBox="0 0 840 128" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id="traffic-fill" x1="0" y1="0" x2="0" y2="1">
            <stop stopColor="var(--accent)" stopOpacity=".13" />
            <stop offset="1" stopColor="var(--accent)" stopOpacity=".015" />
          </linearGradient>
        </defs>
        {[12, 62, 112].map((y) => (
          <line key={y} x1="0" y1={y} x2="840" y2={y} className="grid-line" />
        ))}
        <polygon points={`0,125 ${points} 840,125`} fill="url(#traffic-fill)" />
        <polyline
          points={points}
          fill="none"
          stroke="var(--accent)"
          strokeWidth="2"
          vectorEffect="non-scaling-stroke"
        />
        <polyline
          points={up}
          fill="none"
          stroke="var(--teal)"
          strokeWidth="1.6"
          vectorEffect="non-scaling-stroke"
        />
      </svg>
      <div className="chart-time">
        <span>{period === "5 мин" ? "5 минут" : "60 секунд"} назад</span>
        <span>Сейчас</span>
      </div>
    </div>
  );
}
export function Checkmark() {
  return (
    <span className="checkmark">
      <Check size={12} />
    </span>
  );
}
export function DeviceIcon() {
  return (
    <span className="device-icon">
      <Laptop size={22} />
    </span>
  );
}
