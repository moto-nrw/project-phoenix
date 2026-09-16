// Localization helpers for permission resources/actions
//
// Every permission moto ships is listed in
// backend/auth/authorize/permissions/catalog.json, and the test beside this
// file checks each catalog entry against the three tables below. A missing
// entry used to fall back silently to the raw key ("manage_categories") or to
// the English database description (#3238), so the fallbacks stay as a safety
// net for permissions a school adds at runtime — not as a licence to skip the
// German wording for a shipped one.

export const resourceLabels: Record<string, string> = {
  users: "Benutzer",
  roles: "Rollen",
  permissions: "Berechtigungen",
  activities: "Aktivitäten",
  rooms: "Räume",
  groups: "Gruppen",
  visits: "Besuche",
  substitutions: "Vertretungen",
  schedules: "Zeitpläne",
  config: "Konfiguration",
  feedback: "Feedback",
  iot: "Geräte",
  system: "System",
  admin: "Administration",
  time_tracking: "Zeiterfassung",
  grade_transitions: "Klassenwechsel",
  calendar: "Kalender",
  staff: "Mitarbeitende",
  files: "Dateiablage",
  auth: "Anmeldung",
  guardians: "Eltern",
  staff_documents: "Personalunterlagen",
  student_documents: "Unterlagen der Kinder",
  staff_notices: "Tagesinformationen",
  class_day: "Klassenansicht",
  supervision: "Aufsichten",
  display: "Info-Point",
  vacation: "Urlaub",
  "*": "Alle Bereiche",
};

export const actionLabels: Record<string, string> = {
  create: "Erstellen",
  read: "Lesen",
  update: "Bearbeiten",
  delete: "Löschen",
  list: "Auflisten",
  manage: "Verwalten",
  assign: "Zuweisen",
  enroll: "Einschreiben",
  own: "Eigene",
  apply: "Anwenden",
  financial: "Bank- & Steuerdaten",
  stammdaten: "Personalstammdaten",
  documents: "Personalunterlagen",
  manage_categories: "Kategorien verwalten",
  checkin: "An- und abmelden",
  absence: "Abwesenheiten",
  approve: "Genehmigen",
  health: "Gesundheitsdaten",
  legal: "Sorgerecht",
  arrival_exception_write: "Ankunftszeit ändern",
  "*": "Alle",
};

/**
 * Action labels that one resource needs to read differently. The action table
 * is shared, so "financial" cannot say "Bank- & Steuerdaten" for staff and
 * "Bankdaten" for guardians at the same time — and a school holds no tax data
 * of a parent.
 */
const actionLabelsByPermission: Record<string, string> = {
  "guardians:financial": "Bankdaten",
};

export function localizeResource(resource: string): string {
  return resourceLabels[resource] ?? resource;
}

export function localizeAction(action: string, resource?: string): string {
  if (resource !== undefined) {
    const specific = actionLabelsByPermission[`${resource}:${action}`];
    if (specific !== undefined) return specific;
  }
  return actionLabels[action] ?? action;
}

export function formatPermissionDisplay(
  resource: string,
  action: string,
): string {
  return `${localizeResource(resource)}: ${localizeAction(action, resource)}`;
}

/**
 * German descriptions for permissions, keyed by "resource:action".
 * Used to override English descriptions stored in the database.
 */
const permissionDescriptions: Record<string, string> = {
  // Users
  "users:create": "Neue Benutzer erstellen",
  "users:read": "Benutzerinformationen ansehen",
  "users:update": "Benutzerinformationen bearbeiten",
  "users:delete": "Benutzer löschen",
  "users:list": "Benutzer auflisten",
  "users:manage": "Benutzerverwaltung (Vollzugriff)",
  "users:checkin": "Kinder in moto anmelden und abmelden",
  "users:absence":
    "Krankmeldungen und Abwesenheiten von Kindern eintragen und über Anfragen der Eltern entscheiden",

  // Activities
  "activities:create": "Neue Aktivitäten erstellen",
  "activities:read": "Aktivitäten ansehen",
  "activities:update": "Aktivitäten bearbeiten",
  "activities:delete": "Aktivitäten löschen",
  "activities:list": "Aktivitäten auflisten",
  "activities:manage": "Aktivitätenverwaltung (Vollzugriff)",
  "activities:enroll": "Kinder in Aktivitäten einschreiben",
  "activities:assign": "Betreuer zu Aktivitäten zuweisen",
  "activities:manage_categories":
    "Kategorien für Angebote anlegen, umbenennen und archivieren",

  // Roles and permissions
  "roles:create": "Neue Rollen erstellen",
  "roles:read": "Rollen ansehen",
  "roles:update": "Rollen bearbeiten",
  "roles:delete": "Rollen löschen",
  "permissions:create": "Neue Berechtigungen erstellen",
  "permissions:read": "Berechtigungen ansehen",
  "permissions:update": "Berechtigungen bearbeiten",
  "permissions:delete": "Berechtigungen löschen",

  // Rooms
  "rooms:create": "Neue Räume erstellen",
  "rooms:read": "Räume ansehen",
  "rooms:update": "Räume bearbeiten",
  "rooms:delete": "Räume löschen",
  "rooms:list": "Räume auflisten",
  "rooms:manage": "Raumverwaltung (Vollzugriff)",

  // Groups
  "groups:create": "Neue Gruppen erstellen",
  "groups:read": "Gruppen ansehen",
  "groups:update": "Gruppen bearbeiten",
  "groups:delete": "Gruppen löschen",
  "groups:list": "Gruppen auflisten",
  "groups:manage": "Gruppenverwaltung (Vollzugriff)",
  "groups:assign": "Kinder zu Gruppen zuweisen",

  // Substitutions
  "substitutions:create": "Neue Vertretungen erstellen",
  "substitutions:read": "Vertretungen ansehen",
  "substitutions:update": "Vertretungen bearbeiten",
  "substitutions:delete": "Vertretungen löschen",
  "substitutions:list": "Vertretungen auflisten",
  "substitutions:manage": "Vertretungsverwaltung (Vollzugriff)",

  // Schedules
  "schedules:create": "Neue Stundenpläne erstellen",
  "schedules:read": "Stundenpläne ansehen",
  "schedules:update": "Stundenpläne bearbeiten",
  "schedules:delete": "Stundenpläne löschen",
  "schedules:list": "Stundenpläne auflisten",
  "schedules:manage": "Betreuungsplanverwaltung (Vollzugriff)",

  // Visits
  "visits:create": "Neue Besuche erstellen",
  "visits:read": "Besuche ansehen",
  "visits:update": "Besuche bearbeiten",
  "visits:delete": "Besuche löschen",
  "visits:list": "Besuche auflisten",
  "visits:manage": "Besuchsverwaltung (Vollzugriff)",

  // Feedback
  "feedback:create": "Neues Feedback erstellen",
  "feedback:read": "Feedback ansehen",
  "feedback:delete": "Feedback löschen",
  "feedback:list": "Feedback auflisten",
  "feedback:manage": "Feedbackverwaltung (Vollzugriff)",

  // Config
  "config:read": "Konfiguration ansehen",
  "config:update": "Konfiguration bearbeiten",
  "config:manage": "Konfigurationsverwaltung (Vollzugriff)",

  // IoT
  "iot:read": "IoT-Geräte ansehen",
  "iot:update": "IoT-Geräte bearbeiten",
  "iot:manage": "IoT-Geräteverwaltung (Vollzugriff)",

  // Auth
  "auth:manage": "Authentifizierungsverwaltung (Vollzugriff)",

  // Time Tracking
  "time_tracking:own": "Eigene Arbeitszeiten erfassen",
  "time_tracking:manage":
    "Arbeitszeiten aller Mitarbeitenden ansehen und bearbeiten",

  // Urlaub
  "vacation:approve": "Urlaubsanträge genehmigen oder ablehnen",

  // Info-Point
  "display:read": "Info-Point-Anzeigen und ihren Status ansehen",
  "display:manage": "Info-Point-Anzeigen anlegen, umbenennen und löschen",

  // Dateiablage
  "files:manage":
    "Ordner anlegen, festlegen wer sie sieht, Dateien hochladen und löschen",

  // Staff Stammdaten
  "staff:financial":
    "Bank- und Steuerdaten von Mitarbeitenden ansehen und bearbeiten (IBAN, Steuer-ID, SV-Nummer)",
  "staff:stammdaten":
    "Personalstammdaten von Mitarbeitenden ansehen und bearbeiten (Geburtsdatum, Privatanschrift, Notfallkontakt, Vertrag, Qualifikationen)",
  "staff:documents":
    "Allgemeine Personalunterlagen von Mitarbeitenden verwalten (Arbeitsvertrag, Zeugnis, Bewerbung, Sonstiges)",
  "staff:manage":
    "Mitarbeiter-Datensätze anderer Personen ändern (Notizen, Betreuungsprofil, Qualifikationen)",

  // Personalunterlagen und Unterlagen der Kinder
  "staff_documents:health":
    "AU-Bescheinigungen von Mitarbeitenden ansehen, hochladen und löschen",
  "student_documents:health":
    "Gesundheitsunterlagen von Kindern ansehen, hochladen und löschen (Attest, Impfnachweis, Medikamentenplan)",
  "student_documents:legal":
    "Sorgerechtsnachweise von Kindern ansehen, hochladen und löschen",

  // Schul-Portal (moto schule)
  "class_day:read": "Tagesansicht der eigenen Klassen ansehen",
  "class_day:arrival_exception_write":
    "Für eine eigene Klasse an einem Tag eine andere Ankunftszeit eintragen",
  "staff_notices:read": "Tagesinformationen der Leitung lesen und bestätigen",
  "supervision:own": "Eigene Aufsichten aus dem Betreuungsplan durchführen",

  // Guardian payment data
  "guardians:financial":
    "Bankverbindungen von Eltern ansehen, bearbeiten und als Liste exportieren (IBAN)",

  // Calendar
  "calendar:own": "Eigenen Kalender nutzen und Einladungen beantworten",
  "calendar:manage": "Termine und Einladungen erstellen und verwalten",

  // Grade Transitions
  "grade_transitions:read": "Klassenwechsel ansehen",
  "grade_transitions:create": "Klassenwechsel erstellen",
  "grade_transitions:update": "Klassenwechsel bearbeiten",
  "grade_transitions:delete": "Klassenwechsel löschen",
  "grade_transitions:apply": "Klassenwechsel anwenden/zurücksetzen",

  // System & Admin
  "system:manage": "Systemeinstellungen verwalten",
  "admin:*": "Vollzugriff auf alle Ressourcen",
  "*:*": "Vollzugriff auf alle Ressourcen",

  // Legacy names (from initial migration)
  "user:create": "Neue Benutzer erstellen",
  "user:read": "Benutzerinformationen ansehen",
  "user:update": "Benutzerinformationen bearbeiten",
  "user:delete": "Benutzer löschen",
  "role:create": "Neue Rollen erstellen",
  "role:read": "Rolleninformationen ansehen",
  "role:update": "Rolleninformationen bearbeiten",
  "role:delete": "Rollen löschen",
  "permission:create": "Neue Berechtigungen erstellen",
  "permission:read": "Berechtigungsinformationen ansehen",
  "permission:update": "Berechtigungsinformationen bearbeiten",
  "permission:delete": "Berechtigungen löschen",
};

/**
 * Returns a German description for the given permission.
 * Falls back to the DB description if no German translation exists.
 */
export function localizeDescription(
  resource: string,
  action: string,
  dbDescription?: string,
): string {
  const key = `${resource}:${action}`;
  return permissionDescriptions[key] ?? dbDescription ?? "";
}
