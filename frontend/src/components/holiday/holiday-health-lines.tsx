// Die Gesundheitsangaben eines Kindes aus der Ferienanmeldung (#4722) als
// Zeilen „Bezeichnung: Angabe“: in der Teilnehmerliste, beim Feriengast und
// beim eigenen Kind der ausrichtenden OGS. Nur lesend.

import {
  DataField,
  DataGrid,
  InfoText,
} from "~/components/ui/detail-modal-components";
import {
  formatHolidayHostAnswer,
  type HolidayHostHealth,
} from "~/lib/holiday-host-api";

interface HealthLine {
  readonly label: string;
  readonly value: string;
}

/** Jede Angabe mit Wert als Zeile, in der Reihenfolge des Formulars. */
function holidayHealthLines(health: HolidayHostHealth): HealthLine[] {
  const lines: HealthLine[] = [];
  if (health.allergies) {
    lines.push({ label: "Allergien", value: health.allergies });
  }
  if (health.medication) {
    lines.push({ label: "Medikamente", value: health.medication });
  }
  if (health.healthNotes) {
    lines.push({
      label: "Weitere Hinweise zur Gesundheit",
      value: health.healthNotes,
    });
  }
  for (const answer of health.answers) {
    const value = formatHolidayHostAnswer(answer);
    if (value !== "–") lines.push({ label: answer.label, value });
  }
  return lines;
}

/** Die Angaben eng untereinander, für eine Tabellenzelle. */
export function HolidayHealthCell({
  health,
}: Readonly<{ health: HolidayHostHealth | null }>) {
  const lines = health ? holidayHealthLines(health) : [];
  if (lines.length === 0) return "–";
  return (
    <span className="flex flex-col gap-0.5">
      {lines.map((line) => (
        <span key={line.label}>
          <span className="font-medium">{line.label}:</span> {line.value}
        </span>
      ))}
    </span>
  );
}

/** Die Angaben als Felder einer Detailansicht. */
export function HolidayHealthFields({
  health,
}: Readonly<{ health: HolidayHostHealth }>) {
  return (
    <DataGrid>
      {holidayHealthLines(health).map((line) => (
        <DataField key={line.label} label={line.label} fullWidth>
          <InfoText>{line.value}</InfoText>
        </DataField>
      ))}
    </DataGrid>
  );
}
