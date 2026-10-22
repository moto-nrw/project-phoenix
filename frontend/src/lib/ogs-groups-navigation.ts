/**
 * Der eine Navigationseintrag „Gruppen“ (#4351). Seitenleiste, mobile
 * Navigation, Breadcrumb und Seitentitel lesen Namen und Ziel von hier.
 *
 * Bis #4351 standen die Gruppen unter „Meine Gruppen“ und „Weitere Gruppen“
 * (#2803, #3890), zwei Einträge mit demselben Symbol. Jetzt führt ein Eintrag
 * auf die Auswahlseite; dort ist die eigene Gruppe markiert.
 */
export const OGS_GROUPS_LABEL = "Gruppen";

/** Der Bereich der Gruppenseiten; Schlüssel des Eintrags in den Navigationen. */
export const OGS_GROUPS_PATH = "/ogs-groups";

/** Die Auswahlseite mit allen Gruppen, die die Person sehen darf. */
export const OGS_GROUPS_OVERVIEW_PATH = "/ogs-groups/overview";

/** Die Seite einer Gruppe. */
export function ogsGroupHref(groupId: string): string {
  return `${OGS_GROUPS_PATH}?group=${encodeURIComponent(groupId)}`;
}

/**
 * Wohin ein Navigationseintrag führt. Der Eintrag „Gruppen“ steht für den
 * ganzen Bereich /ogs-groups und leuchtet dort; sein Ziel ist die
 * Auswahlseite. Alle anderen Einträge führen auf ihren Pfad.
 */
export function navigationEntryTarget(href: string): string {
  return href === OGS_GROUPS_PATH ? OGS_GROUPS_OVERVIEW_PATH : href;
}

/**
 * Eine eigene Gruppe: Gruppenleitung oder Vertretung. Fehlt das Kennzeichen,
 * zählt die Gruppe als eigene; so liefern die älteren Endpunkte sie.
 */
export function isOwnGroup(group: { readonly is_personal?: boolean }): boolean {
  return group.is_personal !== false;
}
