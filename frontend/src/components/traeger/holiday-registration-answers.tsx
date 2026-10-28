"use client";

// Die Antworten auf die Fragen des Trägers in einer Ferienanmeldung (#5198):
// beschriftet mit der Formularversion, mit der die Eltern die Anmeldung
// abgeschickt haben. Gesundheitsangaben sind gekennzeichnet; sie stehen nur
// hier, nicht in Liste und Export.

import {
  DataField,
  DataGrid,
  InfoText,
} from "~/components/ui/detail-modal-components";
import { formatDate } from "~/lib/date-helpers";
import type { HolidayFormField } from "~/lib/holiday-form-field";

/** The answer as the Träger-Büro reads it; "Keine Angabe" without one. */
function holidayAnswerText(field: HolidayFormField, value: unknown): string {
  if (field.type === "boolean") {
    if (value === true) return "Ja";
    if (value === false) return "Nein";
    return "Keine Angabe";
  }
  const answer = typeof value === "string" ? value.trim() : "";
  if (answer === "") return "Keine Angabe";
  if (field.type === "select") {
    return (
      field.options?.find((option) => option.value === answer)?.label ?? answer
    );
  }
  if (field.type === "date") return formatDate(answer);
  return answer;
}

/**
 * The questions of one scope that carry an answer. A question the parents
 * did not see or left empty has no entry in the stored answers.
 */
export function answeredQuestions(
  fields: readonly HolidayFormField[],
  child: boolean,
  answers: Readonly<Record<string, unknown>>,
): HolidayFormField[] {
  return fields.filter(
    (field) =>
      field.type !== "information" &&
      Boolean(field.applies_to_child) === child &&
      field.key in answers,
  );
}

export function HolidayRegistrationAnswers({
  questions,
  answers,
}: Readonly<{
  questions: readonly HolidayFormField[];
  answers: Readonly<Record<string, unknown>>;
}>) {
  return (
    <DataGrid>
      {questions.map((field) => (
        <DataField
          key={field.key}
          label={field.health ? `${field.label} (Gesundheit)` : field.label}
          fullWidth={field.type === "textarea"}
        >
          <InfoText>{holidayAnswerText(field, answers[field.key])}</InfoText>
        </DataField>
      ))}
    </DataGrid>
  );
}
