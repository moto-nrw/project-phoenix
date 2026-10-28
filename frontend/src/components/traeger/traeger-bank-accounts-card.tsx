"use client";

import { Landmark, Plus } from "lucide-react";
import { useState } from "react";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiFormError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  deleteTraegerBankAccount,
  type TraegerBankAccount,
  updateTraegerBankAccount,
} from "~/lib/traeger-api";
import { formatIBAN } from "./traeger-bank-account-format";
import {
  type BankAccountEdit,
  TraegerBankAccountModal,
} from "./traeger-bank-account-modal";

const logger = createLogger({ component: "TraegerBankAccountsCard" });

/**
 * The bank accounts of the Träger (#4653), each with its label. Exactly one
 * is the default; it stands on every letter and invoice.
 */
export function TraegerBankAccountsCard({
  accounts,
  canEdit,
  onChanged,
}: Readonly<{
  accounts: readonly TraegerBankAccount[];
  canEdit: boolean;
  onChanged: () => Promise<unknown>;
}>) {
  const [edit, setEdit] = useState<BankAccountEdit | null>(null);
  const [deleting, setDeleting] = useState<TraegerBankAccount | null>(null);
  const [busy, setBusy] = useState(false);
  const deleteErrors = useApiFormError();
  const { show: showError } = useApiErrorDisplay();
  const { success } = useToast();

  const makeDefault = async (account: TraegerBankAccount) => {
    try {
      await updateTraegerBankAccount(account.id, {
        ...account,
        isDefault: true,
      });
      success(`„${account.label}“ ist jetzt der Standard.`);
      await onChanged();
    } catch (err) {
      logger.error("traeger_bank_account_default_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void showError(err, { object: "die Bankverbindung" });
    }
  };

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      await deleteTraegerBankAccount(deleting.id);
      success(`Die Bankverbindung „${deleting.label}“ ist gelöscht.`);
      setDeleting(null);
      await onChanged();
    } catch (err) {
      logger.error("traeger_bank_account_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "das Löschen der Bankverbindung" });
    } finally {
      setBusy(false);
    }
  };

  const columns: DataTableColumn<TraegerBankAccount>[] = [
    {
      key: "label",
      header: "Bezeichnung",
      render: (account) => (
        <span className="flex min-w-0 flex-wrap items-center gap-2 font-medium text-gray-900">
          <span className="min-w-0 wrap-anywhere">{account.label}</span>
          {account.isDefault ? (
            <StatusBadge label="Standard" tone="green" compact />
          ) : null}
        </span>
      ),
      stacked: "title",
    },
    {
      key: "holder",
      header: "Kontoinhaber",
      render: (account) => (
        <span className="wrap-anywhere">{account.accountHolder}</span>
      ),
      stacked: "field",
    },
    {
      key: "iban",
      header: "IBAN",
      render: (account) => (
        <span className="wrap-anywhere tabular-nums sm:whitespace-nowrap">
          {formatIBAN(account.iban)}
        </span>
      ),
      stacked: "field",
    },
    {
      key: "bank",
      header: "Bank",
      render: (account) => (
        <span className="wrap-anywhere">
          {[account.bankName, account.bic].filter(Boolean).join(" · ") || "–"}
        </span>
      ),
      stacked: "field",
    },
  ];
  if (canEdit) {
    columns.push({
      key: "actions",
      header: "",
      render: (account) => (
        <OverflowMenu
          ariaLabel={`Aktionen für die Bankverbindung ${account.label}`}
          triggerSize="sm"
          items={[
            {
              label: "Ändern",
              onClick: () => setEdit({ kind: "change", account }),
            },
            ...(account.isDefault
              ? []
              : [
                  {
                    label: "Als Standard festlegen",
                    onClick: () => void makeDefault(account),
                  },
                ]),
            {
              label: "Löschen",
              destructive: true,
              onClick: () => setDeleting(account),
            },
          ]}
        />
      ),
      align: "right",
      stacked: "meta",
    });
  }

  return (
    <SectionCard
      title="Bankverbindungen"
      description="Auf Briefen und Rechnungen steht die Standard-Bankverbindung."
      actions={
        canEdit ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() =>
              setEdit({ kind: "create", first: accounts.length === 0 })
            }
          >
            <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
            Bankverbindung hinzufügen
          </Button>
        ) : null
      }
    >
      <DataTable
        columns={columns}
        rows={[...accounts]}
        getRowKey={(account) => account.id}
        rowHasInteractiveControls={canEdit}
        stackedOnMobile
        emptyState={
          <EmptyState
            icon={<Landmark className="h-12 w-12" aria-hidden="true" />}
            title="Noch keine Bankverbindung hinterlegt."
          />
        }
      />
      <TraegerBankAccountModal
        edit={edit}
        onClose={() => setEdit(null)}
        onSaved={onChanged}
      />
      <ConfirmDeleteModal
        isOpen={deleting !== null}
        onClose={() => {
          setDeleting(null);
          deleteErrors.clear();
        }}
        onConfirm={() => void remove()}
        title="Bankverbindung löschen?"
        description={
          deleting?.isDefault && accounts.length > 1
            ? `„${deleting.label}“ ist der Standard. Danach wird die älteste andere Bankverbindung der Standard.`
            : `„${deleting?.label ?? ""}“ steht danach auf keinem neuen Brief mehr.`
        }
        confirmLabel="Löschen"
        loadingLabel="Wird gelöscht …"
        gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
        loading={busy}
        error={deleteErrors.error}
      />
    </SectionCard>
  );
}
