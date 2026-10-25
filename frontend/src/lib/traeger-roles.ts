/**
 * The fixed Träger roles (ADR 0084 section 4). Each bundles Träger rights;
 * a person may hold several. Shared by the operator portal and the
 * Träger-Portal (#4649, #4650).
 */
export const TRAEGER_ROLES = [
  "administration",
  "billing",
  "personnel",
  "read_only",
] as const;

export type TraegerRole = (typeof TRAEGER_ROLES)[number];

export const TRAEGER_ROLE_LABELS: Readonly<Record<TraegerRole, string>> = {
  administration: "Verwaltung",
  billing: "Abrechnung",
  personnel: "Personal",
  read_only: "Nur lesen",
};

export function isTraegerRole(value: string): value is TraegerRole {
  return (TRAEGER_ROLES as readonly string[]).includes(value);
}

/** The known roles among the given names, in the fixed role order. */
export function traegerRoles(values: readonly string[]): TraegerRole[] {
  return TRAEGER_ROLES.filter((role) => values.includes(role));
}
