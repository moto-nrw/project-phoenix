/**
 * Texts of the consent prompt and the consent setting (#4901). The OGS and
 * the school portal are German-only; the parents portal translates them in
 * `parent-analytics-consent-copy.ts`, which keeps next-intl out of the staff
 * bundles.
 */
export interface AnalyticsConsentCopy {
  readonly title: string;
  readonly promptTitle: string;
  readonly promptIntro: string;
  readonly promptPrivacy: string;
  readonly promptNoDisadvantage: string;
  readonly promptChangeLater: string;
  readonly allow: string;
  readonly refuse: string;
  readonly saving: string;
  readonly closeLabel: string;
  readonly sectionDescription: string;
  readonly switchLabel: string;
  readonly schoolOff: string;
  /**
   * Recordings exist only in the OGS portal of a school with Analyse-Freigabe.
   * The consent covers them, so the OGS texts say so before and while they
   * run; portals without recordings leave both empty.
   */
  readonly recordingActive: string;
  readonly recordingPossible: string;
  /** Noun phrase with article for the shared error display. */
  readonly errorObject: string;
}

function germanCopy(
  place: "Profil" | "Einstellungen",
  recordings: boolean,
): AnalyticsConsentCopy {
  return {
    title: "Nutzungsanalyse",
    promptTitle: "Nutzungsanalyse erlauben?",
    promptIntro:
      "moto möchte zählen, welche Seiten und Schaltflächen Sie nutzen. So machen wir moto besser.",
    promptPrivacy: "Namen und Eingaben sieht moto dabei nicht.",
    promptNoDisadvantage: "Wenn Sie ablehnen, können Sie moto genauso nutzen.",
    promptChangeLater: `Ihre Wahl gilt für Ihr Konto. Sie können sie jederzeit unter „${place}“ ändern.`,
    allow: "Erlauben",
    refuse: "Ablehnen",
    saving: "Wird gespeichert…",
    closeLabel: "Später entscheiden",
    sectionDescription:
      "Darf moto zählen, welche Seiten und Schaltflächen Sie nutzen? Namen und Eingaben sieht moto dabei nicht. Das entscheiden nur Sie.",
    switchLabel: "Nutzungsanalyse erlauben",
    schoolOff:
      "Ihre Schule hat die Nutzungsanalyse ausgeschaltet. Ihre Wahl gilt, sobald die Schule sie wieder einschaltet.",
    recordingActive: recordings
      ? "Ihre Schule hat zusätzlich Aufzeichnungen erlaubt. Ihre Wahl gilt auch dafür. Texte, Eingaben und Bilder sind darin unkenntlich. Niemand wird damit bewertet."
      : "",
    recordingPossible: recordings
      ? "Erlaubt Ihre Schule Aufzeichnungen, gilt Ihre Wahl auch dafür. Texte, Eingaben und Bilder sind darin unkenntlich."
      : "",
    errorObject: "die Einstellung zur Nutzungsanalyse",
  };
}

/** OGS portal: changed under „Profil“, the only portal with recordings. */
export const TENANT_ANALYTICS_CONSENT_COPY = germanCopy("Profil", true);

/** moto schule: changed under „Einstellungen“, never recorded. */
export const SCHOOL_ANALYTICS_CONSENT_COPY = germanCopy("Einstellungen", false);
