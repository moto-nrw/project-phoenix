// Das Muster-Feld der Vorlagen-Katalogseite (#6891): derselbe Wochen-Editor
// wie im Reiter „Arbeitszeitmodell“, dazu der Beginn der Woche A und vor dem
// Speichern der Hinweis, wen eine Änderung erreicht.

import { Alert } from "~/components/ui/alert";
import { ISODatePicker } from "~/components/ui/date-picker";
import { formatCount } from "~/lib/format-utils";

import {
  validatePatternDraft,
  type PatternDraft,
} from "./work-time-pattern-draft";
import { WorkTimePatternEditor } from "./work-time-pattern-editor";

export interface PatternFieldValue {
  readonly draft: PatternDraft;
  /** "YYYY-MM-DD": ab diesem Tag zählt die erste Woche als Woche A. */
  readonly anchorDate: string;
}

function isPatternFieldValue(value: unknown): value is PatternFieldValue {
  return (
    typeof value === "object" &&
    value !== null &&
    "draft" in value &&
    "anchorDate" in value
  );
}

/** Für `validation` des Formularfelds. */
export function validatePatternField(value: unknown): string | null {
  if (!isPatternFieldValue(value)) {
    return "Bitte tragen Sie die Stunden ein.";
  }
  if (value.draft.rotationLength > 1 && !value.anchorDate) {
    return "Bitte wählen Sie, wann Woche A beginnt.";
  }
  return validatePatternDraft(value.draft);
}

/** Wen eine Änderung erreicht, als ganzer Satz. */
function assigneeNotice(count: number): string {
  return count === 1
    ? "1 Person nutzt diese Vorlage. Wenn Sie speichern, gilt das neue Soll ab heute auch für sie."
    : `${formatCount(count)} Personen nutzen diese Vorlage. Wenn Sie speichern, gilt das neue Soll ab heute für alle ${formatCount(count)}.`;
}

export function WorkTimeModelPatternField({
  value,
  onChange,
  assignedCount = 0,
}: {
  readonly value: unknown;
  readonly onChange: (value: unknown) => void;
  readonly label: string;
  /** Personen, die die Vorlage nutzen; 0 beim Anlegen. */
  readonly assignedCount?: number;
}) {
  if (!isPatternFieldValue(value)) return null;
  return (
    <div className="space-y-4">
      {assignedCount > 0 && (
        <Alert type="warning" message={assigneeNotice(assignedCount)} />
      )}
      <WorkTimePatternEditor
        draft={value.draft}
        idPrefix="vorlage"
        onChange={(draft) => onChange({ ...value, draft })}
      >
        {value.draft.rotationLength > 1 && (
          <ISODatePicker
            label="Woche A beginnt am"
            hint="Ab diesem Tag zählt die erste Woche als Woche A."
            value={value.anchorDate}
            onChange={(anchorDate) => onChange({ ...value, anchorDate })}
          />
        )}
      </WorkTimePatternEditor>
    </div>
  );
}
