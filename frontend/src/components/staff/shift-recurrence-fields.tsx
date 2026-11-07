"use client";

import { SegmentedControl } from "~/components/ui/segmented-control";
import { WeekdayToggleGroup } from "~/components/ui/weekday-toggle-group";

// Recurrence controls shared by the two places that define a shift series
// (#2028): creating one ("Als Serie wiederholen") and editing the whole rule
// ("Alle Termine der Serie"). Both must offer the identical weekday and
// rhythm semantics — one component instead of two drifting copies.

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
  weekdays,
  onToggle,
}: {
  readonly weekdays: readonly number[];
  readonly onToggle: (value: number) => void;
}) {
  return (
    <WeekdayToggleGroup
      ariaLabel="Wochentage"
      days={WEEKDAY_OPTIONS}
      value={weekdays}
      onToggle={onToggle}
    />
  );
}

export function RhythmPicker({
  biweekly,
  onBiweeklyChange,
  abPattern,
  onAbPatternChange,
  periodHasCycle,
  weekHint,
}: {
  readonly biweekly: boolean;
  readonly onBiweeklyChange: (next: boolean) => void;
  readonly abPattern: 1 | 2;
  /** Fires on every manual click, including re-selecting the active option:
   *  that gesture still means "I chose this", which stops the caller from
   *  auto-defaulting the pattern. */
  readonly onAbPatternChange: (next: 1 | 2) => void;
  readonly periodHasCycle: boolean;
  /** e.g. "Die Woche vom 07.09.2026 ist Woche A." — omitted when unknown. */
  readonly weekHint?: string;
}) {
  return (
    <div className="space-y-2">
      <SegmentedControl
        ariaLabel="Wochenrhythmus"
        value={biweekly ? "biweekly" : "weekly"}
        onChange={(next) => onBiweeklyChange(next === "biweekly")}
        items={[
          { value: "weekly", label: "Jede Woche" },
          {
            value: "biweekly",
            label: "Alle 2 Wochen",
            disabled: !periodHasCycle,
          },
        ]}
      />
      {!periodHasCycle && (
        <p className="text-xs text-gray-500">
          Woche A/B benötigt einen Kalenderzeitraum mit gepflegtem Wochenzyklus.
        </p>
      )}
      {biweekly && periodHasCycle && (
        <div className="space-y-1">
          <SegmentedControl
            ariaLabel="A/B-Woche"
            value={abPattern === 1 ? "a" : "b"}
            onChange={(next) => onAbPatternChange(next === "a" ? 1 : 2)}
            items={[
              { value: "a", label: "Woche A" },
              { value: "b", label: "Woche B" },
            ]}
          />
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
