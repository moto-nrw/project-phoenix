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
  students: "Kinder",
  roles: "Rollen",
  permissions: "Berechtigungen",
  activities: "Aktivitäten",
  rooms: "Räume",
  groups: "Gruppen",
  visits: "Besuche",
  substitutions: "Vertretungen",
  schedules: "Zeitpläne",
  config: "Einstellungen",
  feedback: "Feedback",
  iot: "Geräte",
  system: "System",
  admin: "Administration",
  time_tracking: "Zeiterfassung",
  grade_transitions: "Schuljahreswechsel",
  calendar: "Kalender",
  staff: "Mitarbeitende",
  files: "Dateiablage",
  auth: "Anmeldung",
  guardians: "Eltern",
  staff_documents: "Personalunterlagen",
  student_documents: "Unterlagen der Kinder",
  enrollment_uploads: "Nachweise aus Anmeldungen",
  staff_notices: "Tagesinformationen",
  surveys: "Umfragen",
  class_day: "Klassenansicht",
  supervision: "Aufsichten",
  display: "Info-Point",
  vacation: "Urlaub",
  meals: "Mittagessen",
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
  orders: "Essenslisten",
  register: "Anmeldungen ändern",
  all_classes: "Alle Klassen",
  school: "In moto schule",
  class_trip_write: "Ausflug melden",
  day_hint_write: "Hinweis für die OGS",
  morning_attendance_write: "Anwesenheit am Vormittag eintragen",
  homework_write: "Hausaufgaben eintragen",
  handover_write: "Klasse an die OGS übergeben",
  parent_letter_write: "Elternbriefe an die Klasse",
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
  "users:manage":
    "Konten anlegen, Rollen vergeben, Konten sperren und Anmeldungen beenden",
  "users:checkin": "Kinder in moto anmelden und abmelden",
  "users:absence":
    "Krankmeldungen und Abwesenheiten von Kindern eintragen und über Anfragen der Eltern entscheiden",

  // Students (ADR 0091): the read-only child views of the role „Gast“.
  "students:read": "Kinder ansehen, ohne Elterndaten",

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
  "roles:create": "Neue Rollen anlegen",
  "roles:read": "Rollen und ihre Berechtigungen ansehen",
  "roles:update": "Name und Beschreibung von Rollen ändern",
  "roles:delete": "Rollen löschen",
  "permissions:create": "Neue Berechtigungen anlegen (nur moto-Team)",
  "permissions:read": "Liste aller Berechtigungen ansehen",
  "permissions:update": "Berechtigungen ändern (nur moto-Team)",
  "permissions:delete": "Berechtigungen löschen (nur moto-Team)",

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
  "config:read": "Einstellungen der Schule ansehen",
  "config:update": "Einstellungen der Schule ändern",
  "config:manage":
    "Alle Einstellungen der Schule ändern, auch Anmeldung, Formulare und Datenschutz",

  // Mittagessen (#3873)
  "meals:read": "Speiseplan und Essenszahlen ansehen",
  "meals:manage": "Speiseplan pflegen und den Caterer verwalten",
  "meals:orders": "Essenslisten mit Namen sehen",
  "meals:register": "Anmeldung eines Kindes ändern, auch nach der Frist",

  // IoT
  "iot:read": "Tablets und Geräte ansehen",
  "iot:update": "Tablets und Geräte bearbeiten",
  "iot:manage": "Tablets und Geräte anlegen und entfernen",

  // Auth
  "auth:manage": "Zahlen zu Rollen und Berechtigungen in der Übersicht sehen",

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
  "enrollment_uploads:read":
    "Nachweise aus Anmeldungen ansehen, zum Beispiel Einkommensnachweise",
  "student_documents:legal":
    "Sorgerechtsnachweise von Kindern ansehen, hochladen und löschen",

  // Schul-Portal (moto schule)
  "class_day:read": "Tagesansicht der eigenen Klassen ansehen",
  "class_day:all_classes": "Alle Klassen der Schule sehen, auch ohne Zuweisung",
  "class_day:arrival_exception_write":
    "Für eine eigene Klasse an einem Tag eine andere Ankunftszeit eintragen",
  "class_day:class_trip_write":
    "Für eine eigene Klasse einen Ausflug oder eine Klassenfahrt melden",
  "class_day:day_hint_write":
    "Für eine eigene Klasse oder eines ihrer Kinder einen Hinweis für heute an die OGS schreiben",
  "class_day:homework_write":
    "Für eine eigene Klasse Hausaufgaben eintragen, ändern und löschen",
  "class_day:morning_attendance_write":
    "Für eine eigene Klasse heute eintragen, wer am Vormittag da ist",
  "class_day:handover_write":
    "Eine eigene Klasse nach dem Unterricht an die OGS übergeben",
  "class_day:parent_letter_write":
    "Elternbriefe an eine eigene Klasse schreiben und sehen, wer sie gelesen hat",
  "staff_notices:read": "Tagesinformationen der Leitung lesen und bestätigen",

  // Umfragen (#4781). Beantworten braucht kein Recht; dieses Recht legt an,
  // veröffentlicht und wertet aus. Auch dann sieht niemand, wer bei einer
  // anonymen Umfrage was geantwortet hat.
  "surveys:manage":
    "Umfragen an Eltern oder das Team anlegen, veröffentlichen und auswerten",
  "supervision:own": "Eigene Aufsichten aus dem Betreuungsplan durchführen",

  // Guardian payment data
  "guardians:financial":
    "Bankverbindungen von Eltern ansehen, bearbeiten und als Liste exportieren (IBAN)",

  // Calendar
  "calendar:own": "Eigenen Kalender nutzen und Einladungen beantworten",
  "calendar:manage": "Termine und Einladungen erstellen und verwalten",
  "calendar:school":
    "Kalender in moto schule nutzen und Schultermine eintragen",

  // Grade Transitions
  "grade_transitions:read": "Schuljahreswechsel ansehen",
  "grade_transitions:create": "Schuljahreswechsel erstellen",
  "grade_transitions:update": "Schuljahreswechsel bearbeiten",
  "grade_transitions:delete": "Schuljahreswechsel löschen",
  "grade_transitions:apply": "Schuljahreswechsel anwenden/zurücksetzen",

  // System & Admin
  "system:manage": "Einstellungen von moto verwalten (nur moto-Team)",
  "admin:*": "Darf alles in der Schule (nur Rolle Verwaltung)",
  "*:*": "Darf alles, auch Bereiche des moto-Teams (nur moto-Team)",

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

/**
 * Permissions no school role may gain (#4574): admin:* stays with the system
 * role Verwaltung, the others belong to the moto team. The role editor hides
 * them, and the backend refuses them with identity.role_platform_permission.
 * Mirrors reservedPermissions in backend/modules/identityaccess/role_templates.go.
 */
const RESERVED_PERMISSIONS: ReadonlySet<string> = new Set([
  "*:*",
  "admin:*",
  "system:manage",
  "permissions:create",
  "permissions:update",
  "permissions:delete",
]);

export function isReservedPermission(
  resource: string,
  action: string,
): boolean {
  return RESERVED_PERMISSIONS.has(`${resource}:${action}`);
}

/**
 * Permissions that let a person change who may do what (#4574). They stay
 * selectable; the role editor shows the hint next to them.
 */
const riskyPermissionHints: Record<string, string> = {
  "roles:create": "Wer Rollen anlegt, bestimmt mit, wer was darf.",
  "roles:update": "Wer Rollen ändert, bestimmt mit, wer was darf.",
  "roles:delete":
    "Wer Rollen löscht, nimmt allen Personen mit dieser Rolle die Rechte.",
  "permissions:read": "Zeigt alle Rechte, die es in moto gibt.",
  "users:manage":
    "Diese Person kann sich und anderen jede Rolle der Schule geben.",
  "auth:manage": "Gehört zur Verwaltung. Nur an die Leitung geben.",
  "config:manage": "Diese Person kann alle Einstellungen der Schule ändern.",
};

/** The hint for a risky permission, or undefined. */
export function riskyPermissionHint(
  resource: string,
  action: string,
): string | undefined {
  return riskyPermissionHints[`${resource}:${action}`];
}
