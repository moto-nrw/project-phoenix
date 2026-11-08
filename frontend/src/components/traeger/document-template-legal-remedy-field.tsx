"use client";

// Die Rechtsbehelfsbelehrung des Bescheids (#4688): Der Träger schreibt sie
// selbst, moto liefert keinen Rechtstext. Jeder Gebührenbescheid druckt sie
// nach dem Brieftext. Ohne sie gibt moto keinen Gebührenbescheid frei.

import { Alert } from "~/components/ui/alert";
import { Textarea } from "~/components/ui/textarea";

/** Höchstlänge wie im Backend (`templates.MaxLegalRemedyLength`). */
const MAX_LENGTH = 5000;

export function DocumentTemplateLegalRemedyField({
  value,
  onChange,
  error,
  disabled,
}: Readonly<{
  value: string;
  onChange: (value: string) => void;
  error?: string;
  disabled?: boolean;
}>) {
  const empty = value.trim() === "";
  return (
    <div className="flex flex-col gap-2">
      <Textarea
        id="template-legal-remedy"
        label="Rechtsbehelfsbelehrung"
        name="legal_remedy"
        hint="Steht auf jedem Gebührenbescheid nach dem Brieftext. Nennen Sie Frist, Stelle und Form des Widerspruchs."
        rows={6}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        error={error}
        maxLength={MAX_LENGTH}
        disabled={disabled}
      />
      {empty ? (
        <Alert
          type="warning"
          size="compact"
          message="Ohne Rechtsbehelfsbelehrung gibt moto keinen Gebührenbescheid frei."
        />
      ) : null}
    </div>
  );
}
