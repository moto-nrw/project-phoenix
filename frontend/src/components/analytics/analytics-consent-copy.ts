"use client";

import { useTranslations } from "next-intl";

/**
 * Texts of the consent prompt and the consent setting (#4901). The OGS and
 * the school portal are German-only; the parents portal translates them.
 */
export interface AnalyticsConsentCopy {
  readonly title: string;
  readonly promptTitle: string;
  readonly promptIntro: string;
  readonly promptPrivacy: string;
  readonly promptRecording: string;
  readonly promptNoDisadvantage: string;
  readonly promptChangeLater: string;
  readonly allow: string;
  readonly refuse: string;
  readonly saving: string;
  readonly closeLabel: string;
  readonly sectionDescription: string;
  readonly switchLabel: string;
  readonly schoolOff: string;
  readonly recording: string;
  /** Noun phrase with article for the shared error display. */
  readonly errorObject: string;
}

/** Where the person changes the answer later, as the navigation names it. */
type SettingsPlace = "Profil" | "Einstellungen";

export function germanAnalyticsConsentCopy(
  place: SettingsPlace,
): AnalyticsConsentCopy {
  return {
    title: "Nutzungsanalyse",
    promptTitle: "Nutzungsanalyse erlauben?",
    promptIntro:
      "moto möchte zählen, welche Seiten und Schaltflächen Sie nutzen. So machen wir moto besser.",
    promptPrivacy: "Namen und Eingaben sieht moto dabei nicht.",
    promptRecording:
      "Ihre Schule hat zusätzlich Aufzeichnungen erlaubt. Texte, Eingaben und Bilder sind darin unkenntlich.",
    promptNoDisadvantage: "Wenn Sie ablehnen, können Sie moto genauso nutzen.",
    promptChangeLater: `Sie können das jederzeit unter „${place}“ ändern.`,
    allow: "Erlauben",
    refuse: "Ablehnen",
    saving: "Wird gespeichert…",
    closeLabel: "Später entscheiden",
    sectionDescription:
      "Darf moto zählen, welche Seiten und Schaltflächen Sie nutzen? Namen und Eingaben sieht moto dabei nicht. Das entscheiden nur Sie.",
    switchLabel: "Nutzungsanalyse erlauben",
    schoolOff:
      "Ihre Schule hat die Nutzungsanalyse ausgeschaltet. Ihre Wahl gilt, sobald die Schule sie wieder einschaltet.",
    recording:
      "Ihre Schule hat zusätzlich Aufzeichnungen erlaubt. Texte, Eingaben und Bilder sind darin unkenntlich. Niemand wird damit bewertet.",
    errorObject: "die Einstellung zur Nutzungsanalyse",
  };
}

export function useParentAnalyticsConsentCopy(): AnalyticsConsentCopy {
  const t = useTranslations("analyticsConsent");
  return {
    title: t("title"),
    promptTitle: t("promptTitle"),
    promptIntro: t("promptIntro"),
    promptPrivacy: t("promptPrivacy"),
    // Recordings exist only in the OGS portal of a school with Freigabe.
    promptRecording: "",
    promptNoDisadvantage: t("promptNoDisadvantage"),
    promptChangeLater: t("promptChangeLater"),
    allow: t("allow"),
    refuse: t("refuse"),
    saving: t("saving"),
    closeLabel: t("closeLabel"),
    sectionDescription: t("sectionDescription"),
    switchLabel: t("switchLabel"),
    schoolOff: t("schoolOff"),
    recording: "",
    errorObject: t("errorObject"),
  };
}
