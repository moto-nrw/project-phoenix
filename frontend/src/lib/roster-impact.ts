/**
 * Auswirkung einer Buchungsänderung auf die Regeltermine eines Kindes (#5102).
 *
 * Das Backend rechnet sie beim Speichern aus und liefert sie in der Antwort
 * mit: welche Regeltermine automatisch angepasst wurden und welche das Kind an
 * weggefallenen Wochentagen weiter einplanen, weil sie von Hand gepflegt
 * werden. Die Oberflächen zeigen sie nur an.
 */
import { apiErrorFromBody, transportFetch } from "~/lib/api-error";

/** Wire-Form wie im Backend (`roster_impact`). Wochentage nach ISO, 1 = Montag. */
export interface RosterImpactWire {
  readonly effective_from: string;
  readonly adjusted?: ReadonlyArray<{
    readonly template_id: string;
    readonly name: string;
    readonly removed_weekdays?: readonly number[];
    readonly added_weekdays?: readonly number[];
  }>;
  readonly not_adjusted?: ReadonlyArray<{
    readonly template_id: string;
    readonly name: string;
    readonly weekdays?: readonly number[];
    readonly via_target?: boolean;
  }>;
}

export interface AdjustedRosterTemplate {
  readonly templateId: string;
  readonly name: string;
  readonly removedWeekdays: readonly number[];
  readonly addedWeekdays: readonly number[];
}

export interface ManualRosterTemplate {
  readonly templateId: string;
  readonly name: string;
  readonly weekdays: readonly number[];
  /** Klasse, Jahrgang oder Gruppe nimmt das Kind in neue Termine wieder auf. */
  readonly viaTarget: boolean;
}

export interface RosterImpact {
  /** Erster Tag, ab dem die Änderung gilt (YYYY-MM-DD). */
  readonly effectiveFrom: string;
  readonly adjusted: readonly AdjustedRosterTemplate[];
  readonly notAdjusted: readonly ManualRosterTemplate[];
}

/** Wandelt die Wire-Form um; fehlt sie, gibt es keine Auswirkung zu zeigen. */
export function mapRosterImpact(
  wire: RosterImpactWire | null | undefined,
): RosterImpact | null {
  if (!wire?.effective_from) return null;
  return {
    effectiveFrom: wire.effective_from,
    adjusted: (wire.adjusted ?? []).map((template) => ({
      templateId: template.template_id,
      name: template.name,
      removedWeekdays: template.removed_weekdays ?? [],
      addedWeekdays: template.added_weekdays ?? [],
    })),
    notAdjusted: (wire.not_adjusted ?? []).map((template) => ({
      templateId: template.template_id,
      name: template.name,
      weekdays: template.weekdays ?? [],
      viaTarget: template.via_target === true,
    })),
  };
}

const WEEKDAY_ADVERBS: Record<number, string> = {
  1: "montags",
  2: "dienstags",
  3: "mittwochs",
  4: "donnerstags",
  5: "freitags",
  6: "samstags",
  7: "sonntags",
};

/** „freitags“, „montags und freitags“, „montags, mittwochs und freitags“. */
export function formatWeekdayAdverbs(weekdays: readonly number[]): string {
  const words = [...weekdays]
    .sort((left, right) => left - right)
    .map((weekday) => WEEKDAY_ADVERBS[weekday])
    .filter((word): word is string => word !== undefined);
  if (words.length <= 1) return words[0] ?? "";
  return `${words.slice(0, -1).join(", ")} und ${words[words.length - 1]}`;
}

/** „RDST 1b: freitags entfernt“ als Teilsatz hinter dem Namen. */
export function describeAdjustment(template: AdjustedRosterTemplate): string {
  const parts: string[] = [];
  if (template.removedWeekdays.length > 0) {
    parts.push(`${formatWeekdayAdverbs(template.removedWeekdays)} entfernt`);
  }
  if (template.addedWeekdays.length > 0) {
    parts.push(`${formatWeekdayAdverbs(template.addedWeekdays)} ergänzt`);
  }
  return parts.join(", ");
}

/**
 * Nimmt das Kind ab dem Stichtag an den Wochentagen aus dem von Hand
 * gepflegten Regeltermin: aus den Hand-Einträgen und aus den schon
 * angelegten künftigen Terminen.
 */
export async function removeStudentTemplateDays(input: {
  templateId: string;
  studentId: string;
  weekdays: readonly number[];
  effectiveFrom: string;
}): Promise<void> {
  const response = await transportFetch(
    `/api/timetable/templates/${encodeURIComponent(input.templateId)}/students/${encodeURIComponent(input.studentId)}/remove-days`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        weekdays: input.weekdays,
        effective_from: input.effectiveFrom,
      }),
    },
  );
  if (!response.ok) {
    throw apiErrorFromBody(
      "Das Kind konnte nicht aus dem Regeltermin entfernt werden",
      response.status,
      await response.json().catch(() => undefined),
    );
  }
}
