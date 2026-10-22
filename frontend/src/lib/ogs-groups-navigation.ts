/**
 * Der eine Navigationseintrag „Gruppen“ (#4351). Seitenleiste, mobile
 * Navigation, Breadcrumb und Seitentitel lesen Namen und Ziel von hier.
 *
 * Bis #4351 standen die Gruppen unter „Meine Gruppen“ und „Weitere Gruppen“
 * (#2803, #3890), zwei Einträge mit demselben Symbol. Jetzt führt ein Eintrag
 * auf die Auswahlseite; dort ist die eigene Gruppe markiert.
 */
export const OGS_GROUPS_LABEL = "Gruppen";

/** Die Auswahlseite mit allen Gruppen, die die Person sehen darf. */
export const OGS_GROUPS_OVERVIEW_PATH = "/ogs-groups/overview";

/** Die Seite einer Gruppe. */
export function ogsGroupHref(groupId: string): string {
  return `/ogs-groups?group=${encodeURIComponent(groupId)}`;
}
