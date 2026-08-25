"use client";

import { useCallback, useMemo, useState } from "react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import useSWR from "swr";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { isOperatorApiError } from "~/lib/operator/api-helpers";
import {
  centsToEuroInput,
  createSchoolInvoice,
  deleteSchoolInvoice,
  formatCents,
  invoiceStatusLabel,
  invoiceStatusTone,
  listSchoolInvoices,
  parseEuroToCents,
  updateSchoolInvoice,
  type InvoiceStatus,
  type SchoolInvoice,
  type SchoolInvoiceInput,
} from "~/lib/operator/school-invoices-api";

// Zahlungsplan einer Schule (#1459 Demo) — Operator-Oberfläche.
//
// Die Schule sieht dieselben Zeilen unter /vertrag, aber nur lesend. Hier
// entscheidet ein Mensch bei moto, ob eine Zahlung eingegangen ist; einen
// Zahlungsdienstleister gibt es bewusst nicht.

const STATUS_OPTIONS: readonly { value: InvoiceStatus; label: string }[] = [
  { value: "offen", label: "Offen" },
  { value: "bezahlt", label: "Bezahlt" },
  { value: "storniert", label: "Storniert" },
];

/** Leeres Formular für eine neue Rechnung. */
function emptyForm(): SchoolInvoiceInput & { amountInput: string } {
  return {
    periodLabel: "",
    invoiceNumber: "",
    amountCents: 0,
    amountInput: "",
    dueDate: "",
    status: "offen",
    paidOn: null,
    note: "",
  };
}

function formFromInvoice(
  invoice: SchoolInvoice,
): SchoolInvoiceInput & { amountInput: string } {
  return {
    periodLabel: invoice.periodLabel,
    invoiceNumber: invoice.invoiceNumber,
    amountCents: invoice.amountCents,
    amountInput: centsToEuroInput(invoice.amountCents),
    dueDate: invoice.dueDate,
    status: invoice.status,
    paidOn: invoice.paidOn,
    note: invoice.note,
  };
}

/** Übersetzt einen Fehler in einen Satz, den ein Operator lesen kann. */
function errorMessage(error: unknown, fallback: string): string {
  if (isOperatorApiError(error)) return error.message;
  if (error instanceof Error && error.message !== "") return error.message;
  return fallback;
}

export function SchoolInvoicesPanel({
  schoolId,
}: Readonly<{ schoolId: string }>) {
  const {
    data: invoices,
    error,
    isLoading,
    mutate,
  } = useSWR(["school-invoices", schoolId], () => listSchoolInvoices(schoolId));

  const [editing, setEditing] = useState<SchoolInvoice | null>(null);
  const [isCreating, setIsCreating] = useState(false);
  const [form, setForm] = useState(emptyForm);
  const [formError, setFormError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<SchoolInvoice | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const isModalOpen = isCreating || editing !== null;

  const openCreate = useCallback(() => {
    setForm(emptyForm());
    setFormError(null);
    setEditing(null);
    setIsCreating(true);
  }, []);

  const openEdit = useCallback((invoice: SchoolInvoice) => {
    setForm(formFromInvoice(invoice));
    setFormError(null);
    setIsCreating(false);
    setEditing(invoice);
  }, []);

  const closeModal = useCallback(() => {
    if (isSaving) return;
    setIsCreating(false);
    setEditing(null);
    setFormError(null);
  }, [isSaving]);

  const handleSave = useCallback(async () => {
    const cents = parseEuroToCents(form.amountInput);
    if (form.periodLabel.trim() === "") {
      setFormError(
        "Bitte einen Zeitraum eintragen, zum Beispiel „Januar 2026“.",
      );
      return;
    }
    if (cents === null) {
      setFormError("Bitte einen Betrag wie 19,90 eintragen.");
      return;
    }
    if (form.dueDate === "") {
      setFormError("Bitte ein Fälligkeitsdatum wählen.");
      return;
    }
    if (form.status === "bezahlt" && !form.paidOn) {
      setFormError(
        "Bitte das Datum eintragen, an dem die Zahlung eingegangen ist.",
      );
      return;
    }

    const payload: SchoolInvoiceInput = {
      periodLabel: form.periodLabel.trim(),
      invoiceNumber: form.invoiceNumber.trim(),
      amountCents: cents,
      dueDate: form.dueDate,
      status: form.status,
      // Ein Zahlungsdatum ergibt nur bei „bezahlt“ Sinn; sonst wird es geleert.
      paidOn: form.status === "bezahlt" ? form.paidOn : null,
      note: form.note.trim(),
    };

    setIsSaving(true);
    setFormError(null);
    try {
      if (editing) {
        await updateSchoolInvoice(schoolId, editing.id, payload);
      } else {
        await createSchoolInvoice(schoolId, payload);
      }
      await mutate();
      setIsCreating(false);
      setEditing(null);
    } catch (saveError) {
      setFormError(
        errorMessage(
          saveError,
          "Die Rechnung konnte nicht gespeichert werden.",
        ),
      );
    } finally {
      setIsSaving(false);
    }
  }, [editing, form, mutate, schoolId]);

  const handleDelete = useCallback(async () => {
    if (!deleteTarget) return;
    setIsSaving(true);
    setDeleteError(null);
    try {
      await deleteSchoolInvoice(schoolId, deleteTarget.id);
      await mutate();
      setDeleteTarget(null);
    } catch (removeError) {
      setDeleteError(
        errorMessage(removeError, "Die Rechnung konnte nicht gelöscht werden."),
      );
    } finally {
      setIsSaving(false);
    }
  }, [deleteTarget, mutate, schoolId]);

  const columns: DataTableColumn<SchoolInvoice>[] = useMemo(
    () => [
      {
        key: "period",
        header: "Zeitraum",
        render: (row) => (
          <span className="font-medium text-gray-900">{row.periodLabel}</span>
        ),
        sortValue: (row) => row.dueDate,
      },
      {
        key: "due",
        header: "Fällig am",
        render: (row) => formatDate(row.dueDate),
        sortValue: (row) => row.dueDate,
      },
      {
        key: "amount",
        header: "Betrag",
        align: "right",
        render: (row) => formatCents(row.amountCents),
        sortValue: (row) => row.amountCents,
      },
      {
        key: "status",
        header: "Status",
        render: (row) => (
          <div className="flex flex-col items-start gap-1">
            <StatusBadge
              label={invoiceStatusLabel(row)}
              tone={invoiceStatusTone(row)}
            />
            {row.paidOn ? (
              <span className="text-xs text-gray-500">
                Bezahlt am {formatDate(row.paidOn)}
              </span>
            ) : null}
          </div>
        ),
        sortValue: (row) => invoiceStatusLabel(row),
      },
      {
        key: "number",
        header: "Rechnungsnummer",
        render: (row) =>
          row.invoiceNumber === "" ? (
            <span className="text-gray-500">Noch keine</span>
          ) : (
            row.invoiceNumber
          ),
        sortValue: (row) => row.invoiceNumber,
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (row) => (
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => openEdit(row)}
            >
              Bearbeiten
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => {
                setDeleteError(null);
                setDeleteTarget(row);
              }}
            >
              Löschen
            </Button>
          </div>
        ),
      },
    ],
    [openEdit],
  );

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="max-w-2xl text-sm text-gray-600">
          Zahlungsplan dieser Schule. Die Schule sieht diese Zeilen unter
          „Vertrag“, kann sie aber nicht ändern. Tarif und gebuchte Kinderzahl
          stehen im Reiter „Vertrag“ der Einstellungen.
        </p>
        <Button type="button" variant="primary" size="md" onClick={openCreate}>
          Rechnung anlegen
        </Button>
      </div>

      {error ? (
        <Alert
          type="error"
          message={errorMessage(
            error,
            "Der Zahlungsplan konnte nicht geladen werden.",
          )}
        />
      ) : null}

      <DataTable
        columns={columns}
        rows={invoices ?? []}
        getRowKey={(row) => row.id}
        isLoading={isLoading}
        caption="Zahlungsplan"
        defaultSortKey="due"
        defaultSortDirection="desc"
        emptyState={
          <EmptyState
            title="Noch keine Rechnungen"
            description="Legen Sie die erste Rechnung an, damit die Schule ihren Zahlungsplan sieht."
          />
        }
      />

      <FormModal
        isOpen={isModalOpen}
        onClose={closeModal}
        closeDisabled={isSaving}
        title={editing ? "Rechnung bearbeiten" : "Rechnung anlegen"}
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={closeModal}
              disabled={isSaving}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="primary"
              size="md"
              onClick={() => void handleSave()}
              disabled={isSaving}
            >
              {isSaving ? "Wird gespeichert…" : "Speichern"}
            </Button>
          </div>
        }
      >
        <div className="space-y-4">
          {formError ? <Alert type="error" message={formError} /> : null}

          <Input
            id="invoice-period-label"
            label="Zeitraum"
            placeholder="Januar 2026"
            value={form.periodLabel}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, periodLabel: event.target.value }))
            }
          />

          <Input
            id="invoice-amount"
            label="Betrag in Euro"
            placeholder="19,90"
            inputMode="decimal"
            value={form.amountInput}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, amountInput: event.target.value }))
            }
          />

          <Input
            id="invoice-due-date"
            label="Fällig am"
            type="date"
            value={form.dueDate}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, dueDate: event.target.value }))
            }
          />

          <div>
            <p className="mb-1 block text-sm font-medium text-gray-700">
              Status
            </p>
            <CustomSelect
              value={form.status}
              options={STATUS_OPTIONS}
              ariaLabel="Status"
              onChange={(value) =>
                setForm((prev) => ({
                  ...prev,
                  status: value as InvoiceStatus,
                  paidOn: value === "bezahlt" ? prev.paidOn : null,
                }))
              }
            />
          </div>

          {form.status === "bezahlt" ? (
            <Input
              id="invoice-paid-on"
              label="Zahlung eingegangen am"
              type="date"
              value={form.paidOn ?? ""}
              onChange={(event) =>
                setForm((prev) => ({
                  ...prev,
                  paidOn: event.target.value === "" ? null : event.target.value,
                }))
              }
            />
          ) : null}

          <Input
            id="invoice-number"
            label="Rechnungsnummer (optional)"
            placeholder="R-2026-001"
            value={form.invoiceNumber}
            onChange={(event) =>
              setForm((prev) => ({
                ...prev,
                invoiceNumber: event.target.value,
              }))
            }
          />

          <Input
            id="invoice-note"
            label="Interne Notiz (optional)"
            placeholder="Nur für das moto-Team"
            value={form.note}
            onChange={(event) =>
              setForm((prev) => ({ ...prev, note: event.target.value }))
            }
          />
        </div>
      </FormModal>

      <FormModal
        isOpen={deleteTarget !== null}
        onClose={() => {
          if (!isSaving) setDeleteTarget(null);
        }}
        closeDisabled={isSaving}
        size="sm"
        title="Rechnung löschen"
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setDeleteTarget(null)}
              disabled={isSaving}
            >
              Abbrechen
            </Button>
            <Button
              type="button"
              variant="danger"
              size="md"
              onClick={() => void handleDelete()}
              disabled={isSaving}
            >
              Löschen
            </Button>
          </div>
        }
      >
        <div className="space-y-3">
          {deleteError ? <Alert type="error" message={deleteError} /> : null}
          <p className="text-sm text-gray-700">
            {deleteTarget
              ? `„${deleteTarget.periodLabel}“ wird aus dem Zahlungsplan entfernt. Die Schule sieht die Zeile dann nicht mehr.`
              : ""}
          </p>
          <p className="text-sm text-gray-600">
            Eine zurückgezogene, aber echte Rechnung besser auf „Storniert“
            setzen — dann bleibt sie nachvollziehbar.
          </p>
        </div>
      </FormModal>
    </div>
  );
}
