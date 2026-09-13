import { useEffect, useState } from "react";
export type ThemeMode = "dark" | "light" | "system";
export const isDesktop = () => Boolean((window as any)._wails || (window as any).chrome?.webview || (window as any).webkit?.messageHandlers?.external);
const valid = (mode: unknown): mode is ThemeMode => ["dark", "light", "system"].includes(String(mode));
const preferred = (): ThemeMode => { try { const mode=localStorage.getItem("tiho.appearance"); return valid(mode) ? mode : "dark"; } catch { return "dark"; } };
export function paint(mode: ThemeMode, resolved?: string) {
  const dark = resolved ? resolved === "dark" : mode === "dark" || (mode === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  document.documentElement.classList.toggle("dark", dark);
  document.documentElement.style.colorScheme = dark ? "dark" : "light";
}
export function useAppearance() {
  const [mode, setMode] = useState<ThemeMode>(preferred);
  const [error, setError] = useState("");
  const [platform, setPlatform] = useState(/Mac/i.test(navigator.platform) ? "darwin" : "windows");
  useEffect(() => {
    const receive = (state: {mode: string; resolved: string; platform: string}) => {
      if (!valid(state.mode)) return;
      setMode(state.mode); setPlatform(state.platform); paint(state.mode, state.resolved);
      try { localStorage.setItem("tiho.appearance", state.mode); } catch {}
    };
    const event = (event: Event) => receive((event as CustomEvent).detail);
    const media = matchMedia("(prefers-color-scheme: dark)");
    const changed = () => { if (!isDesktop()) paint(preferred()); };
    window.addEventListener("tiho-appearance", event);
    media.addEventListener("change", changed);
    if (isDesktop()) import("../bindings/github.com/d0kur0/Yoru/appearance").then(api => api.Get()).then(receive).catch(() => setError("Не удалось прочитать тему приложения"));
    else paint(preferred());
    return () => { window.removeEventListener("tiho-appearance", event); media.removeEventListener("change", changed); };
  }, []);
  async function change(next: string) {
    if (!valid(next)) return;
    try {
      if (isDesktop()) { const api = await import("../bindings/github.com/d0kur0/Yoru/appearance"); const state = await api.Set(next); paint(next,state.resolved); }
      else paint(next);
      localStorage.setItem("tiho.appearance",next); setMode(next); setError("");
    } catch { setError("Не удалось сохранить тему"); }
  }
  return {mode, change, error, platform};
}
