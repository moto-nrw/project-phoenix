"use client";

// Text und Vorschau der Annahme-Mail eines Ferienzeitraums (#6672). Eltern
// bekommen die Mail, wenn das Büro ihre Anmeldung bestätigt. Kinder, Wochen
// und Zeitmodell setzt moto selbst ein; der Träger schreibt nur seinen Text.
// Die Vorschau zeigt die Mail so, wie Eltern sie lesen: mit einem
// Beispielkind in der ersten Woche des Zeitraums und dem Text, wie er gerade
// im Feld steht. Der Text bleibt Text: Zeilenumbrüche bleiben, HTML nicht.

import { InsetPanel } from "~/components/ui/inset-panel";
import { SectionLabel } from "~/components/ui/section-label";
import { Textarea } from "~/components/ui/textarea";
import { buttonClassName } from "~/components/ui/button";
import { formatDate } from "~/lib/date-helpers";
import { useShellAuthSafe } from "~/lib/shell-auth-context";
import type { TraegerSchool } from "~/lib/traeger-api";
import {
  type HolidayPeriodDraft,
  MAX_ACCEPTANCE_MAIL_TEXT,
} from "./holiday-period-model";

const EXAMPLE_WEEK =
  "04.01.2027 bis 08.01.2027: OGS, bis 16 Uhr (08:00 bis 16:00 Uhr)";

/** The line of the first week as the mail names it, or an example. */
function exampleWeek(
  draft: HolidayPeriodDraft,
  schools: readonly TraegerSchool[],
): string {
  const week = draft.weeks.find(
    (candidate) => candidate.startsOn && candidate.endsOn,
  );
  const model = week?.timeModels.find(
    (candidate) => !candidate.archived && candidate.name.trim(),
  );
  if (!week || !model) return EXAMPLE_WEEK;
  const school = schools.find(
    (candidate) => candidate.id === week.hostSchoolId,
  )?.name;
  const place = school ? `${school}, ${model.name.trim()}` : model.name.trim();
  const times =
    model.startsAt && model.endsAt
      ? ` (${model.startsAt} bis ${model.endsAt} Uhr)`
      : "";
  return `${formatDate(week.startsOn)} bis ${formatDate(week.endsOn)}: ${place}${times}`;
}

export function HolidayAcceptanceMailField({
  draft,
  schools,
  error,
  onChange,
}: Readonly<{
  draft: HolidayPeriodDraft;
  schools: readonly TraegerSchool[];
  error?: string;
  onChange: (text: string) => void;
}>) {
  const traegerName = useShellAuthSafe()?.brandLabel ?? "Ihr Träger";
  const periodName = draft.name.trim() || "Ihr Ferienzeitraum";
  const text = draft.acceptanceMailText.trim();
  const used = [...draft.acceptanceMailText].length;

  return (
    <div className="space-y-4">
      <div>
        <h3 className="text-sm font-semibold text-gray-600">
          Annahme-Mail an Eltern
        </h3>
        <p className="text-sm text-gray-600">
          Eltern bekommen diese E-Mail, wenn Sie ihre Anmeldung bestätigen.
          Kinder, Wochen und Zeitmodell setzt moto selbst ein.
        </p>
      </div>
      <div className="grid gap-6 lg:grid-cols-2">
        <div className="space-y-1">
          <Textarea
            name="acceptance_mail_text"
            label="Text der Annahme-Mail"
            hint="Freiwillig. Zum Beispiel: was die Kinder mitbringen sollen."
            rows={8}
            maxLength={MAX_ACCEPTANCE_MAIL_TEXT}
            placeholder={
              "Zum Beispiel:\nBei Betreuung bis 16 Uhr bitte Mittagessen mitgeben.\nBitte an Wechselkleidung, Sportsachen und Regenkleidung denken."
            }
            value={draft.acceptanceMailText}
            error={error}
            onChange={(event) => onChange(event.target.value)}
          />
          <p
            className="text-right text-xs text-gray-500 tabular-nums"
            aria-live="polite"
          >
            {`${used} von ${MAX_ACCEPTANCE_MAIL_TEXT} Zeichen`}
          </p>
        </div>
        <section aria-label="Vorschau der Annahme-Mail" className="space-y-2">
          <SectionLabel as="h4">Vorschau</SectionLabel>
          <InsetPanel padding="md">
            <InsetPanel
              tone="white"
              padding="md"
              className="space-y-4 text-sm text-gray-700"
            >
              <div className="space-y-1 border-b border-gray-100 pb-3 text-xs text-gray-500">
                <p className="truncate">
                  Von:{" "}
                  <span className="font-medium text-gray-900">
                    {traegerName}
                  </span>
                </p>
                <p>
                  Betreff:{" "}
                  <span className="font-medium text-gray-900">
                    Ferienanmeldung bestätigt: {periodName}
                  </span>
                </p>
              </div>
              <div className="space-y-2">
                <p className="text-base font-semibold text-gray-900">
                  Ihre Ferienanmeldung ist bestätigt
                </p>
                <p>Guten Tag Eva Beispiel,</p>
                <p>
                  der Träger{" "}
                  <strong className="text-gray-900">{traegerName}</strong> hat
                  Ihre Anmeldung für{" "}
                  <strong className="text-gray-900">{periodName}</strong>{" "}
                  angenommen.
                </p>
              </div>
              <InsetPanel tone="white" padding="sm" className="space-y-1">
                <p className="font-semibold text-gray-900">Lia Beispiel</p>
                <p className="text-gray-600">{exampleWeek(draft, schools)}</p>
              </InsetPanel>
              {text ? (
                <InsetPanel tone="white" padding="sm" className="space-y-1">
                  <p className="font-semibold text-gray-900">
                    Hinweise vom Träger
                  </p>
                  <p className="break-words whitespace-pre-line text-gray-600">
                    {text}
                  </p>
                </InsetPanel>
              ) : (
                <InsetPanel dashed padding="sm" className="text-gray-500">
                  Hier steht Ihr Text. Ohne Text geht die E-Mail ohne Hinweise
                  raus.
                </InsetPanel>
              )}
              <div className="space-y-1 text-center">
                <span
                  aria-hidden="true"
                  className={buttonClassName({
                    variant: "primary",
                    size: "md",
                    className: "pointer-events-none",
                  })}
                >
                  In der Eltern-App ansehen
                </span>
                <p className="text-xs text-gray-500">
                  Diesen Knopf sehen nur Eltern mit Elternkonto.
                </p>
              </div>
            </InsetPanel>
          </InsetPanel>
        </section>
      </div>
    </div>
  );
}
