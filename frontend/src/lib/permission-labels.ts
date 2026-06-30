// Localization helpers for permission resources/actions

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
  suggestions: "Vorschläge",
  time_tracking: "Zeiterfassung",
  grade_transitions: "Klassenwechsel",
  calendar: "Kalender",
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
  "*": "Alle",
};

export function localizeResource(resource: string): string {
  return resourceLabels[resource] ?? resource;
}

export function localizeAction(action: string): string {
  return actionLabels[action] ?? action;
}

export function formatPermissionDisplay(
  resource: string,
  action: string,
): string {
  return `${localizeResource(resource)}: ${localizeAction(action)}`;
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

  // Activities
  "activities:create": "Neue Aktivitäten erstellen",
  "activities:read": "Aktivitäten ansehen",
  "activities:update": "Aktivitäten bearbeiten",
  "activities:delete": "Aktivitäten löschen",
  "activities:list": "Aktivitäten auflisten",
  "activities:manage": "Aktivitätenverwaltung (Vollzugriff)",
  "activities:enroll": "Kinder in Aktivitäten einschreiben",
  "activities:assign": "Betreuer zu Aktivitäten zuweisen",

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

  // Suggestions
  "suggestions:create": "Neue Vorschläge erstellen",
  "suggestions:read": "Vorschläge ansehen",
  "suggestions:update": "Vorschläge bearbeiten",
  "suggestions:delete": "Vorschläge löschen",
  "suggestions:list": "Vorschläge auflisten",
  "suggestions:manage": "Vorschlagsverwaltung (Vollzugriff)",

  // Time Tracking
  "time_tracking:own": "Eigene Arbeitszeiten erfassen",

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
