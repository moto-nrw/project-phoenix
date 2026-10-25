"use client";

import { useEffect, useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";

/** Reads `structured.day|month|year` and `months` of the `enrollmentForm` messages. */
export type DateOfBirthTranslator = ((key: string) => string) & {
  raw: (key: string) => unknown;
};

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string")
    ? value
    : [];
}

// DateOfBirthPicker uses three dropdowns (Tag / Monat / Jahr).
// Calendar widgets are awkward for DOB because parents need to jump
// 5-10 years back; native <input type="date"> auto-validates partial
// year input and snaps back. Three explicit dropdowns let parents pick fast
// without touching a keyboard. Wire format stays YYYY-MM-DD.
export function DateOfBirthPicker({
  value,
  onChange,
  error,
  tr,
}: {
  readonly value: string;
  readonly onChange: (v: string) => void;
  readonly error?: string;
  readonly tr: DateOfBirthTranslator;
}) {
  // Internal state so partial selections (only day picked, only month
  // picked, etc.) survive between renders. We sync from `value` when
  // it changes externally (e.g. "Übernehmen" prefill), then push the
  // full YYYY-MM-DD upward only when all three parts are filled.
  const [day, setDay] = useState("");
  const [month, setMonth] = useState("");
  const [year, setYear] = useState("");

  useEffect(() => {
    const parsed = parseDOBParts(value);
    if (parsed) {
      setDay(parsed.day);
      setMonth(parsed.month);
      setYear(parsed.year);
    } else if (value === "") {
      setDay("");
      setMonth("");
      setYear("");
    }
    // value is the only external driver; re-sync only on incoming changes
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value]);

  const currentYear = new Date().getFullYear();
  const monthLabels = asStringArray(tr.raw("months"));
  const years: number[] = [];
  for (let y = currentYear; y >= currentYear - 25; y--) years.push(y);

  const daysInMonth =
    month && year ? new Date(Number(year), Number(month), 0).getDate() : 31;

  const emit = (d: string, m: string, y: string) => {
    if (!d || !m || !y) {
      // Don't blank the parent on partial selections. Wait until all
      // three are picked. The parent's `value` may already be empty;
      // that's fine.
      onChange("");
      return;
    }
    const dd = d.padStart(2, "0");
    const mm = m.padStart(2, "0");
    onChange(`${y}-${mm}-${dd}`);
  };

  const handleDay = (v: string) => {
    setDay(v);
    emit(v, month, year);
  };
  const handleMonth = (v: string) => {
    setMonth(v);
    // If the new month has fewer days than the picked day, clamp.
    if (day && year) {
      const max = new Date(Number(year), Number(v), 0).getDate();
      if (Number(day) > max) {
        const clamped = String(max);
        setDay(clamped);
        emit(clamped, v, year);
        return;
      }
    }
    emit(day, v, year);
  };
  const handleYear = (v: string) => {
    setYear(v);
    emit(day, month, v);
  };

  return (
    <>
      <div className="mt-1 grid grid-cols-3 gap-2">
        <CustomSelect
          value={day}
          onChange={handleDay}
          ariaLabel={tr("structured.day")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.day") },
            ...Array.from({ length: daysInMonth }, (_, i) => {
              const dayValue = String(i + 1);
              return { value: dayValue, label: dayValue };
            }),
          ]}
        />
        <CustomSelect
          value={month}
          onChange={handleMonth}
          ariaLabel={tr("structured.month")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.month") },
            ...monthLabels.map((label, idx) => ({
              value: String(idx + 1),
              label,
            })),
          ]}
        />
        <CustomSelect
          value={year}
          onChange={handleYear}
          ariaLabel={tr("structured.year")}
          invalid={Boolean(error)}
          options={[
            { value: "", label: tr("structured.year") },
            ...years.map((yearValue) => ({
              value: String(yearValue),
              label: String(yearValue),
            })),
          ]}
        />
      </div>
      {error && <p className="text-moto-red mt-1 text-xs">{error}</p>}
    </>
  );
}

function parseDOBParts(
  value: string,
): { day: string; month: string; year: string } | null {
  if (!value) return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!m) return null;
  return {
    year: m[1] ?? "",
    month: String(Number(m[2])), // strip leading zero so the dropdown matches
    day: String(Number(m[3])),
  };
}
