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

type Draft = {
  readonly startDate: string;
  readonly endDate: string;
  readonly hours: string;
};

type FieldErrors = {
  readonly startDate?: string;
  readonly endDate?: string;
  readonly hours?: string;
};

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

function validateDraft(draft: Draft): {
  readonly fields: FieldErrors;
  readonly minutes: number | null;
} {
  const parsed = parseDecimalHours(draft.hours);
  const fields: FieldErrors = {
    startDate: draft.startDate ? undefined : "Bitte den ersten Tag wählen.",
    endDate: !draft.endDate
      ? "Bitte den letzten Tag wählen."
      : draft.startDate && draft.endDate < draft.startDate
        ? "Der letzte Tag liegt vor dem ersten Tag."
        : undefined,
    hours:
      parsed.status === "valid"
        ? undefined
        : "Bitte 0 bis 12 Stunden eingeben, zum Beispiel 8,5.",
  };
  return {
    fields,
    minutes: parsed.status === "valid" ? parsed.minutes : null,
  };
}

// Sonderarbeitszeiten (#3259): a different daily Soll for a date range, for
// example holiday care inside the autumn closure. Listed on the
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
    setDraft({ startDate: "", endDate: "", hours: "" });
  };

  const handleSave = async () => {
    if (!draft) return;
    const { fields, minutes } = validateDraft(draft);
    setFieldErrors(fields);
    if (minutes === null || fields.startDate || fields.endDate) {
      setFormError("Bitte die markierten Felder prüfen.");
      return;
    }
    setSaving(true);
    setFormError(null);
    try {
      await staffTargetOverrideService.create(staffId, {
        startDate: draft.startDate,
        endDate: draft.endDate,
        dailyMinutes: minutes,
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
        <span className="tabular-nums">
          {formatDecimalHours(row.dailyMinutes)} Std.
        </span>
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
      description="Für einen Zeitraum gelten andere Stunden pro Tag, zum Beispiel für die Ferienbetreuung. Sie gelten Montag bis Freitag, auch an Schließtagen. Feiertage bleiben frei. Danach gilt wieder das Arbeitszeitmodell."
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
              description={
                canEdit
                  ? "Arbeitet die Person in den Ferien andere Stunden? Legen Sie dafür eine Sonderarbeitszeit an."
                  : "Es gilt immer das Arbeitszeitmodell."
              }
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
                0 bedeutet: an diesen Tagen muss nicht gearbeitet werden.
              </p>
            </div>
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
