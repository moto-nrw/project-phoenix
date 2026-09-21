"use client";

import { useState, useEffect, useCallback, useRef } from "react";
import { Pencil } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import {
  completesOneDigitHour,
  normalizeTimeInput,
} from "~/components/ui/time-field";

interface SettingsTimeFieldProps {
  readonly ariaLabel?: string;
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onBlur?: () => void;
  readonly disabled?: boolean;
  /** Label shown when value is empty (e.g., "Jederzeit"). If not set, empty values show the HH:MM placeholder. */
  readonly emptyLabel?: string;
}

const COMPLETE_TIME_RE = /^\d{2}:\d{2}$/;

function isValidTime(display: string): boolean {
  if (!COMPLETE_TIME_RE.test(display)) return false;
  const [h, m] = display.split(":").map(Number);
  return h !== undefined && m !== undefined && h <= 23 && m <= 59;
}

/**
 * Uhrzeitfeld der Einstellungen (Bauart 4). Es teilt sich Ziffernmaske und
 * Doppelpunkt mit dem Kit-`TimeField`, verhält sich aber anders: es meldet
 * nur vollständige, gültige Zeiten nach außen, setzt bei Blur auf den
 * letzten gespeicherten Wert zurück und zeigt eine leere Einstellung als
 * Pille („Jederzeit“). Das Kit-`TimeField` ist ein Formularfeld mit Label
 * und Formathinweis, das jeden Tastendruck durchreicht; darum heißt dieses
 * hier anders (#3117).
 */
export function SettingsTimeField({
  ariaLabel = "Einstellung",
  value,
  onChange,
  onBlur,
  disabled = false,
  emptyLabel,
}: SettingsTimeFieldProps) {
  const [display, setDisplay] = useState(value);
  const [isEditing, setIsEditing] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const completedOneDigitHour = useRef<string | null>(null);

  useEffect(() => {
    setDisplay(value);
    if (completedOneDigitHour.current !== value) {
      completedOneDigitHour.current = null;
    }
    if (value === "") {
      setIsEditing(false);
    }
  }, [value]);

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const formatted = normalizeTimeInput(
        e.target.value,
        display,
        completedOneDigitHour.current === display,
      );
      completedOneDigitHour.current = completesOneDigitHour(e.target.value)
        ? formatted
        : null;
      setDisplay(formatted);
      if (formatted === "" && emptyLabel) {
        onChange("");
        return;
      }
      if (isValidTime(formatted)) {
        onChange(formatted);
      }
    },
    [display, emptyLabel, onChange],
  );

  const handleBlur = useCallback(() => {
    if (!isValidTime(display)) {
      setDisplay(value);
      if (value === "") {
        setIsEditing(false);
      }
    }
    onBlur?.();
  }, [display, value, onBlur]);

  const handleStartEditing = useCallback(() => {
    setIsEditing(true);
    setTimeout(() => inputRef.current?.focus(), 0);
  }, []);

  // Empty value with emptyLabel, not editing — the pill is a kit button.
  if (!value && !isEditing && emptyLabel) {
    return (
      <Button
        aria-label={`${ariaLabel}: ${emptyLabel}`}
        type="button"
        variant="surface"
        size="md"
        onClick={handleStartEditing}
        disabled={disabled}
        className="gap-1.5"
      >
        {emptyLabel}
        {!disabled && <Pencil className="h-3.5 w-3.5" aria-hidden="true" />}
      </Button>
    );
  }

  // Has value or editing — show input
  return (
    <div className="w-24">
      <Input
        aria-label={ariaLabel}
        ref={inputRef}
        type="text"
        inputMode="numeric"
        placeholder="HH:MM"
        value={display}
        onChange={handleChange}
        onBlur={handleBlur}
        onKeyDown={(e) => {
          if (e.key === "Enter") {
            e.currentTarget.blur();
          }
        }}
        disabled={disabled}
        maxLength={6}
        controlSize="compact"
        className="text-center tabular-nums"
      />
    </div>
  );
}
