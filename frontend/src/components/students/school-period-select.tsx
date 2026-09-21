"use client";

import { CustomSelect } from "~/components/ui/custom-select";
import type { SchoolPeriod } from "~/lib/student-arrival-api";

/**
 * Picks an arrival time by lesson (#3372): „nach der 5. Stunde“ copies the end
 * time the school maintains for that lesson into the time field next to it.
 * Nothing references the lesson afterwards, so the choice shown is derived
 * from the time alone: a time typed by hand that matches no lesson shows the
 * placeholder again.
 *
 * Renders nothing while the school maintains no lesson end times.
 */
export function SchoolPeriodSelect({
  id,
  periods,
  time,
  ariaLabel,
  disabled = false,
  onSelect,
}: {
  readonly id: string;
  readonly periods: readonly SchoolPeriod[];
  /** The current value of the time field, "HH:MM" or empty. */
  readonly time: string;
  readonly ariaLabel: string;
  readonly disabled?: boolean;
  /** Receives the end time of the chosen lesson as "HH:MM". */
  readonly onSelect: (endTime: string) => void;
}) {
  if (periods.length === 0) return null;

  const selected = periods.find((period) => period.end_time === time);
  return (
    <CustomSelect
      id={id}
      value={selected ? String(selected.period) : ""}
      options={periods.map((period) => ({
        value: String(period.period),
        label: `${period.period}. Stunde (${period.end_time})`,
      }))}
      onChange={(value) => {
        const chosen = periods.find(
          (period) => String(period.period) === value,
        );
        if (chosen) onSelect(chosen.end_time);
      }}
      ariaLabel={ariaLabel}
      placeholder="Nach Schulstunde"
      disabled={disabled}
      triggerClassName="moto-content-surface h-8 w-full text-xs hover:border-gray-300"
    />
  );
}
