"use client";

// Kinderzahl der Familien (#5918): wie viele Kinder eine Familie hat, auch
// Geschwister ohne Betreuungsvertrag. Eltern geben die Zahl im Vertrag an,
// die Abrechnung bestätigt oder ändert sie mit Gültigkeit. Eine
// Geschwisterregel, die das einschaltet, zählt diese Zahl, mindestens aber
// die Kinder mit Vertrag. Nur das Recht „Abrechnung“ sieht die Seite; jedes
// Lesen steht im Zugriffsprotokoll des Vertrags.

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  billingAreaTitle,
  billingTabs,
} from "~/components/traeger/billing-family-format";
import {
  checkReason,
  childrenLabel,
  familyName,
  NEEDS_CHECK,
  STATUS_LABELS,
  STATUS_TONES,
  statedOn,
  validityLabel,
} from "~/components/traeger/family-children-format";
import { FamilyChildrenModal } from "~/components/traeger/family-children-modal";
import { ChevronDown, ChevronUp, TriangleAlert } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { parentsConcept } from "~/lib/concepts/parents";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type FamilyChildren,
  type FamilyChildrenSet,
  fetchFamilyChildren,
  removeFamilyChildren,
} from "~/lib/traeger-family-children-api";

const logger = createLogger({ component: "TraegerKinderzahlPage" });

/** Rows of „Zu prüfen“ shown before „N weitere anzeigen“. */
const INITIAL_CHECKS = 5;

function ContractChildrenCell({
  family,
}: Readonly<{ family: FamilyChildren }>) {
  if (family.children.length === 0) {
    return (
      <span className="text-sm text-gray-600">Noch kein wirksamer Vertrag</span>
    );
  }
  return (
    <div className="min-w-0">
      <p className="text-sm text-gray-900">
        {childrenLabel(family.children.length)}
      </p>
      <p className="mt-0.5 text-sm text-gray-600">
        {family.children
          .map((child) => `${child.childFirstName} (${child.schoolName})`)
          .join(", ")}
      </p>
    </div>
  );
}

function ConfirmedCell({ family }: Readonly<{ family: FamilyChildren }>) {
  const count = family.count;
  if (!count?.confirmedChildren) {
    return <span className="text-sm text-gray-500">– nicht bestätigt</span>;
  }
  return (
    <div className="min-w-0">
      <p className="text-sm font-semibold text-gray-900 tabular-nums">
        {childrenLabel(count.confirmedChildren)}
      </p>
      <p className="mt-0.5 text-sm text-gray-600">{validityLabel(count)}</p>
    </div>
  );
}

function StatedCell({ family }: Readonly<{ family: FamilyChildren }>) {
  const count = family.count;
  if (!count?.statedChildren) {
    return <span className="text-sm text-gray-500">–</span>;
  }
  return (
    <div className="min-w-0">
      <p className="text-sm text-gray-900 tabular-nums">
        {childrenLabel(count.statedChildren)}
      </p>
      <p className="mt-0.5 text-sm text-gray-600">am {statedOn(count)}</p>
    </div>
  );
}

/**
 * What the Monatslauf counts until the office checks, per kind of entry: an
 * expired number no longer counts, a changed or expiring one still does.
 */
function checkCardText(
  toCheck: readonly FamilyChildren[],
  missing: number,
): string {
  const expired = toCheck.some((family) => family.status === "expired");
  const stillValid = toCheck.some(
    (family) => family.status === "changed" || family.status === "expiring",
  );
  return [
    stillValid
      ? "Bei geänderten oder bald ablaufenden Zahlen zählt der Monatslauf bis zu Ihrer Prüfung die bisher bestätigte Zahl."
      : null,
    expired
      ? "Eine abgelaufene Kinderzahl gilt nicht mehr: Bis Sie neu prüfen, zählt der Monatslauf nur die Kinder mit Vertrag."
      : null,
    toCheck.some((family) => family.status === "stated")
      ? "Eine neue Angabe der Eltern zählt erst, wenn Sie sie bestätigen."
      : null,
    missing > 0
      ? `Ohne Angabe (${missing === 1 ? "1 Familie" : `${missing} Familien`}) zählen ebenfalls nur die Kinder mit Vertrag.`
      : null,
  ]
    .filter(Boolean)
    .join(" ");
}

export default function TraegerKinderzahlPage() {
  const today = berlinTodayISO();
  const [data, setData] = useState<FamilyChildrenSet | null>(null);
  const [search, setSearch] = useState("");
  const [checking, setChecking] = useState<FamilyChildren | null>(null);
  const [removing, setRemoving] = useState<FamilyChildren | null>(null);
  const [busy, setBusy] = useState(false);
  const [checksExpanded, setChecksExpanded] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const removeErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchFamilyChildren());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_family_children_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Übersicht der Kinderzahlen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async () => {
    const contract = removing?.count?.contractId;
    if (!removing || !contract) return;
    setBusy(true);
    removeErrors.clear();
    try {
      setData(await removeFamilyChildren(contract));
      success(`Die Kinderzahl von ${familyName(removing)} ist entfernt.`);
      setRemoving(null);
    } catch (err) {
      logger.error("family_children_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void removeErrors.show(err, { object: "die Kinderzahl" });
    } finally {
      setBusy(false);
    }
  };

  const families = data?.families ?? [];
  const toCheck = families.filter((family) =>
    NEEDS_CHECK.includes(family.status),
  );
  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase();
    return (data?.families ?? []).filter((family) =>
      needle
        ? [
            familyName(family),
            family.recipientEmail,
            ...family.children.map(
              (child) => `${child.childFirstName} ${child.childLastName}`,
            ),
          ]
            .join(" ")
            .toLowerCase()
            .includes(needle)
        : true,
    );
  }, [data, search]);

  const columns: DataTableColumn<FamilyChildren>[] = [
    {
      key: "family",
      header: "Familie",
      stacked: "title",
      className: "w-64 align-top",
      render: (family) => (
        <div className="min-w-0">
          <p className="font-semibold text-gray-900">{familyName(family)}</p>
          <p className="mt-0.5 truncate text-sm text-gray-600">
            {family.recipientEmail}
          </p>
        </div>
      ),
    },
    {
      key: "contracts",
      header: "Kinder mit Vertrag",
      stacked: "field",
      className: "align-top",
      render: (family) => <ContractChildrenCell family={family} />,
    },
    {
      key: "stated",
      header: "Angabe der Eltern",
      stacked: "field",
      className: "w-40 align-top",
      render: (family) => <StatedCell family={family} />,
    },
    {
      key: "confirmed",
      header: "Bestätigte Kinderzahl",
      stacked: "field",
      className: "w-48 align-top",
      render: (family) => <ConfirmedCell family={family} />,
    },
    {
      key: "status",
      header: "Stand",
      stacked: "field",
      className: "w-44 align-top",
      // „Keine Angabe“ is plain text: a missing number is no state of its
      // own, and the gray pill belongs to „Abgelaufen“.
      render: (family) =>
        family.status === "missing" ? (
          <span className="text-sm text-gray-600">{STATUS_LABELS.missing}</span>
        ) : (
          <StatusBadge
            tone={STATUS_TONES[family.status]}
            label={STATUS_LABELS[family.status]}
          />
        ),
    },
    {
      key: "actions",
      header: "",
      align: "right",
      className: "w-12 align-top",
      render: (family) => (
        <OverflowMenu
          ariaLabel={`Kinderzahl: ${familyName(family)}`}
          items={[
            {
              label: family.count?.confirmedChildren
                ? "Kinderzahl ändern"
                : "Kinderzahl bestätigen",
              onClick: () => setChecking(family),
            },
            ...(family.count
              ? [
                  {
                    label: "Kinderzahl entfernen",
                    destructive: true,
                    onClick: () => setRemoving(family),
                  },
                ]
              : []),
          ]}
        />
      ),
    },
  ];

  // Each figure of the status line counts one pill of the table.
  const countStatus = (status: FamilyChildren["status"]) =>
    families.filter((family) => family.status === status).length;
  const expired = countStatus("expired");
  const missing = countStatus("missing");

  return (
    <TenantPage
      title={billingAreaTitle("kinderzahl")}
      info="Hier steht, wie viele Kinder eine Familie hat, auch Geschwister ohne Betreuungsvertrag. Eltern geben die Zahl im Vertrag an. Sie bestätigen oder ändern sie. Eine Geschwisterregel nutzt die Zahl nur, wenn Sie das unter „Ermäßigungen“ einschalten. moto speichert nur die Zahl, keine Namen der Geschwister. Jedes Öffnen dieser Seite steht im Zugriffsprotokoll."
      stats={
        families.length > 0 ? (
          <TenantPageStats
            // Three figures fit the line; each counts one pill of the table.
            items={[
              { value: countStatus("confirmed"), label: "bestätigt" },
              ...(expired > 0 ? [{ value: expired, label: "abgelaufen" }] : []),
              { value: missing, label: "ohne Angabe" },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Familie, Adresse oder Kind",
      }}
      tabs={billingTabs("kinderzahl")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        <>
          <FamilyChildrenModal
            family={checking}
            onClose={() => setChecking(null)}
            onSaved={(next) => {
              setData(next);
              setChecking(null);
            }}
          />
          <ConfirmDeleteModal
            isOpen={removing !== null}
            onClose={() => {
              setRemoving(null);
              removeErrors.clear();
            }}
            onConfirm={() => void remove()}
            title="Kinderzahl entfernen?"
            description={
              removing
                ? `Die Kinderzahl von ${familyName(removing)} fällt weg. Neue Monatsläufe zählen dann nur die Kinder mit Vertrag. Ausgestellte Rechnungen bleiben, wie sie sind.`
                : ""
            }
            confirmLabel="Entfernen"
            loadingLabel="Wird entfernt …"
            gate={{ mode: "twoStep", firstStepLabel: "Ja, entfernen" }}
            loading={busy}
            error={removeErrors.error}
          />
        </>
      }
    >
      {data && families.length === 0 ? (
        <EmptyState
          icon={<MotoConceptIcon concept={parentsConcept} size={48} />}
          title="Noch keine Familie."
          description="Familien erscheinen hier, sobald ein Kind einen Betreuungsvertrag hat."
        />
      ) : null}
      {data && families.length > 0 && data.rules.length === 0 ? (
        <Alert
          type="info"
          message="Noch keine Geschwisterregel zählt diese Zahl. Schalten Sie das bei der Regel unter „Ermäßigungen“ ein."
        />
      ) : null}
      {data && toCheck.length > 0 ? (
        <SectionCard
          title="Zu prüfen"
          titleBadge={
            <StatusBadge tone="orange" label={`${toCheck.length} offen`} />
          }
          description={checkCardText(toCheck, missing)}
          leading={
            <TriangleAlert
              className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
          }
        >
          <ResponsiveActionList>
            {(checksExpanded ? toCheck : toCheck.slice(0, INITIAL_CHECKS)).map(
              (family) => (
                <li
                  key={family.recipientEmail}
                  className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center"
                >
                  <div className="min-w-0">
                    <p className="flex flex-wrap items-center gap-2 text-sm font-semibold text-gray-900">
                      {familyName(family)}
                      <StatusBadge
                        tone={STATUS_TONES[family.status]}
                        label={STATUS_LABELS[family.status]}
                      />
                    </p>
                    <p className="mt-0.5 text-sm text-gray-600">
                      {checkReason(family, today)}
                    </p>
                  </div>
                  <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                    <Button
                      type="button"
                      variant="surface"
                      size="compact"
                      aria-label={`Kinderzahl von ${familyName(family)} prüfen`}
                      onClick={() => setChecking(family)}
                    >
                      Prüfen
                    </Button>
                  </div>
                </li>
              ),
            )}
          </ResponsiveActionList>
          {toCheck.length > INITIAL_CHECKS ? (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              className="mt-3"
              aria-expanded={checksExpanded}
              onClick={() => setChecksExpanded((open) => !open)}
            >
              {checksExpanded ? (
                <>
                  <ChevronUp className="size-4" aria-hidden="true" />
                  Weniger anzeigen
                </>
              ) : (
                <>
                  <ChevronDown className="size-4" aria-hidden="true" />
                  {toCheck.length - INITIAL_CHECKS} weitere anzeigen
                </>
              )}
            </Button>
          ) : null}
        </SectionCard>
      ) : null}
      {data && families.length > 0 ? (
        <DataTable
          columns={columns}
          rows={rows}
          getRowKey={(family) => family.recipientEmail}
          stackedOnMobile
          emptyState={
            <EmptyState
              title="Keine Familie passt zur Suche."
              description="Ändern Sie die Suche."
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
