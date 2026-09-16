"use client";

// „Abwesenheit eintragen" (#3256): die Leitung trägt für eine Mitarbeiterin
// direkt ein, ohne Antrag, und wählt dabei, von welchem Kontingent der Tag
// abgeht. Jede Art zeigt, wie viele Tage noch übrig sind; was nicht mehr
// reicht, lässt sich nicht eintragen. Nur das Stundenkonto darf ins Minus
// (Freizeitausgleich, mit Bestätigung).
//
// Krank melden bleibt ein eigener Dialog, weil er Dienst- und Betreuungsplan
// ändert.

import { useEffect, useMemo, useState } from "react";

import {
  CompTimePreviewPanel,
  useCompTimePreview,
} from "~/components/staff/comp-time-preview";
import type { SickReportStaff } from "~/components/staff/sick-report-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { Radio } from "~/components/ui/radio";
import { useToast } from "~/contexts/ToastContext";
import { formatDayCount } from "~/lib/absence-helpers";
import {
  absenceRequestFor,
  customIdFromOptionValue,
  customOptionValue,
} from "~/lib/absence-type-select";
import {
  absenceTypeService,
  type AbsenceType,
  type AbsenceTypeAllowanceSummary,
} from "~/lib/absence-type-api";
import { todayISO, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { staffAbsenceService, type StaffAbsenceRow } from "~/lib/staff-api";

const logger = createLogger({ component: "AbsenceBookingModal" });

const COUNTED_STATUSES = new Set([
  "reported",
  "approved",
  "requested",
  "question",
]);

/** Everything one calendar year holds for the booking decision. */
interface YearAccount {
  readonly vacationRemaining: number;
  readonly allowances: ReadonlyMap<string, AbsenceTypeAllowanceSummary>;
  readonly absences: readonly StaffAbsenceRow[];
}

type OptionKind = "quota" | "comp_time" | "plain";

interface BookingOption {
  readonly value: string;
  readonly label: string;
  readonly kind: OptionKind;
}

function bookingOptions(types: readonly AbsenceType[]): BookingOption[] {
  const active = types.filter((type) => type.isActive);
  return [
    { value: "vacation", label: "Urlaub", kind: "quota" },
    ...active
      .filter((type) => type.allowanceEnabled)
      .map((type) => ({
        value: customOptionValue(type.id),
        label: type.name,
        kind: "quota" as const,
      })),
    { value: "comp_time", label: "Freizeitausgleich", kind: "comp_time" },
    { value: "training", label: "Fortbildung", kind: "plain" },
    { value: "other", label: "Sonstige", kind: "plain" },
    ...active
      .filter((type) => !type.allowanceEnabled)
      .map((type) => ({
        value: customOptionValue(type.id),
        label: type.name,
        kind: "plain" as const,
      })),
  ];
}

function isWeekday(day: Date): boolean {
  return day.getUTCDay() !== 0 && day.getUTCDay() !== 6;
}

function eachDay(
  from: string,
  to: string,
  visit: (iso: string, day: Date) => void,
) {
  const start = new Date(`${from}T00:00:00Z`);
  const count = Math.round(
    (new Date(`${to}T00:00:00Z`).getTime() - start.getTime()) / 86_400_000,
  );
  for (let offset = 0; offset <= count; offset += 1) {
    const day = new Date(start);
    day.setUTCDate(start.getUTCDate() + offset);
    visit(toISODate(day), day);
  }
}

function coverageOn(row: StaffAbsenceRow, date: string): number {
  if (date < row.date_start || date > row.date_end) return 0;
  if (row.date_start === row.date_end) return row.half_day ? 0.5 : 1;
  if (date === row.date_start && row.start_half_day) return 0.5;
  if (date === row.date_end && row.end_half_day) return 0.5;
  return 1;
}

/**
 * Days the booking spends from `value` in `year`. Mirrors the server: Monday
 * to Friday count, a half day counts 0.5, and an own art only charges days
 * that are not already booked under the same art.
 */
function daysNeeded(args: {
  value: string;
  year: number;
  dateStart: string;
  dateEnd: string;
  halfDay: boolean;
  absences: readonly StaffAbsenceRow[];
}): number {
  const from =
    args.dateStart > `${args.year}-01-01`
      ? args.dateStart
      : `${args.year}-01-01`;
  const to =
    args.dateEnd < `${args.year}-12-31` ? args.dateEnd : `${args.year}-12-31`;
  if (to < from) return 0;
  const unit = args.halfDay ? 0.5 : 1;
  const customId = args.value.startsWith("custom:")
    ? customIdFromOptionValue(args.value)
    : null;
  let days = 0;
  eachDay(from, to, (iso, day) => {
    if (!isWeekday(day)) return;
    if (customId === null) {
      days += unit;
      return;
    }
    const booked = Math.max(
      0,
      ...args.absences
        .filter(
          (row) =>
            row.absence_type_id === customId &&
            COUNTED_STATUSES.has(row.status),
        )
        .map((row) => coverageOn(row, iso)),
    );
    days += Math.max(0, unit - booked);
  });
  return days;
}

function remainingFor(value: string, account: YearAccount): number {
  if (value === "vacation") return account.vacationRemaining;
  return (
    account.allowances.get(customIdFromOptionValue(value))?.remainingDays ?? 0
  );
}

async function loadYear(
  staffId: string,
  year: number,
  quotaTypes: readonly AbsenceType[],
): Promise<YearAccount> {
  const [vacation, absences, summaries] = await Promise.all([
    staffAbsenceService.getVacationQuota(staffId, year),
    staffAbsenceService.getAbsences(staffId, `${year}-01-01`, `${year}-12-31`),
    Promise.all(
      quotaTypes.map((type) =>
        absenceTypeService.getAllowance(type.id, staffId, year),
      ),
    ),
  ]);
  return {
    vacationRemaining: vacation.remaining_days,
    allowances: new Map(
      quotaTypes.map((type, index) => [type.id, summaries[index]!]),
    ),
    absences,
  };
}

function yearsBetween(dateStart: string, dateEnd: string): number[] {
  const first = Number(dateStart.slice(0, 4));
  const last = Number((dateEnd || dateStart).slice(0, 4));
  if (!Number.isFinite(first) || !Number.isFinite(last) || last < first)
    return [];
  return Array.from({ length: last - first + 1 }, (_, index) => first + index);
}

function useYearAccounts(
  staffId: string,
  types: readonly AbsenceType[],
  years: readonly number[],
) {
  const [accounts, setAccounts] = useState<ReadonlyMap<number, YearAccount>>(
    new Map(),
  );
  const [failed, setFailed] = useState(false);
  const quotaTypes = useMemo(
    () => types.filter((type) => type.isActive && type.allowanceEnabled),
    [types],
  );
  const missing = years.filter((year) => !accounts.has(year));
  const missingKey = missing.join(",");

  useEffect(() => {
    if (!missingKey) return;
    let stale = false;
    Promise.all(
      missingKey.split(",").map(async (year) => {
        const value = Number(year);
        return [value, await loadYear(staffId, value, quotaTypes)] as const;
      }),
    )
      .then((loaded) => {
        if (stale) return;
        setAccounts((current) => new Map([...current, ...loaded]));
      })
      .catch((err: unknown) => {
        if (stale) return;
        logger.error("booking_accounts_load_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        setFailed(true);
      });
    return () => {
      stale = true;
    };
  }, [missingKey, quotaTypes, staffId]);

  return { accounts, loading: missing.length > 0 && !failed, failed };
}

interface Projection {
  readonly year: number;
  readonly remaining: number;
  readonly needed: number;
}

function OptionHint({
  option,
  account,
}: {
  option: BookingOption;
  account: YearAccount | undefined;
}) {
  if (option.kind === "comp_time") {
    return (
      <span className="shrink-0 text-xs whitespace-nowrap text-gray-500">
        vom Stundenkonto
      </span>
    );
  }
  if (option.kind === "plain") {
    return (
      <span className="shrink-0 text-xs whitespace-nowrap text-gray-500">
        ohne Kontingent
      </span>
    );
  }
  if (!account) {
    return <span className="text-xs text-gray-400">…</span>;
  }
  const remaining = remainingFor(option.value, account);
  return (
    <span
      className={`shrink-0 text-xs whitespace-nowrap tabular-nums ${remaining <= 0 ? "text-moto-red-strong" : "text-gray-600"}`}
    >
      noch {formatDayCount(remaining)}
    </span>
  );
}

function TypeChoice({
  options,
  value,
  account,
  onChange,
}: {
  options: readonly BookingOption[];
  value: string;
  account: YearAccount | undefined;
  onChange: (value: string) => void;
}) {
  return (
    <fieldset>
      <legend className="mb-2 text-sm font-medium text-gray-700">
        Von welchem Kontingent?
      </legend>
      <div className="grid grid-cols-1 gap-2">
        {options.map((option) => (
          <ChoiceTile
            key={option.value}
            selected={value === option.value}
            className="justify-between"
          >
            <span className="flex min-w-0 items-center gap-3">
              <Radio
                name="absence-booking-type"
                value={option.value}
                checked={value === option.value}
                onChange={() => onChange(option.value)}
              />
              <span className="truncate">{option.label}</span>
            </span>
            <OptionHint option={option} account={account} />
          </ChoiceTile>
        ))}
      </div>
    </fieldset>
  );
}

function QuotaSummary({
  label,
  projections,
}: {
  label: string;
  projections: readonly Projection[];
}) {
  const multiYear = projections.length > 1;
  return (
    <div className="space-y-2 rounded-lg bg-gray-50 p-3">
      {projections.map((item) => (
        <DataGrid key={item.year} columns={1}>
          <DataField
            inline
            label={
              multiYear ? `${label} ${item.year}: noch übrig` : "Noch übrig"
            }
          >
            <span className="tabular-nums">
              {formatDayCount(item.remaining)}
            </span>
          </DataField>
          <DataField inline label="Diese Eintragung">
            <span className="tabular-nums">{formatDayCount(item.needed)}</span>
          </DataField>
          <DataField inline label="Danach übrig">
            <span
              className={`font-semibold tabular-nums ${
                item.remaining - item.needed < 0 ? "text-moto-red-strong" : ""
              }`}
            >
              {formatDayCount(item.remaining - item.needed)}
            </span>
          </DataField>
        </DataGrid>
      ))}
    </div>
  );
}

function effectLine(kind: OptionKind): string {
  return kind === "comp_time"
    ? "Freizeitausgleich zieht das Tagessoll vom Stundenkonto ab."
    : "Das Tagessoll wird gutgeschrieben. Die Überstunden bleiben gleich.";
}

export function AbsenceBookingModal({
  staff,
  types,
  onClose,
  onSaved,
}: {
  readonly staff: SickReportStaff;
  readonly types: readonly AbsenceType[];
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void>;
}) {
  const toast = useToast();
  const options = useMemo(() => bookingOptions(types), [types]);
  const [value, setValue] = useState("");
  const [dateStart, setDateStart] = useState(todayISO());
  const [dateEnd, setDateEnd] = useState(todayISO());
  const [halfDay, setHalfDay] = useState(false);
  const [note, setNote] = useState("");
  const [overdraftConfirmed, setOverdraftConfirmed] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useFormError();

  const effectiveEnd = halfDay ? dateStart : dateEnd;
  const years = useMemo(
    () => yearsBetween(dateStart, effectiveEnd),
    [dateStart, effectiveEnd],
  );
  const { accounts, loading, failed } = useYearAccounts(staff.id, types, years);
  const selected = options.find((option) => option.value === value);
  const isCompTime = selected?.kind === "comp_time";
  const { preview, loading: previewLoading } = useCompTimePreview({
    enabled: isCompTime,
    staffId: staff.id,
    dateStart,
    dateEnd: effectiveEnd,
    halfDay,
  });

  // Eine geänderte Eingabe verwirft Bestätigung und alte Fehlermeldung.
  useEffect(() => {
    setOverdraftConfirmed(false);
    setSaveError(null);
  }, [value, dateStart, effectiveEnd, halfDay, setSaveError]);

  const projections: Projection[] =
    selected?.kind === "quota"
      ? years.flatMap((year) => {
          const account = accounts.get(year);
          if (!account) return [];
          const needed = daysNeeded({
            value,
            year,
            dateStart,
            dateEnd: effectiveEnd,
            halfDay,
            absences: account.absences,
          });
          return needed > 0 || years.length === 1
            ? [{ year, remaining: remainingFor(value, account), needed }]
            : [];
        })
      : [];
  const shortfall = projections.find(
    (item) => item.remaining - item.needed < 0,
  );
  const isOverdraft =
    isCompTime && preview !== null && preview.projectedBalanceMinutes < 0;
  const firstAccount = years.length > 0 ? accounts.get(years[0]!) : undefined;

  const disabled =
    saving ||
    !selected ||
    !dateStart ||
    !effectiveEnd ||
    effectiveEnd < dateStart ||
    (selected.kind === "quota" &&
      (loading || failed || shortfall !== undefined)) ||
    (isCompTime && (previewLoading || (isOverdraft && !overdraftConfirmed)));

  const save = async () => {
    if (disabled || !selected) return;
    setSaving(true);
    setSaveError(null);
    try {
      const request = absenceRequestFor(value);
      await staffAbsenceService.createAbsence(staff.id, {
        absence_type: request.absence_type,
        ...(request.absence_type_id
          ? { absence_type_id: request.absence_type_id }
          : {}),
        date_start: dateStart,
        date_end: effectiveEnd,
        half_day: halfDay || undefined,
        note: note.trim() || undefined,
      });
      toast.success(`${selected.label} eingetragen.`);
      await onSaved();
    } catch (error) {
      logger.error("absence_booking_failed", {
        staff_id: staff.id,
        absence_type: value,
        error: error instanceof Error ? error.message : String(error),
      });
      setSaveError(
        error instanceof Error && error.message
          ? error.message
          : "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      isOpen
      onClose={() => !saving && onClose()}
      title={`Abwesenheit eintragen: ${staff.firstName} ${staff.lastName}`}
      footer={
        <div className="flex w-full justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={saving}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void save()}
            disabled={disabled}
            isLoading={saving}
            loadingText="Wird eingetragen…"
          >
            Eintragen
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <FormErrorAlert message={saveError} />
        <p className="text-sm text-gray-600">
          Wird sofort eingetragen. Es entsteht kein Antrag.
        </p>
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <DateField
            id="absence-booking-start"
            label={halfDay ? "Datum" : "Von"}
            value={dateStart}
            onChange={(next) => {
              setDateStart(next);
              if (next > dateEnd) setDateEnd(next);
            }}
          />
          {!halfDay && (
            <DateField
              id="absence-booking-end"
              label="Bis"
              value={dateEnd}
              min={dateStart}
              onChange={setDateEnd}
            />
          )}
        </div>
        <label
          htmlFor="absence-booking-half"
          className="flex items-center gap-2 text-sm font-medium text-gray-700"
        >
          <Checkbox
            id="absence-booking-half"
            checked={halfDay}
            onChange={(event) => setHalfDay(event.target.checked)}
          />
          Halber Tag
        </label>
        <TypeChoice
          options={options}
          value={value}
          account={firstAccount}
          onChange={setValue}
        />
        {failed ? (
          <Alert
            type="error"
            message="Die Kontingente konnten nicht geladen werden. Bitte schließen und noch einmal öffnen."
          />
        ) : null}
        {selected ? (
          <p className="text-sm text-gray-600">{effectLine(selected.kind)}</p>
        ) : null}
        {selected?.kind === "quota" && projections.length > 0 ? (
          <QuotaSummary label={selected.label} projections={projections} />
        ) : null}
        {shortfall && selected ? (
          <Alert
            type="error"
            message={`Nicht genug Tage: ${selected.label} hat noch ${formatDayCount(shortfall.remaining)}, diese Eintragung braucht ${formatDayCount(shortfall.needed)}. Ändern Sie zuerst den Anspruch oder wählen Sie eine andere Art.`}
          />
        ) : null}
        {isCompTime ? (
          <CompTimePreviewPanel preview={preview} loading={previewLoading} />
        ) : null}
        {isOverdraft ? (
          <div className="space-y-3">
            <Alert
              type="warning"
              message={`Das Stundenkonto von ${staff.firstName} ${staff.lastName} fällt damit unter null.`}
            />
            <label
              htmlFor="comp-time-overdraft-confirm"
              className="flex items-center gap-2"
            >
              <Checkbox
                id="comp-time-overdraft-confirm"
                checked={overdraftConfirmed}
                onChange={(event) =>
                  setOverdraftConfirmed(event.target.checked)
                }
              />
              <span className="text-sm font-medium text-gray-800">
                Ich trage den Freizeitausgleich trotzdem ein.
              </span>
            </label>
          </div>
        ) : null}
        <Input
          name="absence-booking-note"
          label="Notiz (optional)"
          value={note}
          onChange={(event) => setNote(event.target.value)}
          placeholder="z. B. mündlich abgesprochen"
        />
      </div>
    </Modal>
  );
}

function DateField({
  id,
  label,
  value,
  min,
  onChange,
}: {
  id: string;
  label: string;
  value: string;
  min?: string;
  onChange: (value: string) => void;
}) {
  return (
    <div>
      <label
        htmlFor={id}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <ISODatePicker
        id={id}
        value={value}
        min={min}
        onChange={onChange}
        calendarLayout="popover"
        hideClearButton
      />
    </div>
  );
}
