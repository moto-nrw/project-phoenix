"use client";

// Nachweise der Verträge (#4685): welche Einkommensstufe ein Kind hat und ob
// die Ermäßigung für Alleinerziehende gilt, mit Art des Nachweises,
// Prüfdatum und Gültigkeit. Nur das Recht „Abrechnung“ sieht sie; jedes
// Lesen steht im Zugriffsprotokoll des Vertrags.

import { FileCheck2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  billingAreaTitle,
  billingTabs,
} from "~/components/traeger/billing-family-format";
import {
  CareEvidenceModal,
  EVIDENCE_KIND_LABELS,
  type EvidenceTarget,
} from "~/components/traeger/care-evidence-modal";
import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { berlinTodayISO, formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareEvidence,
  type CareEvidenceContract,
  type CareEvidenceKind,
  type CareEvidenceSet,
  deleteCareEvidence,
  fetchCareEvidences,
} from "~/lib/traeger-care-reductions-api";

const logger = createLogger({ component: "TraegerNachweisePage" });

const KINDS: readonly CareEvidenceKind[] = ["income_level", "single_parent"];

/** "ab 01.08.2026" or "01.08.2026 bis 31.07.2027". */
function evidencePeriod(evidence: CareEvidence): string {
  return evidence.validUntil
    ? `${formatDate(evidence.validFrom)} bis ${formatDate(evidence.validUntil)}`
    : `ab ${formatDate(evidence.validFrom)}`;
}

function EvidenceCell({
  evidence,
  data,
  today,
}: Readonly<{
  evidence: CareEvidence | null;
  data: CareEvidenceSet;
  today: string;
}>) {
  if (!evidence) return <span className="text-sm text-gray-500">–</span>;
  const level = data.levels.find((rule) => rule.id === evidence.ruleId);
  const starts = evidence.validFrom > today;
  const expired = evidence.validUntil !== null && evidence.validUntil < today;
  return (
    <div className="min-w-0">
      <p className="flex flex-wrap items-center gap-2 text-sm text-gray-900">
        {/* Under the column „Alleinerziehend“ the value is a yes, not the
            column's name again. */}
        {level ? level.name : "Ja"}
        {starts ? (
          <StatusBadge
            tone="blue"
            label={`Ab ${formatDate(evidence.validFrom)}`}
          />
        ) : null}
        {expired ? <StatusBadge tone="gray" label="Abgelaufen" /> : null}
      </p>
      <p className="mt-0.5 text-sm text-gray-600">
        {evidencePeriod(evidence)}
        {" · "}
        {evidence.evidence} · geprüft am {formatDate(evidence.checkedOn)}
      </p>
    </div>
  );
}

export default function TraegerNachweisePage() {
  const today = berlinTodayISO();
  const [data, setData] = useState<CareEvidenceSet | null>(null);
  const [search, setSearch] = useState("");
  const [target, setTarget] = useState<EvidenceTarget | null>(null);
  const [deleting, setDeleting] = useState<{
    contract: CareEvidenceContract;
    evidence: CareEvidence;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchCareEvidences());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_evidences_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Nachweise",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      setData(await deleteCareEvidence(deleting.evidence.id));
      success("Der Nachweis ist gelöscht.");
      setDeleting(null);
    } catch (err) {
      logger.error("care_evidence_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "die Angabe zum Nachweis" });
    } finally {
      setBusy(false);
    }
  };

  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return (data?.contracts ?? []).filter((contract) =>
      needle
        ? `${contract.childFirstName} ${contract.childLastName} ${contract.schoolName}`
            .toLowerCase()
            .includes(needle)
        : true,
    );
  }, [data, search]);

  const counted = (kind: CareEvidenceKind) =>
    (data?.contracts ?? []).filter((contract) =>
      contract.evidences.some(
        (evidence) =>
          evidence.kind === kind &&
          (evidence.validUntil === null || evidence.validUntil >= today),
      ),
    ).length;

  const columns: DataTableColumn<CareEvidenceContract>[] = [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      className: "w-56 align-top",
      render: (contract) => (
        <div className="min-w-0">
          <p className="font-semibold text-gray-900">
            {contract.childFirstName} {contract.childLastName}
          </p>
          <p className="mt-0.5 truncate text-sm text-gray-600">
            {contract.schoolName}
          </p>
        </div>
      ),
    },
    ...KINDS.map((kind): DataTableColumn<CareEvidenceContract> => ({
      key: kind,
      header: EVIDENCE_KIND_LABELS[kind],
      stacked: "field",
      className: "align-top",
      render: (contract) =>
        data ? (
          <div className="space-y-3">
            {contract.evidences.filter((evidence) => evidence.kind === kind)
              .length === 0 ? (
              <span className="text-sm text-gray-500">–</span>
            ) : null}
            {contract.evidences
              .filter((evidence) => evidence.kind === kind)
              .map((evidence) => (
                <EvidenceCell
                  key={evidence.id}
                  evidence={evidence}
                  data={data}
                  today={today}
                />
              ))}
          </div>
        ) : null,
    })),
    {
      key: "actions",
      header: "",
      align: "right",
      className: "w-12 align-top",
      render: (contract) => {
        const items = KINDS.flatMap((kind) => {
          const existing = contract.evidences.filter(
            (evidence) => evidence.kind === kind,
          );
          const canCreate =
            kind !== "income_level" || data?.levels.length !== 0;
          return [
            ...(canCreate
              ? [
                  {
                    label: `${EVIDENCE_KIND_LABELS[kind]} eintragen`,
                    onClick: () =>
                      setTarget({ contract, kind, evidence: null }),
                  },
                ]
              : []),
            ...existing.flatMap((evidence) => {
              const description = evidencePeriod(evidence);
              return [
                {
                  label: `${EVIDENCE_KIND_LABELS[kind]} ändern`,
                  description,
                  onClick: () => setTarget({ contract, kind, evidence }),
                },
                {
                  label: `${EVIDENCE_KIND_LABELS[kind]} löschen`,
                  description,
                  destructive: true,
                  onClick: () => setDeleting({ contract, evidence }),
                },
              ];
            }),
          ];
        });
        return (
          <OverflowMenu
            ariaLabel={`Nachweise: ${contract.childFirstName} ${contract.childLastName}`}
            items={items}
          />
        );
      },
    },
  ];

  return (
    <TenantPage
      title={billingAreaTitle("nachweise")}
      info="Hier tragen Sie ein, welche Einkommensstufe ein Kind hat und ob die Ermäßigung für Alleinerziehende gilt. moto speichert nur die Art des Nachweises und das Prüfdatum, kein Dokument und kein Einkommen. Jedes Öffnen dieser Seite steht im Zugriffsprotokoll."
      stats={
        data && data.contracts.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: data.contracts.length,
                label: data.contracts.length === 1 ? "Vertrag" : "Verträge",
              },
              { value: counted("income_level"), label: "mit Stufe" },
              { value: counted("single_parent"), label: "alleinerziehend" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Kind oder Schule",
      }}
      tabs={billingTabs("nachweise")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <>
            <CareEvidenceModal
              data={data}
              target={target}
              onClose={() => setTarget(null)}
              onSaved={(next) => {
                setData(next);
                setTarget(null);
              }}
            />
            <ConfirmDeleteModal
              isOpen={deleting !== null}
              onClose={() => {
                setDeleting(null);
                deleteErrors.clear();
              }}
              onConfirm={() => void remove()}
              title="Nachweis löschen?"
              description={
                deleting
                  ? `Der Nachweis für ${deleting.contract.childFirstName} ${deleting.contract.childLastName} (${evidencePeriod(deleting.evidence)}) fällt weg. Neue Monatsläufe rechnen diesen Zeitraum ohne den Nachweis ab. Ausgestellte Rechnungen bleiben, wie sie sind.`
                  : ""
              }
              confirmLabel="Löschen"
              loadingLabel="Wird gelöscht …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
              loading={busy}
              error={deleteErrors.error}
            />
          </>
        ) : null
      }
    >
      {data && data.contracts.length === 0 ? (
        <EmptyState
          icon={<FileCheck2 className="h-12 w-12" aria-hidden="true" />}
          title="Noch kein Vertrag."
          description="Nachweise tragen Sie für Kinder mit Betreuungsvertrag ein."
        />
      ) : null}
      {data && data.contracts.length > 0 && data.levels.length === 0 ? (
        <Alert
          type="info"
          message="Für Einkommensstufen legen Sie zuerst Stufen unter „Ermäßigungen“ an."
        />
      ) : null}
      {data && data.contracts.length > 0 ? (
        <DataTable
          columns={columns}
          rows={rows}
          getRowKey={(contract) => contract.contractId}
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Kein Kind passt zur Suche."
              description="Ändern Sie die Suche."
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
