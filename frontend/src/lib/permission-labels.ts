// Localization helpers for permission resources/actions

export const resourceLabels: Record<string, string> = {
  users: 'Benutzer',
  roles: 'Rollen',
  permissions: 'Berechtigungen',
  activities: 'Aktivitäten',
  rooms: 'Räume',
  groups: 'Gruppen',
  visits: 'Besuche',
  schedules: 'Zeitpläne',
  config: 'Konfiguration',
  feedback: 'Feedback',
  iot: 'Geräte',
  system: 'System',
  admin: 'Administration',
};

export const actionLabels: Record<string, string> = {
  create: 'Erstellen',
  read: 'Ansehen',
  update: 'Bearbeiten',
  delete: 'Löschen',
  list: 'Auflisten',
  manage: 'Verwalten',
  assign: 'Zuweisen',
  enroll: 'Anmelden',
  '*': 'Alle',
};

export function localizeResource(resource: string): string {
  return resourceLabels[resource] ?? resource;
}

export function localizeAction(action: string): string {
  return actionLabels[action] ?? action;
}

export function formatPermissionDisplay(resource: string, action: string): string {
  return `${localizeResource(resource)}: ${localizeAction(action)}`;
}

