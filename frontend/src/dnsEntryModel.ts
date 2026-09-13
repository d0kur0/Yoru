export type DomainScope = "all" | "exact" | "level";
export const domainScopes = {
  all: "Домен и поддомены",
  exact: "Только этот домен",
  level: "Поддомены одного уровня",
};
export function domainInfo(pattern: string) {
  const scope: DomainScope = pattern.startsWith("+.")
    ? "all"
    : pattern.startsWith("*.")
      ? "level"
      : "exact";
  return { scope, name: pattern.replace(/^[+*]\./, "") };
}
export function domainPattern(raw: string, scope: DomainScope) {
  const value = raw.trim().toLowerCase();
  return /^[+*]\./.test(value)
    ? value
    : (scope === "all" ? "+." : scope === "level" ? "*." : "") + value;
}
