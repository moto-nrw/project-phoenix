// Der Editor eines Wochenmusters: Rotation, Woche, Stunden und Startzeit je
// Werktag. Der Reiter „Arbeitszeitmodell“ einer Person und die Katalogseite
// der Vorlagen (#6891) nutzen ihn beide; den Entwurf hält der Aufrufer.

import { useState } from "react";

import { CustomSelect } from "~/components/ui/custom-select";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { Input } from "~/components/ui/input";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { formatDuration } from "~/lib/time-tracking-helpers";

import {
  entryKey,
  rotationTotal,
  WORK_DAYS,
  weekLabel,
  weekTotal,
  withDecimalHours,
  withRotationLength,
  withStartTime,
  type PatternDraft,
} from "./work-time-pattern-draft";

export const dayLabels = ["Mo", "Di", "Mi", "Do", "Fr"] as const;
const dayNames = [
  "Montag",
  "Dienstag",
  "Mittwoch",
  "Donnerstag",
  "Freitag",
] as const;
const ROTATION_OPTIONS = [
  { value: 1, label: "Standard (1 Woche)" },
  { value: 2, label: "A/B-Wochen (2 Wochen)" },
  { value: 3, label: "3-Wochen-Rotation" },
  { value: 4, label: "4-Wochen-Rotation" },
] as const;

export function WorkTimePatternEditor({
  draft,
  onChange,
  idPrefix = "arbeitszeitmodell",
  children,
}: {
  readonly draft: PatternDraft;
  readonly onChange: (next: PatternDraft) => void;
  /** Hält die Feld-IDs eindeutig, wenn zwei Editoren auf einer Seite stehen. */
  readonly idPrefix?: string;
  /** Zusätzliche Felder unter dem Muster, z. B. „als Vorlage speichern“. */
  readonly children?: React.ReactNode;
}) {
  const [selectedWeek, setSelectedWeek] = useState(0);
  // Eine entfernte Rotationswoche kann nicht ausgewählt bleiben.
  const activeWeek = Math.min(selectedWeek, draft.rotationLength - 1);
  const total = rotationTotal(draft);

  return (
    <div className="space-y-4">
      <CustomSelect
        label="Rotation"
        id={`${idPrefix}-rotation-select`}
        value={String(draft.rotationLength)}
        onChange={(next) =>
          onChange(withRotationLength(draft, Number.parseInt(next, 10)))
        }
        options={ROTATION_OPTIONS.map((opt) => ({
          value: String(opt.value),
          label: opt.label,
        }))}
      />

      {draft.rotationLength > 1 && (
        <SegmentedControl
          ariaLabel="Woche der Rotation"
          fullWidth
          items={Array.from({ length: draft.rotationLength }, (_, w) => ({
            value: String(w),
            label: weekLabel(w),
          }))}
          value={String(activeWeek)}
          onChange={(next) => setSelectedWeek(Number(next))}
        />
      )}

      <p className="text-xs text-gray-500">
        Dezimalstunden mit Komma oder Punkt eingeben. Die Dauer wird auf die
        nächste volle Minute gerundet, halbe Minuten werden aufgerundet.
      </p>

      <div className="space-y-2">
        {WORK_DAYS.map((d) => {
          const entry = draft.entries.find(
            (e) => e.weekIndex === activeWeek && e.dayOfWeek === d,
          );
          const minutes = entry?.targetMinutes ?? 0;
          const startTime = entry?.startTime ?? "";
          const key = entryKey(activeWeek, d);
          const decimalHours = draft.hourInputs[key] ?? "";
          const isInvalid = draft.invalidKeys.has(key);
          const durationId = `${idPrefix}-duration-${activeWeek}-${d}`;
          const errorId = `${idPrefix}-error-${activeWeek}-${d}`;
          const startId = `${idPrefix}-start-${activeWeek}-${d}`;
          return (
            <div
              key={d}
              className="flex flex-wrap items-center gap-2 py-1 sm:gap-3"
            >
              <span className="w-10 shrink-0 text-sm font-medium text-gray-600">
                {dayLabels[d]}
              </span>
              <div className="w-28">
                <Input
                  aria-label={`Dezimalstunden ${dayNames[d]}`}
                  aria-describedby={
                    isInvalid ? `${durationId} ${errorId}` : durationId
                  }
                  aria-invalid={isInvalid || undefined}
                  type="text"
                  inputMode="decimal"
                  autoComplete="off"
                  controlSize="compact"
                  value={decimalHours}
                  onFocus={(e) => e.target.select()}
                  onChange={(e) =>
                    onChange(
                      withDecimalHours(draft, activeWeek, d, e.target.value),
                    )
                  }
                  placeholder="z. B. 4,45"
                  className="text-center tabular-nums"
                />
                <FieldErrorHint
                  id={errorId}
                  message={isInvalid ? "Bitte 0 bis 12 eingeben." : undefined}
                  live
                />
              </div>
              <span className="text-xs text-gray-400">Dezimalstd.</span>
              <div className="flex items-center gap-1 text-xs text-gray-500">
                <label htmlFor={startId}>Start</label>
                <div className="w-28">
                  <Input
                    id={startId}
                    type="time"
                    controlSize="compact"
                    value={startTime}
                    disabled={minutes <= 0}
                    onChange={(e) =>
                      onChange(
                        withStartTime(draft, activeWeek, d, e.target.value),
                      )
                    }
                    className="tabular-nums disabled:cursor-not-allowed"
                  />
                </div>
              </div>
              <span
                id={durationId}
                role="status"
                aria-live="polite"
                aria-atomic="true"
                className="ml-auto text-xs text-gray-500 tabular-nums"
              >
                {isInvalid
                  ? "Ungültiger Wert"
                  : `${formatDuration(minutes)} (${minutes} min)`}
              </span>
            </div>
          );
        })}
        <div className="flex items-center justify-between border-t border-gray-100 pt-2 text-xs">
          <span className="text-gray-500">
            Wochensoll {weekLabel(activeWeek)}
          </span>
          <span className="font-bold text-gray-700 tabular-nums">
            {formatDuration(weekTotal(draft, activeWeek))}
          </span>
        </div>
        {draft.rotationLength > 1 && (
          <div className="flex items-center justify-between text-xs">
            <span className="text-gray-500">
              Rotation ({draft.rotationLength} Wochen)
            </span>
            <span className="font-bold text-gray-700 tabular-nums">
              {formatDuration(total)} gesamt · Ø{" "}
              {formatDuration(Math.round(total / draft.rotationLength))} / Woche
            </span>
          </div>
        )}
      </div>

      {children}
    </div>
  );
}
