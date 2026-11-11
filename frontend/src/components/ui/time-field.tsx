"use client";

import { Clock } from "lucide-react";
import { useEffect, useId, useMemo, useRef, useState } from "react";
import { FocusScope } from "@radix-ui/react-focus-scope";
import { useTranslations } from "next-intl";

import { AnchoredPopover } from "./anchored-popover";
import { Button } from "./button";
import { FieldLabel } from "./field-label";
import { TimePickerPanel, type TimePickerLabels } from "./time-picker-panel";
import { MODAL_SCOPE_SELECTOR, SHEET_SCOPE_SELECTOR } from "./overlay-stack";

// Ein Modal kann das Popover in seiner Fokusfalle aufnehmen. Ein Vaul-Slide-over
// transformiert seinen Inhalt dagegen; dessen Auswahl bleibt am Body und
// bekommt dort eine eigene Fokusfalle.
const PICKER_PORTAL_SCOPE = MODAL_SCOPE_SELECTOR;
const SLIDE_OVER_FOCUS_TRAP = SHEET_SCOPE_SELECTOR;
const PICKER_WIDTH = 168;

/**
 * Nur Ziffern und der Doppelpunkt; auch einstellige Stunden bleiben gültig.
 * Geteilt mit dem Uhrzeitfeld der Einstellungen (`SettingsTimeField`), damit
 * es die Maske nicht ein zweites Mal gibt (#3117).
 */
export function normalizeTimeInput(
  raw: string,
  previousValue = "",
  continueOneDigitHour = false,
): string {
  const explicitTime = raw.match(/^(\d{1,2}):(\d{0,2})$/);
  if (explicitTime) {
    return `${explicitTime[1]!.padStart(2, "0")}:${explicitTime[2]!}`;
  }

  const rawDigits = raw.replace(/\D/g, "");
  // After completing a one-digit hour, a fourth typed digit turns
  // "01:53" + "0" back into the intended two-digit time "15:30".
  if (
    continueOneDigitHour &&
    rawDigits.length === 5 &&
    raw.startsWith(previousValue) &&
    previousValue === `${rawDigits.slice(0, 2)}:${rawDigits.slice(2, 4)}`
  ) {
    return `${rawDigits.slice(1, 3)}:${rawDigits.slice(3)}`;
  }

  const digits = rawDigits.slice(0, 4);
  if (digits.length <= 2) return digits;
  if (digits.length === 3) {
    // A completed three-digit value is a one-digit hour ("130" → "01:30").
    // Only keep a two-digit hour open when the one-digit variant is invalid.
    if (Number(digits.slice(1)) <= 59) {
      return `0${digits.slice(0, 1)}:${digits.slice(1)}`;
    }
    if (
      previousValue === digits.slice(0, 2) &&
      Number(digits.slice(0, 2)) <= 23
    ) {
      return `${digits.slice(0, 2)}:${digits.slice(2)}`;
    }
    return `0${digits.slice(0, 1)}:${digits.slice(1)}`;
  }
  return `${digits.slice(0, 2)}:${digits.slice(2)}`;
}

export function completesOneDigitHour(raw: string): boolean {
  return /^\d{3}$/.test(raw) && Number(raw.slice(1)) <= 59;
}

/**
 * Ein Uhrzeitfeld mit sichtbarem Format.
 *
 * Das native `<input type="time">` zeigt leer ein rohes "--:--" und oeffnet je
 * nach Browser ein Systemrad. Dieses Feld ist ein normales Textfeld mit
 * sichtbarem Formathinweis und automatischem Doppelpunkt nach zwei Ziffern;
 * das Uhr-Symbol oeffnet zusaetzlich eine Zeitauswahl aus dem Kit.
 *
 * Der Wert bleibt "HH:MM" wie beim nativen Feld, damit aufrufender Code und
 * Schnittstelle unveraendert bleiben.
 */
export function TimeField({
  value,
  onChange,
  label,
  hint,
  placeholder,
  required = false,
  invalid = false,
  describedBy,
  inputRef,
  name,
  disabled = false,
  inline = false,
  minTime,
  timePickerLabels,
  onBlur,
  onKeyDown,
}: Readonly<{
  value: string;
  onChange: (value: string) => void;
  label: string;
  /** Sichtbarer Formathinweis, z. B. "Uhrzeit im Format 15:30". */
  hint: string;
  /** Beispielzeit im leeren Feld. Nie das rohe native "--:--". */
  placeholder: string;
  required?: boolean;
  invalid?: boolean;
  describedBy?: string;
  inputRef?: React.Ref<HTMLInputElement>;
  /** Form name; a form's error path focuses the first failed field by it. */
  name?: string;
  disabled?: boolean;
  /**
   * Beside another control in one row, where the surrounding field already
   * carries the visible label: label and hint go to screen readers only, text
   * and frame match the neighbouring `DatePicker`.
   */
  inline?: boolean;
  /** Frühester Wert der Zeitauswahl als "HH:MM"; das Tippen bleibt frei. */
  minTime?: string;
  /** Übersetzte Namen und Hinweise der Uhrzeitauswahl. */
  timePickerLabels?: TimePickerLabels;
  /** Der Fokus hat das Feld samt Uhr und Zeitauswahl verlassen. */
  onBlur?: () => void;
  onKeyDown?: React.KeyboardEventHandler<HTMLInputElement>;
}>) {
  const tTimePicker = useTranslations("datePicker.timePicker");
  const localizedTimePickerLabels = useMemo<TimePickerLabels>(
    () => ({
      selectTime: tTimePicker("selectTime"),
      hour: tTimePicker("hour"),
      minute: tTimePicker("minute"),
      minimum: (time) => tTimePicker("minimum", { time }),
    }),
    [tTimePicker],
  );
  const pickerLabels = timePickerLabels ?? localizedTimePickerLabels;
  const inputId = useId();
  const hintId = useId();
  const completedOneDigitHour = useRef<string | null>(null);
  const fieldRef = useRef<HTMLDivElement>(null);
  const initialOptionRef = useRef<HTMLElement>(null);
  const [pickerOpen, setPickerOpen] = useState(false);
  const open = pickerOpen && !disabled;
  const pressedInput = useRef(false);
  const needsPortalledFocusScope =
    typeof document !== "undefined" &&
    fieldRef.current?.closest(SLIDE_OVER_FOCUS_TRAP) !== null;

  // Ein Feld, das gesperrt wurde, geht danach nicht von selbst wieder auf.
  useEffect(() => {
    if (disabled) setPickerOpen(false);
  }, [disabled]);

  const handleFocusLeave = (event: React.FocusEvent) => {
    if (open) return;
    const next = event.relatedTarget;
    if (next instanceof Node && fieldRef.current?.contains(next)) return;
    onBlur?.();
  };

  // Ein Klick außerhalb schließt die Auswahl, ohne dass Feld oder Knopf den
  // Fokus noch einmal bekommen: Auch dann hat die Person das Feld verlassen.
  // Ein Klick ins Eingabefeld schließt sie ebenfalls, doch das Feld bekommt den
  // Fokus erst danach; die Person bleibt also im Feld.
  const wasOpen = useRef(false);
  useEffect(() => {
    if (
      wasOpen.current &&
      !open &&
      !pressedInput.current &&
      !fieldRef.current?.contains(document.activeElement)
    ) {
      onBlur?.();
    }
    wasOpen.current = open;
    pressedInput.current = false;
  }, [open, onBlur]);

  return (
    <div className="block">
      {!inline && (
        <FieldLabel htmlFor={inputId} required={required}>
          {label}
        </FieldLabel>
      )}
      <div ref={fieldRef} className="relative">
        <input
          ref={inputRef}
          id={inputId}
          name={name}
          type="text"
          inputMode="numeric"
          autoComplete="off"
          value={value}
          placeholder={placeholder}
          maxLength={6}
          required={required}
          disabled={disabled}
          aria-label={inline ? label : undefined}
          aria-required={required}
          aria-invalid={invalid}
          aria-describedby={[describedBy, hintId].filter(Boolean).join(" ")}
          onChange={(event) => {
            const nextValue = normalizeTimeInput(
              event.target.value,
              value,
              completedOneDigitHour.current === value,
            );
            completedOneDigitHour.current = completesOneDigitHour(
              event.target.value,
            )
              ? nextValue
              : null;
            onChange(nextValue);
          }}
          onKeyDown={onKeyDown}
          onMouseDown={() => {
            pressedInput.current = true;
          }}
          onBlur={(event) => {
            completedOneDigitHour.current = null;
            handleFocusLeave(event);
          }}
          className={`h-10 w-full rounded-lg border pr-10 pl-3 text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-400 pointer-coarse:pr-12 ${
            inline ? "bg-white text-sm" : "text-base"
          } ${
            invalid
              ? inline
                ? "border-moto-red focus-visible:border-moto-red"
                : "border-parent-red focus-visible:border-parent-red"
              : `focus-visible:border-gray-400 ${inline ? "border-gray-200" : "border-gray-300"}`
          }`}
        />
        <AnchoredPopover
          open={open}
          onOpenChange={setPickerOpen}
          ariaLabel={pickerLabels.selectTime}
          initialFocusRef={initialOptionRef}
          preferredWidth={PICKER_WIDTH}
          align="end"
          portalScopeSelector={PICKER_PORTAL_SCOPE}
          renderTrigger={({ ref, open: isOpen, panelId, toggle }) => (
            <Button
              ref={ref}
              type="button"
              variant="ghost"
              size="icon"
              disabled={disabled}
              aria-label={pickerLabels.selectTime}
              aria-haspopup="dialog"
              aria-expanded={isOpen}
              aria-controls={isOpen ? panelId : undefined}
              // Das Feld behält den Fokus, bis die Auswahl ihn übernimmt.
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => {
                if (isOpen) ref.current?.focus();
                toggle();
              }}
              onBlur={handleFocusLeave}
              className="absolute top-1/2 right-1 -translate-y-1/2 pointer-coarse:right-0"
            >
              <Clock className="h-4 w-4" aria-hidden="true" />
            </Button>
          )}
        >
          {({ close }) => {
            const picker = (
              <TimePickerPanel
                value={value}
                minTime={minTime}
                labels={pickerLabels}
                onChange={onChange}
                close={close}
                initialFocusRef={initialOptionRef}
              />
            );
            return needsPortalledFocusScope ? (
              <FocusScope asChild loop trapped>
                <div>{picker}</div>
              </FocusScope>
            ) : (
              picker
            );
          }}
        </AnchoredPopover>
      </div>
      <span
        id={hintId}
        className={inline ? "sr-only" : "mt-1 block text-xs text-gray-500"}
      >
        {hint}
      </span>
    </div>
  );
}
