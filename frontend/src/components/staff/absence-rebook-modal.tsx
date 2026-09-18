"use client";

// „Art ändern" (#3258): die Leitung bucht vorhandene Einträge auf eine andere
// Art um, zum Beispiel vergangene Freitage von Freizeitausgleich auf
// Krank-Urlaubstage. Die Einträge bleiben stehen. Vor dem Speichern zeigt der
// Dialog, was sich am Stundenkonto und an den Kontingenten ändert; was nicht
// reicht oder gesperrt ist, lässt sich nicht speichern.

import { useEffect, useMemo, useState } from "react";

import {
  bookingOptions,
  QuotaSummary,
  TypeChoice,
  useYearAccounts,
  yearsBetween,
  type Projection,
} from "~/components/staff/absence-booking-modal";
import type { SickReportStaff } from "~/components/staff/sick-report-modal";
import { formatSignedDuration } from "~/components/staff/staff-time-views";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { Modal } from "~/components/ui/modal";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { useToast } from "~/contexts/ToastContext";
import {
  ABSENCE_TYPE_HEX,
  absenceRowLabel,
  formatAbsenceRange,
  formatDayCount,
} from "~/lib/absence-helpers";
import { absenceRequestFor, selectValueFor } from "~/lib/absence-type-select";
import type { AbsenceType } from "~/lib/absence-type-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import {
  AbsenceRebookingBlockedError,
  staffAbsenceService,
  type AbsenceRebooking,
  type StaffAbsenceRow,
} from "~/lib/staff-api";

const logger = createLogger({ component: "AbsenceRebookModal" });

/** Nur direkt eingetragene Einträge lassen sich umbuchen (#3258). */
export function isRebookableAbsence(row: StaffAbsenceRow): boolean {
  return row.status === "reported" && row.absence_type !== "sick";
}

function useRebookingPreview(args: {
  staffId: string;
  absenceIds: readonly number[];
  value: string;
}) {
  const { staffId, absenceIds, value } = args;
  const [preview, setPreview] = useState<AbsenceRebooking | null>(null);
  const [blocked, setBlocked] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const idsKey = absenceIds.join(",");

  useEffect(() => {
    setPreview(null);
    setBlocked(null);
    setFailed(false);
    if (!value || !idsKey) return;
    let stale = false;
    const request = absenceRequestFor(value);
    staffAbsenceService
      .rebookAbsences(staffId, {
        absenceIds: idsKey.split(","),
        absenceType: request.absence_type,
        absenceTypeId: request.absence_type_id,
        reason: "",
        dryRun: true,
      })
      .then((result) => {
        if (!stale) setPreview(result);
      })
      .catch((err: unknown) => {
        if (stale) return;
        if (err instanceof AbsenceRebookingBlockedError) {
          setBlocked(err.message);
          return;
        }
        logger.error("rebooking_preview_failed", {
          staff_id: staffId,
          error: err instanceof Error ? err.message : String(err),
        });
        setFailed(true);
      });
    return () => {
      stale = true;
    };
  }, [staffId, idsKey, value]);

  const loading = Boolean(value) && !preview && !blocked && !failed;
  return { preview, blocked, failed, loading };
}

function BalanceLine({ minutes }: { readonly minutes: number }) {
  if (minutes === 0) {
    return (
      <p className="text-sm text-gray-600">Das Stundenkonto bleibt gleich.</p>
    );
  }
  return (
    <DataGrid columns={1}>
      <DataField inline label="Änderung am Stundenkonto">
        <span
          className={`font-semibold tabular-nums ${minutes < 0 ? "text-moto-red-strong" : ""}`}
        >
          {formatSignedDuration(minutes)}
        </span>
      </DataField>
    </DataGrid>
  );
}

export function AbsenceRebookModal({
  staff,
  types,
  absences,
  onClose,
  onSaved,
}: {
  readonly staff: SickReportStaff;
  readonly types: readonly AbsenceType[];
  readonly absences: readonly StaffAbsenceRow[];
  readonly onClose: () => void;
  readonly onSaved: () => Promise<void>;
}) {
  const toast = useToast();
  // Eine Art, die einer der gewählten Einträge schon hat, würde die gesamte
  // atomare Umbuchung sperren. Deshalb steht sie nicht zur Wahl.
  const currentValues = useMemo(
    () =>
      new Set(
        absences.map((row) =>
          selectValueFor(row.absence_type, row.absence_type_id),
        ),
      ),
    [absences],
  );
  const options = useMemo(
    () =>
      bookingOptions(types).filter(
        (option) => !currentValues.has(option.value),
      ),
    [types, currentValues],
  );
  const [value, setValue] = useState("");
  const [reason, setReason] = useState("");
  const [reasonError, setReasonError] = useState<string | undefined>();
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useFormError();

  const absenceIds = useMemo(() => absences.map((row) => row.id), [absences]);
  const firstDay = useMemo(
    () =>
      absences.reduce(
        (first, row) => (row.date_start < first ? row.date_start : first),
        absences[0]?.date_start ?? "",
      ),
    [absences],
  );
  const accountFirstDays = useMemo(() => {
    const firstDays = new Map<number, string>();
    for (const absence of absences) {
      for (const year of yearsBetween(absence.date_start, absence.date_end)) {
        const firstDayInYear =
          absence.date_start > `${year}-01-01`
            ? absence.date_start
            : `${year}-01-01`;
        const current = firstDays.get(year);
        if (!current || firstDayInYear < current) {
          firstDays.set(year, firstDayInYear);
        }
      }
    }
    return firstDays;
  }, [absences]);
  const years = useMemo(
    () => [...accountFirstDays.keys()].sort((left, right) => left - right),
    [accountFirstDays],
  );
  const { accounts } = useYearAccounts(staff.id, types, years);
  const accountHints = useMemo(
    () =>
      years.map((year) => ({
        year,
        account: accounts.get(year),
        firstDay: accountFirstDays.get(year)!,
      })),
    [accountFirstDays, accounts, years],
  );
  const selected = options.find((option) => option.value === value);
  const { preview, blocked, failed, loading } = useRebookingPreview({
    staffId: staff.id,
    absenceIds,
    value,
  });

  useEffect(() => {
    setSaveError(null);
  }, [value, setSaveError]);

  const allowanceProjections: Projection[] = (preview?.allowances ?? [])
    .filter((item) => item.bookingDays > 0)
    .map((item) => ({
      year: item.year,
      remaining: item.remainingDays + item.bookingDays,
      needed: item.bookingDays,
    }));
  const vacationProjections: Projection[] = (preview?.vacation ?? [])
    .filter((item) => item.remainingBefore > item.remainingAfter)
    .map((item) => ({
      year: item.year,
      remaining: item.remainingBefore,
      needed: item.remainingBefore - item.remainingAfter,
    }));
  const freedVacation = (preview?.vacation ?? []).reduce(
    (sum, item) =>
      sum + Math.max(0, item.remainingAfter - item.remainingBefore),
    0,
  );
  const exceeded =
    (preview?.allowanceExceeded ?? false) ||
    (preview?.vacationExceeded ?? false);

  const disabled =
    saving || !selected || loading || failed || blocked !== null || exceeded;

  const save = async () => {
    if (disabled || !selected) return;
    if (reason.trim() === "") {
      setReasonError("Bitte kurz sagen, warum sich die Art ändert.");
      setSaveError("Bitte prüfen Sie die markierten Felder.");
      return;
    }
    setReasonError(undefined);
    setSaving(true);
    setSaveError(null);
    try {
      const request = absenceRequestFor(value);
      await staffAbsenceService.rebookAbsences(staff.id, {
        absenceIds,
        absenceType: request.absence_type,
        absenceTypeId: request.absence_type_id,
        reason: reason.trim(),
        dryRun: false,
      });
      toast.success(
        absences.length === 1
          ? `Eintrag ist jetzt ${selected.label}.`
          : `${absences.length} Einträge sind jetzt ${selected.label}.`,
      );
      await onSaved();
    } catch (error) {
      logger.error("rebooking_failed", {
        staff_id: staff.id,
        absence_count: absences.length,
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
      title={`Art ändern: ${staff.firstName} ${staff.lastName}`}
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
            loadingText="Wird geändert…"
          >
            Art ändern
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <FormErrorAlert message={saveError} />
        <p className="text-sm text-gray-600">
          Die Einträge bleiben stehen. Nur ihre Art ändert sich.
        </p>
        <section aria-label="Gewählte Einträge" className="space-y-2">
          <h4 className="text-sm font-medium text-gray-700">
            {absences.length === 1
              ? "1 Eintrag"
              : `${absences.length} Einträge`}
            {preview ? ` · ${formatDayCount(preview.days)}` : ""}
          </h4>
          <ul className="max-h-40 divide-y divide-gray-100 overflow-y-auto rounded-lg bg-gray-50 px-3">
            {absences.map((row) => (
              <li
                key={row.id}
                className="flex items-center justify-between gap-3 py-1.5 text-sm"
              >
                <span className="text-gray-800 tabular-nums">
                  {formatAbsenceRange(row.date_start, row.date_end)}
                </span>
                <StatusColorBadge
                  label={absenceRowLabel(row)}
                  color={
                    ABSENCE_TYPE_HEX[row.absence_type] ??
                    LOCATION_COLORS.UNKNOWN
                  }
                />
              </li>
            ))}
          </ul>
        </section>
        <TypeChoice
          legend="Neue Art"
          name="absence-rebook-type"
          options={options}
          value={value}
          account={years[0] ? accounts.get(years[0]) : undefined}
          firstDay={firstDay}
          accountHints={accountHints}
          onChange={setValue}
        />
        {loading ? (
          <p className="text-sm text-gray-500">Folgen werden berechnet…</p>
        ) : null}
        {failed ? (
          <Alert
            type="error"
            message="Die Folgen konnten nicht berechnet werden. Bitte schließen und noch einmal öffnen."
          />
        ) : null}
        {blocked ? <Alert type="error" message={blocked} /> : null}
        {preview && selected ? (
          <div className="space-y-3">
            <BalanceLine minutes={preview.balanceDeltaMinutes} />
            {allowanceProjections.length > 0 ? (
              <QuotaSummary
                label={selected.label}
                projections={allowanceProjections}
                neededLabel="Diese Änderung"
              />
            ) : null}
            {vacationProjections.length > 0 ? (
              <QuotaSummary
                label="Urlaub"
                projections={vacationProjections}
                neededLabel="Diese Änderung"
              />
            ) : null}
            {freedVacation > 0 ? (
              <p className="text-sm text-gray-600">
                {formatDayCount(freedVacation)} Urlaub werden wieder frei.
              </p>
            ) : null}
          </div>
        ) : null}
        {preview?.allowanceExceeded && selected ? (
          <Alert
            type="error"
            message={`Nicht genug Tage: ${selected.label} reicht für diese Einträge nicht. Erhöhen Sie zuerst den Anspruch oder wählen Sie weniger Einträge.`}
          />
        ) : null}
        {preview?.vacationExceeded ? (
          <Alert
            type="error"
            message="Nicht genug Urlaub: Der Resturlaub reicht für diese Einträge nicht. Erhöhen Sie zuerst den Urlaubsanspruch."
          />
        ) : null}
        <Input
          name="absence-rebook-reason"
          label="Grund (Pflicht)"
          value={reason}
          onChange={(event) => {
            setReason(event.target.value);
            if (reasonError) setReasonError(undefined);
          }}
          placeholder="z. B. Kontingent jetzt angelegt"
          error={reasonError}
          disabled={saving}
        />
        <p className="text-xs text-gray-500">
          Der Grund steht danach im Änderungsprotokoll.
        </p>
      </div>
    </Modal>
  );
}
