"use client";

// Nachweise der Verträge (#4685): welche Einkommensstufe ein Kind hat und ob
// die Ermäßigung für Alleinerziehende gilt, mit Art des Nachweises,
// Prüfdatum und Gültigkeit. Nur das Recht „Abrechnung“ sieht sie; jedes
// Lesen steht im Zugriffsprotokoll des Vertrags.

import { FileCheck2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import {
  CareEvidenceModal,
  EVIDENCE_KIND_LABELS,
  type EvidenceTarget,
} from "~/components/traeger/care-evidence-modal";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
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

/** The evidence of a kind that holds today, else the next one. */
function currentEvidence(
  contract: CareEvidenceContract,
  kind: CareEvidenceKind,
  today: string,
): CareEvidence | null {
  const ofKind = contract.evidences.filter(
    (evidence) =>
      evidence.kind === kind &&
      (evidence.validUntil === null || evidence.validUntil >= today),
  );
  return (
    ofKind.find((evidence) => evidence.validFrom <= today) ?? ofKind[0] ?? null
  );
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
  return (
    <div className="min-w-0">
      <p className="flex flex-wrap items-center gap-2 text-sm text-gray-900">
        {level ? level.name : "Alleinerziehend"}
        {starts ? (
          <StatusBadge
            tone="blue"
            label={`Ab ${formatDate(evidence.validFrom)}`}
          />
        ) : null}
      </p>
      <p className="mt-0.5 text-xs text-gray-600">
        {evidence.evidence} · geprüft am {formatDate(evidence.checkedOn)}
        {evidence.validUntil ? ` · bis ${formatDate(evidence.validUntil)}` : ""}
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
        object: "die Nachweise",
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
      void deleteErrors.show(err, { object: "den Nachweis" });
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
    (data?.contracts ?? []).filter(
      (contract) => currentEvidence(contract, kind, today) !== null,
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
          <EvidenceCell
            evidence={currentEvidence(contract, kind, today)}
            data={data}
            today={today}
          />
        ) : null,
    })),
    {
      key: "actions",
      header: "",
      align: "right",
      className: "w-12 align-top",
      render: (contract) => {
        const items = KINDS.flatMap((kind) => {
          const evidence = currentEvidence(contract, kind, today);
          if (kind === "income_level" && data?.levels.length === 0) return [];
          return evidence
            ? [
                {
                  label: `${EVIDENCE_KIND_LABELS[kind]} ändern`,
                  onClick: () => setTarget({ contract, kind, evidence }),
                },
                {
                  label: `${EVIDENCE_KIND_LABELS[kind]} löschen`,
                  destructive: true,
                  onClick: () => setDeleting({ contract, evidence }),
                },
              ]
            : [
                {
                  label: `${EVIDENCE_KIND_LABELS[kind]} eintragen`,
                  onClick: () => setTarget({ contract, kind, evidence: null }),
                },
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
      title="Abrechnung"
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
                  ? `Der Nachweis für ${deleting.contract.childFirstName} ${deleting.contract.childLastName} fällt weg. Ab dem nächsten Monatslauf gilt die Ermäßigung nicht mehr. Ausgestellte Rechnungen bleiben, wie sie sind.`
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
      {data && data.contracts.length > 0 ? (
        <SectionCard padding="none">
          {data.levels.length === 0 ? (
            <p className="border-b border-gray-100 px-5 py-3 text-sm text-gray-600">
              Für Einkommensstufen legen Sie zuerst Stufen unter „Ermäßigungen“
              an.
            </p>
          ) : null}
          <DataTable
            columns={columns}
            rows={rows}
            getRowKey={(contract) => contract.contractId}
            stackedOnMobile
          />
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
