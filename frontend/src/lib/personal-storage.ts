// Personenbezogene Einträge im Browser-Speicher (#5000).
//
// Auf einem geteilten Schulgerät darf die nächste Person nichts von der
// vorigen sehen: keine Namen von Gruppen, Räumen oder Kindern, keine
// Vorauswahl, keine Zähler. Dieses Modul ist das Inventar dieser Einträge und
// räumt sie ab, wenn das Konto geht (Logout) oder wechselt (anderes Konto,
// andere Schule, Vorschau als andere Person). Ein Schlüssel, der Daten einer
// Person trägt, gehört in PERSONAL_STORAGE_PREFIXES; sonst überlebt er den
// Logout.
//
// Bewusst nicht im Inventar, weil sie nur Einstellungen ohne fremde Daten
// tragen und ihr Schlüssel schon an Schule und Konto hängt: die Ansicht
// einer Sammlung (`collection-view:`, #3834), die erledigte
// Geräte-Einrichtung (`moto.<portal>.notification-setup.`), der Fortschritt
// der ersten Schritte im Elternportal (`parent-first-steps:v1:`) und die noch
// nicht gespeicherte Sprache (`phoenix_parent_unsynced_locale`). Gelöscht
// würden sie bei jeder Anmeldung neu abgefragt. Ebenso bleiben Geräte-Werte
// ohne Konto: Seitenleiste ein- oder ausgeklappt, Hinweise zur
// App-Installation, die Scrollposition des Tagesplans und die Markierung
// `deliberateLogout`, die der Login-Seite nach dem Logout gerade sagen soll,
// dass er gewollt war.

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
  // Wem die Einträge gerade gehören (bindPersonalStorage).
  OWNER_KEY,
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

/** Entfernt alle personenbezogenen Einträge dieses Browsers für diesen Host. */
export function clearPersonalStorage(): void {
  clearStorage(webStorage("localStorage"));
  clearStorage(webStorage("sessionStorage"));
  clearAnnouncementStudents();
}

/**
 * Bindet die Einträge an den Besitzer der Sitzung. Gehören sie jemand
 * anderem (anderes Konto, andere Schule, Vorschau) oder niemandem
 * Bekanntem, werden sie vorher geräumt.
 */
export function bindPersonalStorage(owner: string): void {
  const storage = webStorage("localStorage");
  if (!storage) return;
  try {
    if (storage.getItem(OWNER_KEY) === owner) return;
  } catch {
    // Unlesbarer Speicher hält auch nichts von jemand anderem.
    return;
  }
  clearPersonalStorage();
  try {
    storage.setItem(OWNER_KEY, owner);
  } catch {
    // Ohne Speicher gibt es nichts zu binden.
  }
}
