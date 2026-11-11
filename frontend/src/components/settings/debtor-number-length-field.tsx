"use client";

// Die Länge der Debitorennummer des Trägers (#6675): unter dem Schalter
// „Debitorennummer: nur Ziffern“ zwei kleine Zahlenfelder „von … bis …
// Ziffern“ mit einem Beispiel, das sich beim Tippen mitändert. Wie jede
// Einstellung speichert ein Feld beim Verlassen selbst. Liegt „von“ über
// „bis“, zieht moto das andere Ende mit, damit beide Werte zusammenpassen.

import { useEffect, useId, useState } from "react";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import {
  DEBTOR_NUMBER_MAX_LENGTH,
  debtorNumberExample,
  debtorNumberFormat,
} from "~/lib/debtor-number-format";
import type { ResolvedSetting } from "~/lib/settings-api";

/** The Träger's switch and its two lengths (registry keys, #6675). */
export const DEBTOR_NUMBER_DIGITS_ONLY_KEY =
  "parent_billing.debtor_number_digits_only";
export const DEBTOR_NUMBER_MIN_DIGITS_KEY =
  "parent_billing.debtor_number_min_digits";
export const DEBTOR_NUMBER_MAX_DIGITS_KEY =
  "parent_billing.debtor_number_max_digits";

function digitsOf(setting: ResolvedSetting): number {
  return typeof setting.value === "number" ? setting.value : 1;
}

/** A whole number from 1 to 35, else null. */
function parseDigits(text: string): number | null {
  if (!/^\d{1,2}$/.test(text.trim())) return null;
  const digits = Number(text.trim());
  return digits >= 1 && digits <= DEBTOR_NUMBER_MAX_LENGTH ? digits : null;
}

export function DebtorNumberLengthField({
  minSetting,
  maxSetting,
  onSave,
}: Readonly<{
  minSetting: ResolvedSetting;
  maxSetting: ResolvedSetting;
  /** Saves one value; throws the ApiError of a failed save. */
  onSave: (key: string, value: unknown) => Promise<void>;
}>) {
  const storedMin = digitsOf(minSetting);
  const storedMax = digitsOf(maxSetting);
  const [minText, setMinText] = useState(String(storedMin));
  const [maxText, setMaxText] = useState(String(storedMax));
  const [saving, setSaving] = useState(false);
  const fieldId = useId();
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();
  const writable = minSetting.writable && maxSetting.writable;

  useEffect(() => setMinText(String(storedMin)), [storedMin]);
  useEffect(() => setMaxText(String(storedMax)), [storedMax]);

  const min = parseDigits(minText);
  const max = parseDigits(maxText);
  const problem =
    min === null || max === null
      ? `Bitte geben Sie eine Zahl von 1 bis ${DEBTOR_NUMBER_MAX_LENGTH} ein.`
      : null;
  const example = debtorNumberExample(
    debtorNumberFormat(true, min ?? storedMin, max ?? storedMax),
  );

  /**
   * Saves what changed after leaving a field. The other end follows when the
   * range would turn round: „von 7“ with „bis 5“ makes it 7 to 7.
   */
  const commit = async (edited: "min" | "max") => {
    if (min === null || max === null || saving) return;
    let nextMin = min;
    let nextMax = max;
    if (nextMin > nextMax) {
      if (edited === "min") nextMax = nextMin;
      else nextMin = nextMax;
    }
    setMinText(String(nextMin));
    setMaxText(String(nextMax));
    const changes: [string, number][] = [];
    if (nextMin !== storedMin) changes.push([minSetting.key, nextMin]);
    if (nextMax !== storedMax) changes.push([maxSetting.key, nextMax]);
    if (changes.length === 0) return;
    setSaving(true);
    try {
      for (const [key, value] of changes) {
        await onSave(key, value);
      }
      success("Die Länge der Debitorennummer ist gespeichert.");
    } catch (err) {
      void showError(err, { object: "die Länge der Debitorennummer" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="px-3 pb-4">
      <InsetPanel padding="md" className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2">
          <div
            role="group"
            aria-labelledby={`${fieldId}-label`}
            aria-describedby={`${fieldId}-example ${fieldId}-note`}
            className="flex flex-wrap items-center gap-x-2 gap-y-2 text-sm text-gray-700"
          >
            <span
              id={`${fieldId}-label`}
              className="mr-1 basis-full font-medium text-gray-900 sm:basis-auto"
            >
              Länge
            </span>
            <span className="flex items-center gap-2 whitespace-nowrap">
              von
              <span className="w-14">
                <Input
                  id={`${fieldId}-min`}
                  name={minSetting.key}
                  aria-label="Ziffern von"
                  controlSize="compact"
                  inputMode="numeric"
                  maxLength={2}
                  className="text-center tabular-nums"
                  value={minText}
                  disabled={!writable || saving}
                  aria-invalid={min === null}
                  aria-describedby={
                    min === null ? `${fieldId}-problem` : undefined
                  }
                  onChange={(event) => setMinText(event.target.value)}
                  onBlur={() => void commit("min")}
                />
              </span>
              bis
              <span className="w-14">
                <Input
                  id={`${fieldId}-max`}
                  name={maxSetting.key}
                  aria-label="Ziffern bis"
                  controlSize="compact"
                  inputMode="numeric"
                  maxLength={2}
                  className="text-center tabular-nums"
                  value={maxText}
                  disabled={!writable || saving}
                  aria-invalid={max === null}
                  aria-describedby={
                    max === null ? `${fieldId}-problem` : undefined
                  }
                  onChange={(event) => setMaxText(event.target.value)}
                  onBlur={() => void commit("max")}
                />
              </span>
              Ziffern
            </span>
          </div>
          <p id={`${fieldId}-example`} className="text-sm text-gray-600">
            Beispiel{" "}
            <span className="font-semibold text-gray-900 tabular-nums">
              {example}
            </span>
          </p>
        </div>
        <FieldErrorHint id={`${fieldId}-problem`} message={problem} live />
        <p id={`${fieldId}-note`} className="text-xs text-gray-600">
          Für genau 5 Ziffern tragen Sie bei von und bis 5 ein. Gespeicherte
          Nummern bleiben, wie sie sind.
        </p>
      </InsetPanel>
    </div>
  );
}
