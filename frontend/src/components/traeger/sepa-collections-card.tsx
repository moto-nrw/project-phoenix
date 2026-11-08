"use client";

// Die Abbuchungen einer Familie je Rechnung (#4697): was in welcher
// Lastschriftdatei war, was noch offen ist und was nicht abgebucht werden
// darf. Nur zur Information; eingereicht wird auf der Seite des Laufs.

import { useCallback, useEffect, useState } from "react";
import { formatEuro } from "~/components/traeger/contribution-format";
import {
  SEPA_DEBIT_LEFT_LABELS,
  SEPA_SEQUENCE_BADGES,
} from "~/components/traeger/sepa-debit-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type { SepaMandate } from "~/lib/traeger-sepa-api";
import {
  type SepaDebit,
  fetchSepaMandateCollections,
} from "~/lib/traeger-sepa-debits-api";

const logger = createLogger({ component: "SepaCollectionsCard" });

function debitState(debit: SepaDebit) {
  if (debit.status === "submitted") {
    return (
      <span className="flex flex-col items-start gap-1">
        <StatusBadge label="Bei der Bank eingereicht" tone="green" size="xs" />
        {debit.submittedAt ? (
          <span className="text-xs text-gray-500 tabular-nums">
            am {formatDate(debit.submittedAt)}
          </span>
        ) : null}
      </span>
    );
  }
  if (debit.left) {
    return (
      <span className="flex flex-col items-start gap-1">
        <StatusBadge label="Nicht in der Datei" tone="red" size="xs" />
        <span className="text-xs text-gray-600">
          {SEPA_DEBIT_LEFT_LABELS[debit.left]}
        </span>
      </span>
    );
  }
  return <StatusBadge label="Noch nicht eingereicht" tone="orange" size="xs" />;
}

const columns: DataTableColumn<SepaDebit>[] = [
  {
    key: "invoice",
    header: "Rechnung",
    stacked: "title",
    render: (debit) => (
      <span className="font-medium text-gray-900 tabular-nums">
        {debit.invoiceNumber}
      </span>
    ),
  },
  {
    key: "amount",
    header: "Betrag",
    align: "right",
    stacked: "meta",
    render: (debit) => (
      <span className="tabular-nums">{formatEuro(debit.amountCents)}</span>
    ),
  },
  {
    key: "due",
    header: "Fällig",
    render: (debit) => (
      <span className="tabular-nums">{formatDate(debit.dueOn)}</span>
    ),
  },
  {
    key: "sequence",
    header: "Art",
    render: (debit) => {
      const badge = SEPA_SEQUENCE_BADGES[debit.sequenceType];
      return <StatusBadge label={badge.label} tone={badge.tone} size="xs" />;
    },
  },
  {
    key: "state",
    header: "Stand",
    render: debitState,
  },
];

export function SepaCollectionsCard({
  mandates,
}: Readonly<{ mandates: readonly SepaMandate[] }>) {
  const [debits, setDebits] = useState<SepaDebit[] | null>(null);
  const load = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = load;
  const ids = mandates.map((mandate) => mandate.id).join(",");

  const reload = useCallback(async () => {
    try {
      const lists = await Promise.all(
        ids
          .split(",")
          .filter(Boolean)
          .map((id) => fetchSepaMandateCollections(id)),
      );
      setDebits(lists.flat().sort((a, b) => b.dueOn.localeCompare(a.dueOn)));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_sepa_collections_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Abbuchungen",
        retry: () => void reload(),
      });
    }
  }, [ids, clearLoadError, showLoadError]);

  useEffect(() => {
    void reload();
  }, [reload]);

  if (mandates.length === 0) return null;
  return (
    <SectionCard
      title="Abbuchungen"
      description="Jede Rechnung, die per Lastschrift abgebucht wird. Nur zur Information: Die Datei für die Bank laden Sie auf der Seite des Laufs."
      loading={debits === null && !load.error}
    >
      {load.error ? (
        <LoadErrorAlert error={load.error} actionLayout="responsive" />
      ) : debits && debits.length > 0 ? (
        <DataTable
          columns={columns}
          rows={debits}
          getRowKey={(debit) => debit.id}
          stackedOnMobile
        />
      ) : debits ? (
        <EmptyState
          variant="compact"
          title="Noch keine Abbuchung"
          description="Sobald ein Lauf mit diesem Mandat freigegeben ist, steht die Rechnung hier."
        />
      ) : null}
    </SectionCard>
  );
}
