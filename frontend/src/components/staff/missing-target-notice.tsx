"use client";

// Fehlendes Soll sichtbar machen (#6887). Ohne Arbeitsplan, Arbeitszeitmodell
// und Sonderarbeitszeit ist das Tages-Soll 0: Krankheit, Urlaub und
// Fortbildung zählen dann 0 Stunden, gearbeitete Zeit zählt ganz als Plus.
// Ob ein Soll gilt, entscheidet das Backend mit derselben Regel, nach der es
// rechnet (`has_target` der Monatskarte); hier steht nur, was die Person sieht.

import { Alert } from "~/components/ui/alert";
import { parseISODate } from "~/lib/date-helpers";
import { useStaffHasTarget } from "~/lib/hooks/use-staff-has-target";

/** Reiter der Personalakte, in dem das Soll gepflegt wird. */
export function workTimeModelPath(staffId: string): string {
  return `/staff/${staffId}?tab=arbeitszeitmodell`;
}

/** Soll-Hinweis eines Dialogs. Nur wer Zeitkonten verwaltet
 *  (`time_tracking:manage`), bekommt ihn; `canSetTarget` heißt, der Reiter
 *  „Arbeitszeitmodell“ steht offen (Leitung, `canEdit` der Personalakte). */
export interface MissingTargetHint {
  readonly canSetTarget: boolean;
}

/** Wer das Soll nicht selbst setzen kann, erfährt, wer es tut. */
export const MISSING_TARGET_OWNER = "Das Soll trägt die Leitung ein.";

/**
 * Hinweis in „Krank melden" und „Abwesenheit eintragen": die Person hat im
 * Startmonat kein Soll. Die Aussage gilt nur für diesen Monat, nicht für
 * den gesamten, möglicherweise monatsübergreifenden Zeitraum.
 * Ohne Link, weil ein Seitenwechsel den
 * Dialog samt Eingaben verwerfen würde; der Eintrag bleibt richtig und zählt
 * nach, sobald ein Soll ab diesem Tag gilt (die Gutschrift wird live
 * berechnet, nicht gespeichert).
 */
export function MissingTargetDialogNotice({
  staffId,
  staffName,
  date,
  entryLabel,
  canSetTarget,
}: {
  readonly staffId: string;
  readonly staffName: string;
  /** Erster Tag des Eintrags, `YYYY-MM-DD`. */
  readonly date: string;
  /** „Die Krankmeldung" / „Die Abwesenheit". */
  readonly entryLabel: string;
  readonly canSetTarget: boolean;
}) {
  const hasTarget = useStaffHasTarget(staffId, date, true);
  if (hasTarget !== false) return null;
  const monthLabel = parseISODate(date).toLocaleDateString("de-DE", {
    month: "long",
    year: "numeric",
  });
  return (
    <Alert
      type="warning"
      size="compact"
      announce="polite"
      message={`Für ${staffName} ist im ${monthLabel} kein Soll hinterlegt. ${entryLabel} zählt in diesem Monat deshalb 0 Stunden. Gilt später ein Soll für diese Tage, rechnet moto die Stunden nach. ${
        canSetTarget
          ? "Das Soll legen Sie in der Personalakte unter „Arbeitszeitmodell“ fest."
          : MISSING_TARGET_OWNER
      }`}
    />
  );
}
