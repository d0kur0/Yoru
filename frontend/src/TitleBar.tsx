import { Minus, Square, X } from "lucide-react";
import { isDesktop } from "./appearance";
type Page = "overview" | "servers" | "rules" | "dns" | "settings";
const pages: {id: Page; label: string}[] = [
  {id: "overview", label: "Подключение"}, {id: "servers", label: "Серверы"},
  {id: "rules", label: "Правила"}, {id: "dns", label: "DNS"}, {id: "settings", label: "Настройки"},
];
type WindowAction = "Minimise" | "ToggleMaximise" | "Close";
export function TitleBar({platform, page, onNavigate}: {platform: string; page: Page; onNavigate: (page: Page) => void}) {
  const native = isDesktop();
  async function action(name: WindowAction) {
    if (!native) return;
    const {Window} = await import("@wailsio/runtime");
    await Window[name]();
  }
  const actions: WindowAction[] = platform === "darwin" ? ["Close", "Minimise", "ToggleMaximise"] : ["Minimise", "ToggleMaximise", "Close"];
  const controls = <div className={"window-controls " + (platform === "darwin" ? "mac-controls" : "win-controls")}>
    {actions.map(name => <button type="button" className={"window-control control-" + name} key={name} disabled={!native}
      aria-label={name === "Close" ? "Скрыть окно в трей" : name === "Minimise" ? "Свернуть" : "Развернуть / восстановить"}
      onClick={() => void action(name)}>
      {name === "Close" ? <X size={14}/> : name === "Minimise" ? <Minus size={14}/> : <Square size={11}/>}
    </button>)}
  </div>;
  return <header className={"titlebar " + (platform === "darwin" ? "titlebar-mac" : "titlebar-win")}>
    {platform === "darwin" && controls}
    <nav className="top-navigation" aria-label="Основная навигация">
      {pages.map(item => <button key={item.id} type="button" aria-current={item.id === page ? "page" : undefined}
        onClick={() => onNavigate(item.id)}>{item.label}</button>)}
    </nav>
    <div className="titlebar-spacer"/>
    {platform !== "darwin" && controls}
  </header>;
}
