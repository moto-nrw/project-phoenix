// Additional modules a school books (#4892). The backend module list
// (backend/modules/settings/records/modules.go) owns names, prerequisites and
// the switches; this file mirrors only the stable keys. moto Basis is always
// included and has no key. The tenant shell sends the booked keys as
// `booked_modules`; each switch is the setting `module.<key>`.
export const MODULE_KEYS = [
  "nfc_anwesenheit",
  "mittagessen",
  "eltern_app",
  "online_anmeldung",
  "unterricht_vormittag",
  "dienstplan",
  "zeiterfassung",
  "personalakte",
  "traeger_uebersicht",
  "ferienbetreuung",
  "vertraege",
  "elternbeitraege",
] as const;

export type ModuleKey = (typeof MODULE_KEYS)[number];

const MODULE_KEY_SET: ReadonlySet<string> = new Set(MODULE_KEYS);

export function isModuleKey(value: unknown): value is ModuleKey {
  return typeof value === "string" && MODULE_KEY_SET.has(value);
}

/** The setting key of a module switch. */
export function moduleSettingKey(module: ModuleKey): `module.${ModuleKey}` {
  return `module.${module}`;
}

/** Every module switch, for caches that follow a booking change. */
export const MODULE_SETTING_KEYS: readonly string[] =
  MODULE_KEYS.map(moduleSettingKey);

/**
 * Parses `booked_modules` from the tenant shell. A missing or malformed list
 * books nothing: a module must never appear because a response was
 * incomplete. Keys this frontend does not know yet are dropped.
 */
export function parseBookedModules(raw: unknown): readonly ModuleKey[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  return raw.filter(isModuleKey);
}
