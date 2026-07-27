"use client";

import { Checkbox } from "~/components/ui/checkbox";
import { Radio } from "~/components/ui/radio";

// Recurrence controls shared by the two places that define a shift series
// (#2028): creating one ("Als Serie wiederholen") and editing the whole rule
// ("Alle Termine der Serie"). Both must offer the identical weekday and
// rhythm semantics — one component instead of two drifting copies.
//
// The two blocks never render at the same time (create vs. edit mode), so the
// element ids stay unique despite the shared markup; idPrefix keeps them
// stable per usage anyway.

const WEEKDAY_OPTIONS: ReadonlyArray<{ value: number; label: string }> = [
  { value: 1, label: "Mo" },
  { value: 2, label: "Di" },
  { value: 3, label: "Mi" },
  { value: 4, label: "Do" },
  { value: 5, label: "Fr" },
  { value: 6, label: "Sa" },
  { value: 7, label: "So" },
];

export function WeekdayPicker({
  idPrefix,
  weekdays,
  onToggle,
}: {
  readonly idPrefix: string;
  readonly weekdays: readonly number[];
  readonly onToggle: (value: number) => void;
}) {
  return (
    <div className="flex flex-wrap gap-2">
      {WEEKDAY_OPTIONS.map((option) => (
        <label
          key={option.value}
          htmlFor={`${idPrefix}-weekday-${option.value}`}
          className="flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5"
        >
          <Checkbox
            id={`${idPrefix}-weekday-${option.value}`}
            checked={weekdays.includes(option.value)}
            onChange={() => onToggle(option.value)}
          />
          <span className="text-xs font-medium text-gray-700">
            {option.label}
          </span>
        </label>
      ))}
    </div>
  );
}

export function RhythmPicker({
  idPrefix,
  biweekly,
  onBiweeklyChange,
  abPattern,
  onAbPatternChange,
  periodHasCycle,
  weekHint,
}: {
  readonly idPrefix: string;
  readonly biweekly: boolean;
  readonly onBiweeklyChange: (next: boolean) => void;
  readonly abPattern: 1 | 2;
  /** Fires on every manual click, including re-selecting the already checked
   *  option — that gesture produces no change event but still means "I chose
   *  this", which stops the caller from auto-defaulting the pattern. */
  readonly onAbPatternChange: (next: 1 | 2) => void;
  readonly periodHasCycle: boolean;
  /** e.g. "Die Woche vom 07.09.2026 ist Woche A." — omitted when unknown. */
  readonly weekHint?: string;
}) {
  return (
    <div className="space-y-2">
      <div className="flex flex-wrap gap-2">
        <label
          htmlFor={`${idPrefix}-rhythm-weekly`}
          className="flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5"
        >
          <Radio
            id={`${idPrefix}-rhythm-weekly`}
            name={`${idPrefix}-rhythm`}
            value="weekly"
            checked={!biweekly}
            onChange={() => onBiweeklyChange(false)}
          />
          <span className="text-xs font-medium text-gray-700">Jede Woche</span>
        </label>
        <label
          htmlFor={`${idPrefix}-rhythm-biweekly`}
          className={
            periodHasCycle
              ? "flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5"
              : "flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5 opacity-50"
          }
        >
          <Radio
            id={`${idPrefix}-rhythm-biweekly`}
            name={`${idPrefix}-rhythm`}
            value="biweekly"
            checked={biweekly}
            onChange={() => onBiweeklyChange(true)}
            disabled={!periodHasCycle}
          />
          <span className="text-xs font-medium text-gray-700">
            Alle 2 Wochen
          </span>
        </label>
      </div>
      {!periodHasCycle && (
        <p className="text-xs text-gray-500">
          Woche A/B benötigt einen Kalenderzeitraum mit gepflegtem Wochenzyklus.
        </p>
      )}
      {biweekly && periodHasCycle && (
        <div className="space-y-1">
          <div className="flex flex-wrap gap-2">
            <label
              htmlFor={`${idPrefix}-ab-pattern-a`}
              className="flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5"
            >
              <Radio
                id={`${idPrefix}-ab-pattern-a`}
                name={`${idPrefix}-ab-pattern`}
                value="a"
                checked={abPattern === 1}
                onClick={() => onAbPatternChange(1)}
                onChange={() => onAbPatternChange(1)}
              />
              <span className="text-xs font-medium text-gray-700">Woche A</span>
            </label>
            <label
              htmlFor={`${idPrefix}-ab-pattern-b`}
              className="flex items-center gap-1.5 rounded-md border border-gray-200 px-2 py-1.5"
            >
              <Radio
                id={`${idPrefix}-ab-pattern-b`}
                name={`${idPrefix}-ab-pattern`}
                value="b"
                checked={abPattern === 2}
                onClick={() => onAbPatternChange(2)}
                onChange={() => onAbPatternChange(2)}
              />
              <span className="text-xs font-medium text-gray-700">Woche B</span>
            </label>
          </div>
          {weekHint && <p className="text-xs text-gray-500">{weekHint}</p>}
        </div>
      )}
    </div>
  );
}

/** "Mo, Mi, Fr" — the weekday set of a series in one line. */
export function formatWeekdays(weekdays: readonly number[]): string {
  return [...weekdays]
    .sort((a, b) => a - b)
    .map(
      (value) =>
        WEEKDAY_OPTIONS.find((option) => option.value === value)?.label ?? "",
    )
    .filter(Boolean)
    .join(", ");
}
