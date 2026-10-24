// The one German name per system role (#4574). The backend stores the
// English role names ("admin", "user", …) for authorization; every screen of
// the tenant portal, the operator portal and the help shows the names below.
// backend/modules/identityaccess/mails.go (invitation mail) and the staff
// import (backend/modules/dataimport/.../staff_import_config.go) carry the
// same names.

export type SystemRoleName =
  "admin" | "user" | "lehrkraft" | "guardian" | "guest" | "ag-leitung";

export const SYSTEM_ROLE_LABELS: Record<
  SystemRoleName,
  { name: string; description: string }
> = {
  admin: {
    name: "Verwaltung",
    description: "Darf alles an der Schule einrichten und verwalten.",
  },
  user: {
    name: "Betreuung",
    description: "Betreut Kinder und Gruppen im Alltag.",
  },
  lehrkraft: {
    name: "Lehrkraft",
    description: "Sieht die Tagesansicht der eigenen Klassen.",
  },
  guardian: {
    name: "Eltern",
    description: "Sieht die Daten der eigenen Kinder.",
  },
  guest: {
    name: "Gast",
    description:
      "Nur ansehen: Kinder, Räume und AGs. Keine Elterndaten, keine Nachrichten.",
  },
  // ADR 0091: the access „Eigene Termine betreuen“ of external caregivers.
  "ag-leitung": {
    name: "AG-Leitung",
    description:
      "Betreut eigene Termine: starten, Kinder ein- und auschecken, beenden",
  },
};

function systemRoleKey(roleName: string): SystemRoleName | undefined {
  const key = roleName.trim().toLowerCase();
  return Object.hasOwn(SYSTEM_ROLE_LABELS, key)
    ? (key as SystemRoleName)
    : undefined;
}

/** The German name of a system role, or undefined for a school's own role. */
export function systemRoleLabel(roleName: string): string | undefined {
  const key = systemRoleKey(roleName);
  return key ? SYSTEM_ROLE_LABELS[key].name : undefined;
}

/** The German description of a system role, or undefined. */
export function systemRoleDescription(roleName: string): string | undefined {
  const key = systemRoleKey(roleName);
  return key ? SYSTEM_ROLE_LABELS[key].description : undefined;
}

/**
 * Whether the name belongs to a system role. "teacher" is the retired
 * predecessor of "user": it can no longer be assigned, but sessions issued
 * before the rename still carry it.
 */
export function isSystemRoleName(roleName: string): boolean {
  return (
    systemRoleKey(roleName) !== undefined ||
    roleName.trim().toLowerCase() === "teacher"
  );
}

/**
 * Templates a new school role starts from (#4574): the backend copies the
 * template role's permissions once, without the platform permissions.
 */
export const ROLE_TEMPLATES = [
  { value: "user", label: `Wie ${SYSTEM_ROLE_LABELS.user.name}` },
  { value: "admin", label: `Wie ${SYSTEM_ROLE_LABELS.admin.name}` },
  { value: "lehrkraft", label: `Wie ${SYSTEM_ROLE_LABELS.lehrkraft.name}` },
] as const;

/**
 * Eltern is a system role the parent-app invitation gives (#4574). Nobody
 * picks it by hand, so the role management neither lists it nor offers it
 * under „Gilt als“. Parent accounts and the role itself stay as they are.
 */
export function isHiddenSystemRole(role: {
  name: string;
  isSystem: boolean;
}): boolean {
  return role.isSystem && systemRoleKey(role.name) === "guardian";
}

/**
 * The „Gilt als“ choices for a school's own role. Eltern is left out: a role
 * that counts as Eltern can never be given to an account (#4574).
 */
export function baseRoleOptions(
  labels: Record<string, string>,
): Array<{ value: string; label: string }> {
  return Object.entries(labels)
    .filter(([value]) => value !== "guardian")
    .map(([value, label]) => ({ value, label }));
}

/**
 * Whether a role is a Lehrkraft role (#1772): the system role lehrkraft or a
 * school role made from the template „Wie Lehrkraft“ (#4574). Such a role
 * works in moto schule, never with a caregiver profile. A school role that
 * merely carries the name is not one.
 */
export function isLehrkraftRole(role: {
  name: string;
  isSystem: boolean;
  schoolPortal?: boolean;
}): boolean {
  return (
    role.schoolPortal === true ||
    (role.isSystem && role.name.trim().toLowerCase() === "lehrkraft")
  );
}
