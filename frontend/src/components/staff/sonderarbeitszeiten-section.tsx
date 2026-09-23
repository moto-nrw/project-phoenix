"use client";

import { useState } from "react";
import { useSWRConfig } from "swr";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
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

// A Sonderarbeitszeit changes the Soll of its days exactly like a model
// change, so the same caches go stale.
function isStaleAfterOverrideChange(key: unknown): boolean {
  return isStaleAfterModelSave(key);
}

type Draft = {
  readonly startDate: string;
  readonly endDate: string;
  readonly hours: string;
};

// Sonderarbeitszeiten (#3259): a different daily Soll for a date range, for
// example holiday care inside the autumn closure. Listed below the model on
// the Arbeitszeitmodell tab; managers add and delete them here. A wrong
// entry is deleted and entered again, there is no edit (#3259).
export function SonderarbeitszeitenSection({
  staffId,
  canEdit,
}: {
  readonly staffId: string;
  readonly canEdit: boolean;
}) {
  const listKey = `staff-target-overrides-${staffId}`;
  const {
    data: overrides,
    error: loadError,
    isLoading,
    mutate: mutateList,
  } = useSWRAuth(listKey, () => staffTargetOverrideService.list(staffId));
  const { mutate } = useSWRConfig();

  const [draft, setDraft] = useState<Draft | null>(null);
  const [saving, setSaving] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<StaffTargetOverride | null>(
    null,
  );
  const [deleting, setDeleting] = useState(false);
  const [deleteError, setDeleteError] = useState("");

  const refresh = () => {
    void mutateList();
    void mutate(isStaleAfterOverrideChange);
  };

  const openCreate = () => {
    setFormError(null);
    setDraft({ startDate: "", endDate: "", hours: "" });
  };

  const handleSave = async () => {
    if (!draft) return;
    if (!draft.startDate || !draft.endDate) {
      setFormError("Bitte wählen Sie den ersten und den letzten Tag.");
      return;
    }
    if (draft.endDate < draft.startDate) {
      setFormError("Der letzte Tag liegt vor dem ersten Tag.");
      return;
    }
    const parsed = parseDecimalHours(draft.hours);
    if (parsed.status !== "valid") {
      setFormError(
        "Bitte geben Sie die Stunden pro Tag ein, zum Beispiel 8,5. Erlaubt sind 0 bis 12.",
      );
      return;
    }
    const input = {
      startDate: draft.startDate,
      endDate: draft.endDate,
      dailyMinutes: parsed.minutes,
    };
    setSaving(true);
    setFormError(null);
    try {
      await staffTargetOverrideService.create(staffId, input);
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

  const columns: DataTableColumn<StaffTargetOverride>[] = [
    {
      key: "range",
      header: "Zeitraum",
      stacked: "title",
      render: (row) =>
        row.startDate === row.endDate
          ? formatDate(row.startDate)
          : `${formatDate(row.startDate)} bis ${formatDate(row.endDate)}`,
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
      action={
        canEdit ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={openCreate}
          >
            Anlegen
          </Button>
        ) : undefined
      }
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
          caption="Sonderarbeitszeiten"
          emptyState={
            <EmptyState
              variant="compact"
              title="Keine Sonderarbeitszeiten eingetragen."
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
              {saving ? "Speichern..." : "Speichern"}
            </Button>
          </div>
        }
      >
        {draft && (
          <div className="space-y-4">
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label
                  htmlFor="target-override-start"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Erster Tag
                </label>
                <ISODatePicker
                  id="target-override-start"
                  controlSize="lg"
                  value={draft.startDate}
                  onChange={(value) => setDraft({ ...draft, startDate: value })}
                  calendarLayout="popover"
                />
              </div>
              <div>
                <label
                  htmlFor="target-override-end"
                  className="mb-1 block text-sm font-medium text-gray-700"
                >
                  Letzter Tag
                </label>
                <ISODatePicker
                  id="target-override-end"
                  controlSize="lg"
                  value={draft.endDate}
                  min={draft.startDate || undefined}
                  onChange={(value) => setDraft({ ...draft, endDate: value })}
                  calendarLayout="popover"
                />
              </div>
            </div>
            <div>
              <label
                htmlFor="target-override-hours"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Stunden pro Tag
              </label>
              <Input
                id="target-override-hours"
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
              <span className="font-medium">
                {formatDate(deleteTarget.startDate)} bis{" "}
                {formatDate(deleteTarget.endDate)}
              </span>{" "}
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
