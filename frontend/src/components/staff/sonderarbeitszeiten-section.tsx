"use client";

import { Plus } from "lucide-react";
import { useState } from "react";
import { useSWRConfig } from "swr";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { useFormError } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useToast } from "~/contexts/ToastContext";
import { formatClosingDayRange } from "~/lib/closing-day-helpers";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  staffTargetOverrideService,
  type StaffTargetOverride,
} from "~/lib/staff-target-overrides-api";
import { useSWRAuth } from "~/lib/swr";

import {
  formatDecimalHours,
  isStaleAfterModelSave,
  parseDecimalHours,
} from "./arbeitszeit-hours";

const logger = createLogger({ component: "SonderarbeitszeitenSection" });

// Monday to Friday, the days a Sonderarbeitszeit sets a Soll on.
const WEEKDAYS = [
  { short: "Mo", name: "Montag" },
  { short: "Di", name: "Dienstag" },
  { short: "Mi", name: "Mittwoch" },
  { short: "Do", name: "Donnerstag" },
  { short: "Fr", name: "Freitag" },
] as const;

type HoursMode = "uniform" | "weekdays";

type Draft = {
  readonly startDate: string;
  readonly endDate: string;
  readonly mode: HoursMode;
  readonly hours: string;
  readonly weekdayHours: readonly string[];
};

type FieldErrors = {
  readonly startDate?: string;
  readonly endDate?: string;
  readonly hours?: string;
  readonly weekdayHours?: string;
};

const HOURS_ERROR = "Bitte 0 bis 12 Stunden eingeben.";

// A range reads like a closing day: "TT.MM.JJJJ – TT.MM.JJJJ", one date for a
// single day.
function formatRange(row: StaffTargetOverride): string {
  return formatClosingDayRange({
    id: row.id,
    startDate: row.startDate,
    endDate: row.endDate,
    reason: "",
  });
}

// The Soll of a row as one line: "8,5 Std." for the same hours every day,
// "Mo 3,5 Std. · Di–Fr je 0,5 Std." for hours per weekday (#3745). Weekdays in
// a row with the same hours are joined, so the usual pattern stays short.
function formatOverrideHours(row: StaffTargetOverride): string {
  if (!row.weekdayMinutes) {
    return `${formatDecimalHours(row.dailyMinutes ?? 0)} Std.`;
  }
  const minutes = row.weekdayMinutes;
  const parts: string[] = [];
  let start = 0;
  while (start < minutes.length) {
    let end = start;
    while (end + 1 < minutes.length && minutes[end + 1] === minutes[start]) {
      end++;
    }
    const hours = `${formatDecimalHours(minutes[start] ?? 0)} Std.`;
    parts.push(
      end === start
        ? `${WEEKDAYS[start]?.short} ${hours}`
        : `${WEEKDAYS[start]?.short}–${WEEKDAYS[end]?.short} je ${hours}`,
    );
    start = end + 1;
  }
  return parts.join(" · ");
}

function parseWeekdayMinutes(weekdayHours: readonly string[]): number[] | null {
  const minutes: number[] = [];
  for (const value of weekdayHours) {
    const parsed = parseDecimalHours(value);
    if (parsed.status !== "valid") return null;
    minutes.push(parsed.minutes);
  }
  return minutes;
}

type ValidatedHours =
  | { readonly dailyMinutes: number }
  | { readonly weekdayMinutes: readonly number[] };

function validateDraft(draft: Draft): {
  readonly fields: FieldErrors;
  readonly hours: ValidatedHours | null;
} {
  const parsed = parseDecimalHours(draft.hours);
  const weekdayMinutes = parseWeekdayMinutes(draft.weekdayHours);
  const uniform = draft.mode === "uniform";
  const fields: FieldErrors = {
    startDate: draft.startDate ? undefined : "Bitte den ersten Tag wählen.",
    endDate: !draft.endDate
      ? "Bitte den letzten Tag wählen."
      : draft.startDate && draft.endDate < draft.startDate
        ? "Der letzte Tag liegt vor dem ersten Tag."
        : undefined,
    hours: uniform && parsed.status !== "valid" ? HOURS_ERROR : undefined,
    weekdayHours:
      !uniform && weekdayMinutes === null
        ? "Bitte für jeden Tag 0 bis 12 Stunden eingeben."
        : undefined,
  };
  let hours: ValidatedHours | null = null;
  if (uniform && parsed.status === "valid") {
    hours = { dailyMinutes: parsed.minutes };
  } else if (!uniform && weekdayMinutes !== null) {
    hours = { weekdayMinutes };
  }
  return { fields, hours };
}

// The weekly sum of the weekday fields, or null while one is not a number.
function weeklyHours(weekdayHours: readonly string[]): string | null {
  const minutes = parseWeekdayMinutes(weekdayHours);
  if (minutes === null) return null;
  return formatDecimalHours(minutes.reduce((sum, value) => sum + value, 0));
}

// Sonderarbeitszeiten (#3259): a different daily Soll for a date range, for
// example holiday care inside the autumn closure. The Soll is the same every
// day or, for part-time staff, set per weekday (#3745). Listed on the
// Arbeitszeitmodell tab; managers add and delete them here. A wrong entry is
// deleted and entered again, there is no edit.
export function SonderarbeitszeitenSection({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const {
    data: overrides,
    error: loadError,
    isLoading,
    mutate: mutateList,
  } = useSWRAuth(`staff-target-overrides-${staffId}`, () =>
    staffTargetOverrideService.list(staffId),
  );
  const { mutate } = useSWRConfig();
  const toast = useToast();

  const [draft, setDraft] = useState<Draft | null>(null);
  const [fieldErrors, setFieldErrors] = useState<FieldErrors>({});
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useFormError();
  const [deleteTarget, setDeleteTarget] = useState<StaffTargetOverride | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  // A Sonderarbeitszeit changes the Soll of its days like a model change.
  const refresh = () => {
    void mutateList();
    void mutate(isStaleAfterModelSave);
  };

  const openCreate = () => {
    setFormError(null);
    setFieldErrors({});
    setDraft({
      startDate: "",
      endDate: "",
      mode: "uniform",
      hours: "",
      weekdayHours: WEEKDAYS.map(() => ""),
    });
  };

  // Switching to hours per weekday starts every day with the hours already
  // typed, so only the days that differ need a change. Switching back keeps
  // the hours when every day has the same valid value.
  const changeMode = (mode: HoursMode) => {
    if (!draft || mode === draft.mode) return;
    setFieldErrors({});
    if (mode === "weekdays") {
      const filled = draft.weekdayHours.every((value) => value.trim() === "");
      setDraft({
        ...draft,
        mode,
        weekdayHours: filled
          ? WEEKDAYS.map(() => draft.hours)
          : draft.weekdayHours,
      });
      return;
    }
    const weekdayMinutes = parseWeekdayMinutes(draft.weekdayHours);
    const [first, ...rest] = weekdayMinutes ?? [];
    const same =
      first !== undefined && rest.every((minutes) => minutes === first);
    setDraft({
      ...draft,
      mode,
      hours: same ? (draft.weekdayHours[0] ?? "") : draft.hours,
    });
  };

  const handleSave = async () => {
    if (!draft) return;
    const { fields, hours } = validateDraft(draft);
    setFieldErrors(fields);
    if (hours === null || fields.startDate || fields.endDate) {
      setFormError("Bitte die markierten Felder prüfen.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await staffTargetOverrideService.create(staffId, {
        startDate: draft.startDate,
        endDate: draft.endDate,
        ...hours,
      });
      toast.success("Sonderarbeitszeit angelegt.");
      setDraft(null);
      refresh();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Speichern hat nicht geklappt.";
      logger.error("target_override_save_failed", { error: message });
      setFormError(message);
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    setDeleteError("");
    try {
      await staffTargetOverrideService.delete(staffId, deleteTarget.id);
      toast.success("Sonderarbeitszeit gelöscht.");
      setDeleteTarget(null);
      refresh();
    } catch (err) {
      const message =
        err instanceof Error ? err.message : "Löschen hat nicht geklappt.";
      logger.error("target_override_delete_failed", { error: message });
      setDeleteError(message);
    } finally {
      setDeleting(false);
    }
  };

  const weekSum = draft ? weeklyHours(draft.weekdayHours) : null;

  const createButton = canEdit ? (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={openCreate}
      className="shrink-0 gap-2"
    >
      <Plus className="h-4 w-4" aria-hidden="true" />
      Sonderarbeitszeit anlegen
    </Button>
  ) : undefined;

  const columns: DataTableColumn<StaffTargetOverride>[] = [
    {
      key: "range",
      header: "Zeitraum",
      stacked: "title",
      render: formatRange,
      sortValue: (row) => row.startDate,
    },
    {
      key: "hours",
      header: "Stunden pro Tag",
      stacked: "meta",
      render: (row) => (
        <span className="tabular-nums">{formatOverrideHours(row)}</span>
      ),
    },
  ];
  if (canEdit) {
    columns.push({
      key: "actions",
      header: "",
      align: "right",
      render: (row) => {
        const items: readonly OverflowMenuEntry[] = [
          {
            label: "Löschen",
            destructive: true,
            onClick: () => {
              setDeleteError("");
              setDeleteTarget(row);
            },
          },
        ];
        return (
          <OverflowMenu
            items={items}
            ariaLabel={`Aktionen für die Sonderarbeitszeit ab ${formatDate(row.startDate)}`}
          />
        );
      },
    });
  }

  return (
    <SectionCard
      title="Sonderarbeitszeiten"
      headingLevel={3}
      description="Für einen Zeitraum gelten andere Stunden pro Tag, zum Beispiel in den Ferien. Das gilt Montag bis Freitag, auch an Schließtagen. Feiertage bleiben frei."
      action={createButton}
    >
      {loadError ? (
        <Alert
          type="error"
          message="Die Sonderarbeitszeiten konnten nicht geladen werden."
        />
      ) : (
        <DataTable
          columns={columns}
          rows={overrides ?? []}
          getRowKey={(row) => row.id}
          rowHasInteractiveControls={canEdit}
          isLoading={isLoading}
          loadingRowCount={1}
          defaultSortKey="range"
          emptyState={
            <EmptyState
              variant="compact"
              title="Keine Sonderarbeitszeiten eingetragen."
              description="Es gilt das Arbeitszeitmodell."
              action={createButton}
            />
          }
        />
      )}

      <FormModal
        isOpen={draft !== null}
        onClose={() => setDraft(null)}
        title="Sonderarbeitszeit anlegen"
        size="md"
        closeDisabled={saving}
        isBackdropDismissDisabled
        error={formError}
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setDraft(null)}
              disabled={saving}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => void handleSave()}
              disabled={saving}
            >
              {saving ? "Speichert…" : "Speichern"}
            </Button>
          </div>
        }
      >
        {draft && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <ISODatePicker
                id="target-override-start"
                label="Erster Tag"
                error={fieldErrors.startDate}
                controlSize="lg"
                value={draft.startDate}
                onChange={(value) => setDraft({ ...draft, startDate: value })}
                calendarLayout="popover"
              />
              <ISODatePicker
                id="target-override-end"
                label="Letzter Tag"
                error={fieldErrors.endDate}
                controlSize="lg"
                value={draft.endDate}
                min={draft.startDate || undefined}
                // An empty last day opens in the month of the first day.
                defaultMonth={draft.startDate || undefined}
                onChange={(value) => setDraft({ ...draft, endDate: value })}
                calendarLayout="popover"
              />
            </div>
            <SegmentedControl<HoursMode>
              ariaLabel="Wie die Stunden gelten"
              fullWidth
              items={[
                { value: "uniform", label: "Jeden Tag gleich" },
                { value: "weekdays", label: "Je Wochentag" },
              ]}
              value={draft.mode}
              onChange={changeMode}
            />
            {draft.mode === "uniform" ? (
              <div>
                <Input
                  id="target-override-hours"
                  label="Stunden pro Tag"
                  error={fieldErrors.hours}
                  // The kit Input marks only aria-invalid; the red ring matches
                  // the date fields, the way ISODateInput does it.
                  className={fieldErrors.hours ? "ring-moto-red" : ""}
                  type="text"
                  inputMode="decimal"
                  value={draft.hours}
                  onChange={(event) =>
                    setDraft({ ...draft, hours: event.target.value })
                  }
                  placeholder="z. B. 8,5"
                />
                <p className="mt-1 text-xs text-gray-500">
                  Bei 0 hat die Person an diesen Tagen frei.
                </p>
              </div>
            ) : (
              <fieldset>
                <legend className="mb-1.5 block text-sm font-medium text-gray-700">
                  Stunden je Wochentag
                </legend>
                <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
                  {WEEKDAYS.map((day, index) => {
                    const value = draft.weekdayHours[index] ?? "";
                    const invalid =
                      fieldErrors.weekdayHours !== undefined &&
                      parseDecimalHours(value).status !== "valid";
                    return (
                      <Input
                        key={day.short}
                        id={`target-override-hours-${index}`}
                        label={day.short}
                        aria-label={`Stunden am ${day.name}`}
                        aria-invalid={invalid || undefined}
                        className={`text-center tabular-nums ${invalid ? "ring-moto-red" : ""}`}
                        type="text"
                        inputMode="decimal"
                        autoComplete="off"
                        value={value}
                        onFocus={(event) => event.target.select()}
                        onChange={(event) =>
                          setDraft({
                            ...draft,
                            weekdayHours: draft.weekdayHours.map((value, i) =>
                              i === index ? event.target.value : value,
                            ),
                          })
                        }
                      />
                    );
                  })}
                </div>
                {fieldErrors.weekdayHours && (
                  <p role="alert" className="text-moto-red-strong mt-1 text-xs">
                    {fieldErrors.weekdayHours}
                  </p>
                )}
                <p className="mt-2 text-xs text-gray-500">
                  {weekSum === null
                    ? "Bei 0 hat die Person an diesem Tag frei."
                    : `Zusammen ${weekSum} Stunden pro Woche. Bei 0 hat die Person an diesem Tag frei.`}
                </p>
              </fieldset>
            )}
          </div>
        )}
      </FormModal>

      <ConfirmDeleteModal
        isOpen={deleteTarget !== null}
        title="Sonderarbeitszeit löschen"
        description={
          deleteTarget ? (
            <>
              Für{" "}
              <span className="font-medium">{formatRange(deleteTarget)}</span>{" "}
              gilt danach wieder das Arbeitszeitmodell.
            </>
          ) : null
        }
        gate={{ mode: "twoStep" }}
        onConfirm={handleDelete}
        onClose={() => setDeleteTarget(null)}
        loading={deleting}
        error={deleteError}
      />
    </SectionCard>
  );
}
