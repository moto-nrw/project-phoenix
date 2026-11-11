import { useId, type ReactNode } from "react";

import { FieldErrorHint } from "./field-error-hint";
import { FieldLabel } from "./field-label";

/**
 * Die Feld-Hülle des Kits (#6579): Beschriftung über dem Feld, Hinweis und
 * Fehler darunter, die Ids dazu und ihre Verknüpfung über
 * `aria-describedby`. `Input`, `Textarea`, `CustomSelect`, `DatePicker`,
 * `ISODatePicker`, `ISODateInput`, `WeekdayToggleGroup` und `TimeField`
 * bauen darauf; ein Feld schreibt diese Teile nicht selbst.
 *
 * Zwei Farben sind noch nicht angeglichen und bleiben, bis #6798 entscheidet,
 * benannte Abweichungen statt Klassen im Feld: Die Datumsfelder zeigen ihren
 * Fehler im helleren Rot (`errorTone="base"`), das Zeitfeld seinen Hinweis
 * in Grau-500 (`hintTone="muted"`).
 */

/** Ids eines Feldes und die Liste für `aria-describedby`. */
export interface FieldIds {
  /** Id des Bedienelements. */
  readonly id: string;
  readonly hintId: string | undefined;
  readonly errorId: string | undefined;
  /** Eigene Beschreibung des Aufrufers, Hinweis, Fehler; sonst `undefined`. */
  readonly describedBy: string | undefined;
}

/**
 * Vergibt die Ids eines Feldes. Ohne `id` des Aufrufers nimmt das Feld eine
 * erzeugte, damit Beschriftung, Hinweis und Fehler trotzdem am Feld hängen.
 */
export function useFieldIds({
  id,
  hint,
  error,
  describedBy,
}: Readonly<{
  id?: string;
  hint?: ReactNode;
  error?: string | null;
  /** `aria-describedby` des Aufrufers; steht vor Hinweis und Fehler. */
  describedBy?: string;
}>): FieldIds {
  const generatedId = useId();
  const fieldId = id ?? generatedId;
  const hintId = hint ? `${fieldId}-hint` : undefined;
  const errorId = error ? `${fieldId}-error` : undefined;
  return {
    id: fieldId,
    hintId,
    errorId,
    describedBy:
      [describedBy, hintId, errorId].filter(Boolean).join(" ") || undefined,
  };
}

const HINT_TONE_CLASS = {
  default: "text-gray-600",
  muted: "text-gray-500",
} as const;

/** Die Zeile unter einem Feld, die stehen bleibt und mitgelesen wird. */
export function FieldHint({
  id,
  children,
  tone = "default",
  visuallyHidden = false,
}: Readonly<{
  id: string | undefined;
  children: ReactNode;
  /** `muted`: das Grau des Zeitfelds, siehe oben. */
  tone?: keyof typeof HINT_TONE_CLASS;
  /** Nur für Vorleser, wenn das Umfeld den Hinweis schon sichtbar trägt. */
  visuallyHidden?: boolean;
}>) {
  return (
    <p
      id={id}
      className={
        visuallyHidden ? "sr-only" : `mt-1 text-xs ${HINT_TONE_CLASS[tone]}`
      }
    >
      {children}
    </p>
  );
}

export function FieldShell({
  ids,
  label,
  labelId,
  labelFor,
  required = false,
  invalid = false,
  hint,
  hintTone,
  hintHidden = false,
  error,
  errorLive = false,
  errorTone,
  keepWrapper = false,
  className,
  children,
}: Readonly<{
  ids: FieldIds;
  label?: ReactNode;
  /** Id der Beschriftung, wenn das Feld sie über `aria-labelledby` nennt. */
  labelId?: string;
  /** Ziel der Beschriftung; Standard ist `ids.id`. */
  labelFor?: string | null;
  /** Stern hinter der Beschriftung. */
  required?: boolean;
  /** Die Beschriftung wird rot wie der Fehler. */
  invalid?: boolean;
  hint?: ReactNode;
  hintTone?: "default" | "muted";
  hintHidden?: boolean;
  error?: string | null;
  /** Fehler sofort vorlesen (`role="alert"`), für eine Prüfung beim Tippen. */
  errorLive?: boolean;
  errorTone?: "strong" | "base";
  /**
   * Die Hülle auch ohne Beschriftung, Hinweis und Fehler rendern. `Input`
   * und `Textarea` brauchen sie, weil ihre Breite an ihr hängt.
   */
  keepWrapper?: boolean;
  /** Layout der Hülle. */
  className?: string;
  children: ReactNode;
}>) {
  if (!label && !hint && !error && !keepWrapper) return <>{children}</>;
  return (
    <div className={className}>
      {label ? (
        <FieldLabel
          id={labelId}
          htmlFor={labelFor === null ? undefined : (labelFor ?? ids.id)}
          required={required}
          invalid={invalid}
        >
          {label}
        </FieldLabel>
      ) : null}
      {children}
      {hint ? (
        <FieldHint id={ids.hintId} tone={hintTone} visuallyHidden={hintHidden}>
          {hint}
        </FieldHint>
      ) : null}
      {ids.errorId ? (
        <FieldErrorHint
          id={ids.errorId}
          message={error}
          live={errorLive}
          tone={errorTone}
        />
      ) : null}
    </div>
  );
}
