/**
 * Die beiden Bereiche der Seitenleiste, unter denen eine OGS-Gruppe steht:
 * die eigenen Gruppen und „Weitere Gruppen" (#2803). Seitenleiste und
 * Breadcrumb lesen den Namen von hier, damit eine fremde Gruppe nicht in der
 * Leiste unter „Weitere Gruppen" und im Breadcrumb unter „Meine Gruppen"
 * steht (#3890).
 */
export type OgsGroupSection = "personal" | "other";

export const OGS_GROUP_SECTION_LABELS: Readonly<
  Record<OgsGroupSection, string>
> = {
  personal: "Meine Gruppen",
  other: "Weitere Gruppen",
};

/**
 * Bereich einer Gruppe nach ihrem `is_personal`-Kennzeichen. Fehlt es, ist
 * die Gruppe eine eigene; so filtert auch die Seitenleiste.
 */
export function ogsGroupSectionOf(
  isPersonal: boolean | undefined,
): OgsGroupSection {
  return isPersonal === false ? "other" : "personal";
}

/**
 * Bereich der zuletzt geöffneten Gruppe, neben `sidebar-last-group` und
 * `sidebar-last-group-name`. Die Kinderseiten kennen die Gruppenliste nicht
 * und lesen den Bereich für ihren Breadcrumb von hier.
 */
export const LAST_GROUP_SECTION_STORAGE_KEY = "sidebar-last-group-section";

export function parseOgsGroupSection(
  value: string | null,
): OgsGroupSection | undefined {
  return value === "personal" || value === "other" ? value : undefined;
}
