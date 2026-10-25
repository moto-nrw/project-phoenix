"use client";

/**
 * Bausteine der Abholerlaubnis (#4780): die Auswahl der Wochentage einer
 * Abholberechtigung und das Unterschriftsfeld mit dem Text, den die Eltern
 * unterschreiben. Kontaktdialog, Abholdialog und Tagesdialog binden sie gleich
 * ein. Das Feld zeigt den serverseitig gebildeten deutschen Wortlaut, der
 * unverändert gespeichert und als Nachweis geprüft wird.
 */

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useState } from "react";
import { Checkbox } from "~/components/ui/checkbox";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
import {
  SignaturePad,
  type SignaturePadLabels,
  type SignatureValue,
} from "~/components/ui/signature-pad";
import {
  getPickupSignatureStatement,
  type PickupSignatureStatementInput,
} from "~/lib/parent-api";

/** Plan-Wochentage 1 = Montag … 5 = Freitag. */
const PICKUP_WEEKDAYS = [1, 2, 3, 4, 5] as const;

/** Sortiert, ohne Doppelte; alle Tage werden zur leeren Liste. */
export function normalizeWeekdays(weekdays: readonly number[]): number[] {
  const unique = [...new Set(weekdays)]
    .filter((day) => day >= 1 && day <= 5)
    .sort((a, b) => a - b);
  return unique.length === PICKUP_WEEKDAYS.length ? [] : unique;
}

/** True, wenn zwei Tageslisten dasselbe erlauben. */
export function sameWeekdays(
  a: readonly number[] | undefined,
  b: readonly number[] | undefined,
): boolean {
  const left = normalizeWeekdays(a ?? []);
  const right = normalizeWeekdays(b ?? []);
  return (
    left.length === right.length && left.every((day, i) => day === right[i])
  );
}

/**
 * Loads the server-owned declaration before the signature field appears. The
 * submit endpoint rejects a statement that no longer matches its live values.
 */
export function usePickupSignatureStatement(
  studentId: string | undefined,
  input: PickupSignatureStatementInput | null,
): {
  statement: string | null;
  error: unknown | null;
  retry: () => void;
} {
  const [statement, setStatement] = useState<string | null>(null);
  const [error, setError] = useState<unknown | null>(null);
  const [attempt, setAttempt] = useState(0);
  const key = input === null ? "" : JSON.stringify(input);
  const retry = useCallback(() => setAttempt((current) => current + 1), []);

  useEffect(() => {
    if (!studentId || key === "") {
      setStatement(null);
      setError(null);
      return;
    }
    const request = JSON.parse(key) as PickupSignatureStatementInput;
    let active = true;
    setStatement(null);
    setError(null);
    void (async () => {
      try {
        const next = await getPickupSignatureStatement(studentId, request);
        if (active) setStatement(next);
      } catch (requestError) {
        if (active) {
          setStatement(null);
          setError(requestError);
        }
      }
    })();
    return () => {
      active = false;
    };
  }, [studentId, key, attempt]);

  return { statement, error, retry };
}

/** Shows a failed declaration load where the signature field would appear. */
export function PickupSignatureStatementLoadError({
  error,
  retry,
}: Readonly<{
  error: unknown | null;
  retry: () => void;
}>) {
  const t = useTranslations("parentPickupPermission");
  const { error: loadError, show, clear } = useApiLoadError();

  useEffect(() => {
    if (error) {
      void show(error, { object: t("signature.title"), retry });
    } else {
      clear();
    }
  }, [clear, error, retry, show, t]);

  return <LoadErrorAlert error={loadError} />;
}

/** Kurze Tagesliste, z. B. „Mo, Mi“; leer für alle Tage. */
export function useWeekdayList(): (weekdays: readonly number[]) => string {
  const t = useTranslations("parentPickupPermission");
  return (weekdays) =>
    normalizeWeekdays(weekdays)
      .map((day) => t(`weekday.${day}` as "weekday.1"))
      .join(", ");
}

/** Die Wochentage, an denen eine Person abholen darf. */
export function PickupWeekdaysField({
  value,
  onChange,
  disabled = false,
  error,
}: Readonly<{
  /** Leer: an allen Tagen. */
  value: readonly number[];
  onChange: (next: number[]) => void;
  disabled?: boolean;
  error?: string | null;
}>) {
  const t = useTranslations("parentPickupPermission");
  const everyDay = value.length === 0;
  return (
    <fieldset className="space-y-3" disabled={disabled}>
      <legend className="mb-2 text-sm font-medium text-gray-700">
        {t("days.label")}
      </legend>
      <SegmentedControl
        items={[
          { value: "all", label: t("days.all") },
          { value: "some", label: t("days.some") },
        ]}
        value={everyDay ? "all" : "some"}
        onChange={(next) => onChange(next === "all" ? [] : [1])}
        ariaLabel={t("days.label")}
        fullWidth
      />
      {!everyDay && (
        <div className="flex flex-wrap gap-2">
          {PICKUP_WEEKDAYS.map((day) => {
            const checked = value.includes(day);
            return (
              <ChoiceTile
                key={day}
                selected={checked}
                className="flex items-center gap-2 px-3 py-2 text-sm"
              >
                <Checkbox
                  checked={checked}
                  disabled={checked && value.length === 1}
                  aria-label={t(`weekdayLong.${day}` as "weekdayLong.1")}
                  onChange={(e) =>
                    onChange(
                      e.target.checked
                        ? [...value, day].sort((a, b) => a - b)
                        : value.filter((other) => other !== day),
                    )
                  }
                />
                <span>{t(`weekday.${day}` as "weekday.1")}</span>
              </ChoiceTile>
            );
          })}
        </div>
      )}
      <FieldErrorHint id="pickup-weekdays-error" message={error} />
    </fieldset>
  );
}

/** Die Texte des Unterschriftsfelds in der Sprache der Eltern. */
function usePickupSignatureLabels(): SignaturePadLabels {
  const t = useTranslations("parentDeclaration.signature");
  return {
    label: t("label"),
    hint: t("hint"),
    canvas: t("canvas"),
    clear: t("clear"),
    typeInstead: t("typeInstead"),
    drawInstead: t("drawInstead"),
    typedName: t("typedName"),
    confirm: t("confirm"),
    confirmed: t("confirmed"),
    confirmedTyped: t("confirmedTyped"),
    change: t("change"),
    preview: t("preview"),
    emptyDrawing: t("emptyDrawing"),
    nameTooShort: t("nameTooShort"),
  };
}

/**
 * Die Erklärung, die die Eltern unterschreiben, und das Unterschriftsfeld.
 * Der Wert ist erst nach „Unterschrift bestätigen“ gesetzt.
 */
export function PickupSignatureField({
  id,
  statement,
  value,
  onChange,
  disabled = false,
  error,
}: Readonly<{
  id: string;
  statement: string;
  value: SignatureValue | null;
  onChange: (next: SignatureValue | null) => void;
  disabled?: boolean;
  error?: string | null;
}>) {
  const t = useTranslations("parentPickupPermission");
  const labels = usePickupSignatureLabels();
  return (
    <div className="space-y-3 rounded-xl border border-gray-200 p-4">
      <div>
        <p className="text-sm font-medium text-gray-900">
          {t("signature.title")}
        </p>
        <p className="mt-1 text-sm text-gray-700">{statement}</p>
        <p className="mt-2 text-xs text-gray-500">{t("signature.procedure")}</p>
      </div>
      <SignaturePad
        id={id}
        value={value}
        onChange={onChange}
        disabled={disabled}
        error={error}
        labels={labels}
      />
    </div>
  );
}
