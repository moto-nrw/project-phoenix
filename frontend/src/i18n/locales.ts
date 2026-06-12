import { localeDefinitions } from "./locales.generated";

export type AppLocale = (typeof localeDefinitions)[number]["code"];

type LocaleDefinition = {
  readonly code: AppLocale;
  readonly label: string;
  readonly fallback?: boolean;
};

const supportedLocales: readonly LocaleDefinition[] = localeDefinitions;

export const LOCALE_COOKIE_NAME = "phoenix_parent_locale";

// Request header the proxy sets on parent-facing surfaces (parents subdomain +
// public enrollment). request.ts only resolves a non-default locale when this
// is present, so the German-only staff/operator portals never inherit a
// parent's locale or a browser's Accept-Language.
export const LOCALE_SCOPE_HEADER = "x-phoenix-localize";

export const SUPPORTED_LOCALES = supportedLocales;

export const DEFAULT_LOCALE = (supportedLocales.find(
  (locale) => locale.fallback,
)?.code ?? supportedLocales[0]?.code) as AppLocale;

const localeCodes = new Set<string>(
  supportedLocales.map((locale) => locale.code),
);

export function normalizeLocale(raw: string | null | undefined): AppLocale {
  const trimmed = raw?.trim().toLowerCase();
  if (!trimmed) return DEFAULT_LOCALE;
  const code = trimmed.split(/[-_]/)[0] ?? "";
  return (localeCodes.has(code) ? code : DEFAULT_LOCALE) as AppLocale;
}

export function writeLocaleCookie(locale: AppLocale): void {
  if (typeof document === "undefined") return;
  // Add Secure on HTTPS so the cookie never rides an unencrypted request in
  // production; omit it on http://localhost so dev (and the host-agnostic guide
  // render) still set the cookie. It carries only a UI language, no secret.
  const secure = window.location.protocol === "https:" ? "; secure" : "";
  document.cookie = `${LOCALE_COOKIE_NAME}=${encodeURIComponent(
    locale,
  )}; path=/; max-age=31536000; samesite=lax${secure}`;
}
