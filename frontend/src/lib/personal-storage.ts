// Personenbezogene Einträge im Browser-Speicher (#5000).
//
// Auf einem geteilten Schulgerät darf die nächste Person nichts von der
// vorigen sehen: keine Namen von Gruppen, Räumen oder Kindern, keine
// Vorauswahl, keine Zähler. Dieses Modul ist das Inventar dieser Einträge und
// räumt sie ab, wenn das Konto geht (Logout) oder wechselt (anderes Konto,
// andere Schule, Vorschau als andere Person).
//
// Wer einen solchen Eintrag in localStorage schreibt, kennt das Konto, für
// das er schreibt, und fragt direkt davor mayWritePersonalStorage(): Nach
// einem Logout oder sobald ein anderer Tab die Einträge an ein anderes Konto
// gebunden hat, schreibt er nicht mehr. Sonst brächte eine späte Antwort oder
// ein noch nicht aktualisierter Tab den Eintrag des vorigen Kontos zurück.
//
// Jeder Schlüssel, den das Frontend in localStorage oder sessionStorage
// schreibt, steht entweder in PERSONAL_STORAGE_PREFIXES (wird geräumt) oder
// in KEPT_STORAGE_KEYS (bleibt, mit Grund). personal-storage.test.ts sucht
// die Schlüssel im Quelltext und schlägt fehl, wenn einer in keiner der
// beiden Listen steht.
//
// Bibliotheken: next-auth gleicht Tabs über BroadcastChannel ab, SWR hält
// seinen Cache im RAM, Sentry zeichnet keine Replays auf. PostHog läuft mit
// `persistence: "memory"`; das SDK schreibt nur die Einwilligung des Geräts
// (`__ph_opt_in_out_<projekt>`) und Fenster-IDs in sessionStorage
// (`ph_<projekt>_window_id`, `ph_<projekt>_primary_window_exists`), ohne
// Konto. Beim Kontowechsel vergibt `posthog.reset()` neue IDs.

import { clearAnnouncementStudents } from "./announcement-prefill";

const OWNER_KEY = "moto:personal-storage-owner";

/**
 * Präfixe aller personenbezogenen Schlüssel, in `localStorage` und
 * `sessionStorage`. Ein Präfix deckt auch die Altform ohne Schule und Konto
 * ab, damit sie beim ersten Räumen nach dem Update verschwindet.
 */
export const PERSONAL_STORAGE_PREFIXES: readonly string[] = [
  // Seitenleiste: zuletzt gewählte Gruppe und Raum samt Namen, laufende
  // Aufsicht, zuletzt offene Seite der Datenverwaltung (sidebar-memory.ts).
  "sidebar-last-",
  "supervision-last-session",
  // Zähler je Konto (useUnreadCount-Caches).
  "messages_unread_count:",
  "team_chat_unread_count:",
  "enrollments_unread_count:",
  "enrollment_requests_pending_count:",
  "change_requests_pending_count:",
  "care_withdrawals_pending_count:",
  "staff_absences_pending_count:",
  "staff_notices_pending:",
  "school_team_chat_unread:",
  "school_staff_notices_pending:",
  // Gemerkte Filter der Kindersuche: Gruppen, Räume samt Namen, Klassen.
  "student-search:last-filters:",
  // Zuletzt gewähltes Angebot im Dialog „Anwesende Kinder“.
  "moto:present-children-offering:",
  // Vorbelegung des Verfassers mit Kindern und Namen; liegt seit #5000 nur
  // noch im RAM, der Präfix räumt Einträge aus der Zeit davor.
  "moto:announcement-prefill:",
  // „Rückgängig“ nach dem Beenden einer Aktivität: Titel und Raum.
  "timetable-reopenable-instance",
  // „Einmal pro Anmeldung“ geöffnete Assistenten, je Konto.
  "school-setup-opened:",
  "staff-onboarding-opened:",
  "parent-first-steps-opened:",
  // „App-Nutzung dieser Sitzung schon gemeldet“, je Portal, Konto und Schule.
  // Nach dem Räumen meldet die nächste Anmeldung einmal mehr; das Backend
  // schreibt dann nur last_seen_at derselben Zeile neu (#2189).
  "moto-pwa-usage-reported.",
  // Wem die Einträge gerade gehören (bindPersonalStorage).
  OWNER_KEY,
];

/**
 * Schlüssel, die Logout und Kontowechsel überleben, jeweils mit Grund. `*`
 * steht für einen eingesetzten Wert. Ein Eintrag, der doch geräumt werden
 * soll, zieht als Präfix nach PERSONAL_STORAGE_PREFIXES um.
 */
export const KEPT_STORAGE_KEYS: readonly string[] = [
  // Einstellungen je Konto ohne Daten anderer Personen; der Schlüssel trägt
  // Schule und Konto, also übernimmt kein anderes Konto sie. Gelöscht fragte
  // moto sie nach jeder Anmeldung neu ab.
  "collection-view:*", // Kacheln oder Tabelle, Spalten (#3834)
  "moto.*.notification-setup.v1*", // Geräte-Einrichtung erledigt oder später
  "parent-first-steps:v1:*", // Fortschritt der ersten Schritte, Elternportal
  // Gewählte Sprache, die das Elternportal noch speichern muss.
  "phoenix_parent_unsynced_locale*",
  "phoenix_parent_locale_save_failure",
  // Gerätelayout.
  "sidebar-collapsed",
  "sidebar-open-groups",
  "sidebar-accordion-expanded",
  // Steuerwerte ohne Konto. deliberateLogout muss den Logout überleben: die
  // Login-Seite liest daraus, dass er gewollt war.
  "deliberateLogout",
  "lastSuccessfulRefresh",
  "tagesplan-scroll",
  // Sperrzeit für die nächste Anfrage „Passwort vergessen“; entsteht vor der
  // Anmeldung und trägt nur einen Zeitpunkt, weder Konto noch Adresse.
  "passwordResetRateLimitUntil",
  "parentPasswordResetRateLimitUntil",
  // Hinweise auf diesem Gerät und die öffentliche Demo.
  "schulhof-banner-dismissed",
  "moto-pwa-install-hint-*",
  "moto-demo-*",
];

export function isPersonalStorageKey(key: string): boolean {
  return PERSONAL_STORAGE_PREFIXES.some((prefix) => key.startsWith(prefix));
}

/**
 * Besitzer der Einträge: Schule (oder Träger) und Konto der Sitzung. Null,
 * solange keine Person angemeldet ist.
 */
export function personalStorageOwner(
  user:
    | {
        readonly id?: string | null;
        readonly tenantId?: number | null;
        readonly orgId?: number | null;
      }
    | null
    | undefined,
): string | null {
  if (!user?.id) return null;
  return `${user.tenantId ?? user.orgId ?? "-"}:${user.id}`;
}

function clearStorage(storage: Storage | undefined): void {
  if (!storage) return;
  try {
    const keys: string[] = [];
    for (let index = 0; index < storage.length; index += 1) {
      const key = storage.key(index);
      if (key !== null && isPersonalStorageKey(key)) keys.push(key);
    }
    for (const key of keys) storage.removeItem(key);
  } catch {
    // Gesperrter Speicher (privates Fenster, Richtlinie) hält auch nichts fest.
  }
}

function webStorage(name: "localStorage" | "sessionStorage") {
  try {
    return globalThis[name];
  } catch {
    // Der Zugriff selbst kann werfen, wenn Websitedaten gesperrt sind.
    return undefined;
  }
}

function boundOwner(): string | null {
  try {
    return webStorage("localStorage")?.getItem(OWNER_KEY) ?? null;
  } catch {
    // Unlesbarer Speicher: es gibt keinen gebundenen Besitzer.
    return null;
  }
}

// Diese Seite gehört einem vorigen Konto und wird gerade ersetzt.
let leaving = false;

/**
 * Darf `owner` (Schule und Konto der Sitzung, die schreibt) jetzt einen
 * personenbezogenen Eintrag schreiben? Nur wenn die Einträge in diesem
 * Browser genau diesem Besitzer gehören. Der Besitzer steht in localStorage
 * und gilt für alle Tabs dieses Hosts; nach einem Logout steht dort keiner.
 * Eine Seite, die gerade für ein anderes Konto neu lädt, schreibt nichts mehr.
 */
export function mayWritePersonalStorage(owner: string | null): boolean {
  return !leaving && owner !== null && boundOwner() === owner;
}

/** Das Konto eines Besitzers, ohne die Schule. */
export function personalStorageAccount(owner: string | null): string | null {
  if (!owner) return null;
  const separator = owner.indexOf(":");
  return separator === -1 ? owner : owner.slice(separator + 1);
}

/**
 * Meldet, wenn ein anderer Tab dieses Hosts die Einträge an einen neuen
 * Besitzer bindet oder sie räumt (`null`). Der eigene Tab löst kein
 * storage-Ereignis aus.
 */
export function onOtherTabOwnerChange(
  listener: (owner: string | null) => void,
): () => void {
  const handleStorage = (event: StorageEvent) => {
    if (event.key !== OWNER_KEY) return;
    if (event.storageArea && event.storageArea !== webStorage("localStorage")) {
      return;
    }
    listener(event.newValue);
  };
  globalThis.addEventListener("storage", handleStorage);
  return () => globalThis.removeEventListener("storage", handleStorage);
}

/**
 * Ersetzt diese Seite durch `href`, weil sie noch Zustand und Adresse eines
 * anderen Kontos trägt (#5000). Ab hier schreibt sie keinen
 * personenbezogenen Eintrag mehr; die Seite, die danach lädt, beginnt leer.
 */
export function leaveForAnotherAccount(href: string): void {
  if (leaving) return;
  leaving = true;
  globalThis.location.replace(href);
}

/** Nur für Tests: in Tests ersetzt keine neue Seite diese. */
export function _resetPersonalStorageLeave(): void {
  leaving = false;
}

/** Entfernt alle personenbezogenen Einträge dieses Browsers für diesen Host. */
export function clearPersonalStorage(): void {
  clearStorage(webStorage("localStorage"));
  clearStorage(webStorage("sessionStorage"));
  clearAnnouncementStudents();
}

/**
 * Bindet die Einträge an den Besitzer der Sitzung. Gehören sie jemand
 * anderem (anderes Konto, andere Schule, Vorschau) oder niemandem
 * Bekanntem, werden sie vorher geräumt. Beide Speicher führen ihren eigenen
 * Besitzer: Ein anderer Tab kann localStorage bereits neu gebunden haben,
 * während dieser Tab noch sessionStorage des vorigen Kontos enthält.
 */
export function bindPersonalStorage(owner: string): void {
  for (const name of ["localStorage", "sessionStorage"] as const) {
    const storage = webStorage(name);
    if (!storage) continue;
    try {
      if (storage.getItem(OWNER_KEY) === owner) continue;
      clearStorage(storage);
      clearAnnouncementStudents();
      storage.setItem(OWNER_KEY, owner);
    } catch {
      // Gesperrter Speicher hindert nicht das Binden des anderen Speichers.
    }
  }
}
