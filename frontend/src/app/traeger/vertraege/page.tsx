"use client";

// Betreuungsverträge des Trägers (#4678, ADR 0153): alle Verträge mit Status,
// gefiltert nach Status, Schule und Name. „Verträge erstellen“ legt Entwürfe
// aus angenommenen Anmeldungen an; ausgewählte Entwürfe gehen per Mail und in
// die Eltern-App. Schulen mit „Bei der Annahme“ (#4680) brauchen das
// nicht: ihr Vertrag entsteht von selbst und ist sofort aktiv. Nur die Rolle
// mit dem Recht „Verträge verwalten“ sieht die Seite.

import { Plus, Send } from "lucide-react";
import { useRouter } from "next/navigation";
import {
  type MouseEvent,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useSession } from "next-auth/react";
import { CareContractCreateModal } from "~/components/traeger/care-contract-create-modal";
import {
  CONCLUSION_MODE_LABELS,
  CONCLUSION_MODE_TONES,
  CONTRACT_STATUS_LABELS,
  CONTRACT_STATUS_ORDER,
  CONTRACT_STATUS_TONES,
  SEND_SKIP_LABELS,
  childName,
} from "~/components/traeger/care-contract-format";
import { careContractTabs } from "~/components/traeger/care-contract-tabs";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import NavigationLink from "~/components/ui/navigation-link";
import type {
  ActiveFilter,
  FilterConfig,
} from "~/components/ui/page-header/types";
import { SectionCard } from "~/components/ui/section-card";
import { ApiError } from "~/lib/api-error";
import { hasPermission } from "~/lib/auth-utils";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { formatBerlinDate, formatDate } from "~/lib/date-helpers";
import { useRefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import {
  type CareContractList,
  type CareContractStatus,
  type CareContractSummary,
  type ContractCandidate,
  type CreateResult,
  type SchoolConclusionMode,
  fetchCareContracts,
  fetchConclusionModes,
  fetchContractCandidates,
  sendCareContracts,
} from "~/lib/traeger-care-contracts-api";
import { fetchTraegerSchools, type TraegerSchool } from "~/lib/traeger-api";
import { useDebounce } from "~/lib/use-debounce";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerVertraegePage" });

type StatusChoice = CareContractStatus | "all";

interface PageData {
  readonly list: CareContractList;
  readonly candidates: readonly ContractCandidate[];
  readonly schools: readonly TraegerSchool[];
  readonly modes: readonly SchoolConclusionMode[];
}

function conclusionDescription(modes: readonly SchoolConclusionMode[]): string {
  const onAcceptance = modes.filter((school) => school.mode === "bei_annahme");
  if (onAcceptance.length === 0) {
    return "Sie erstellen jeden Vertrag hier und schicken ihn den Eltern.";
  }
  if (onAcceptance.length === modes.length) {
    return "Nimmt die Schule eine Anmeldung an, gilt der Vertrag. Die Eltern bekommen ihn per E-Mail und in der Eltern-App. Voraussetzung: Die Eltern haben im Anmeldeformular den Vertragsbedingungen zugestimmt, und Sie haben die Bedingungen seitdem nicht geändert.";
  }
  return "Steht eine Schule auf „Bei der Annahme“, entsteht der Vertrag von selbst, sobald die Schule die Anmeldung annimmt. Voraussetzung: Die Eltern haben im Anmeldeformular den Vertragsbedingungen zugestimmt. Die anderen Verträge erstellen und verschicken Sie hier.";
}

function contractHref(contract: CareContractSummary): string {
  return traegerPath(`/traeger/vertraege/${contract.id}`);
}

const COLUMNS: DataTableColumn<CareContractSummary>[] = [
  {
    key: "child",
    header: "Kind",
    render: (contract) => (
      <span className="block min-w-0">
        <NavigationLink
          href={contractHref(contract)}
          className="font-medium text-gray-900 hover:underline"
          onClick={(event: MouseEvent<HTMLAnchorElement>) =>
            event.stopPropagation()
          }
        >
          {childName(contract)}
        </NavigationLink>
        <span className="block text-sm text-gray-500">
          {contract.modulesText || "–"}
          <span className="sm:hidden">
            {" "}
            · ab {formatDate(contract.effectiveFrom)}
          </span>
        </span>
      </span>
    ),
    sortValue: (contract) =>
      `${contract.childLastName} ${contract.childFirstName}`,
    stacked: "title",
  },
  {
    key: "school",
    header: "Betreuungsort",
    render: (contract) => contract.schoolName,
    sortValue: (contract) => contract.schoolName,
    stacked: "hidden",
  },
  {
    key: "party",
    header: "Eltern",
    render: (contract) => (
      <span className="block min-w-0">
        <span className="block text-gray-900">{contract.partyName || "–"}</span>
        <span className="block text-sm wrap-anywhere text-gray-500">
          {contract.partyEmail || "Keine E-Mail-Adresse"}
        </span>
      </span>
    ),
    stacked: "hidden",
  },
  {
    key: "start",
    header: "Beginn",
    render: (contract) => formatDate(contract.effectiveFrom),
    sortValue: (contract) => contract.effectiveFrom,
    stacked: "hidden",
  },
  {
    key: "status",
    header: "Status",
    render: (contract) => (
      <span className="inline-flex flex-col items-start gap-1">
        <StatusBadge
          label={CONTRACT_STATUS_LABELS[contract.status]}
          tone={CONTRACT_STATUS_TONES[contract.status]}
        />
        {contract.status === "gekuendigt" && contract.endsOn ? (
          <span className="text-xs text-gray-500">
            zum {formatDate(contract.endsOn)}
          </span>
        ) : contract.status === "gekuendigt" ? (
          <span className="text-xs text-gray-500">Ende offen</span>
        ) : contract.status === "widerrufen" ? null : contract.concluded ? (
          <span className="text-xs text-gray-500">
            seit {formatBerlinDate(contract.concluded.at)}
          </span>
        ) : contract.sentAt ? (
          <span className="text-xs text-gray-500">
            am {formatBerlinDate(contract.sentAt)}
          </span>
        ) : null}
      </span>
    ),
    sortValue: (contract) => CONTRACT_STATUS_ORDER.indexOf(contract.status),
    stacked: "meta",
    // On a phone the status alone, without the date under it.
    stackedRender: (contract) => (
      <StatusBadge
        label={CONTRACT_STATUS_LABELS[contract.status]}
        tone={CONTRACT_STATUS_TONES[contract.status]}
      />
    ),
  },
];

export default function TraegerVertraegePage() {
  const router = useRouter();
  const { data: session } = useSession();
  const [data, setData] = useState<PageData | null>(null);
  const [status, setStatus] = useState<StatusChoice>("all");
  const [schoolId, setSchoolId] = useState("");
  const [search, setSearch] = useState("");
  const debouncedSearch = useDebounce(search.trim(), 250);
  const context = useRef<{
    candidates: ContractCandidate[];
    schools: TraegerSchool[];
    modes: SchoolConclusionMode[];
  } | null>(null);
  const request = useRef(0);
  const [loadingMore, setLoadingMore] = useState(false);
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const [creating, setCreating] = useState(false);
  const [sending, setSending] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const actionError = useApiErrorDisplay();
  const { success, warning } = useToast();

  // Reads one page of the list for request `currentRequest` and sets it;
  // rejects on a failure. An answer for an older request is dropped.
  const readContracts = useCallback(
    async (offset: number, currentRequest: number) => {
      const [list, metadata] = await Promise.all([
        fetchCareContracts({
          status: status === "all" ? undefined : status,
          schoolId,
          search: debouncedSearch,
          offset,
        }),
        context.current ??
          Promise.all([
            fetchContractCandidates(),
            fetchTraegerSchools(),
            fetchConclusionModes(),
          ]).then(([candidates, schools, modes]) => ({
            candidates: candidates.map((candidate) =>
              modes.some(
                (school) =>
                  school.schoolId === candidate.schoolId &&
                  school.mode === "bei_annahme",
              )
                ? {
                    ...candidate,
                    templateIssue:
                      candidate.templateIssue ??
                      (!candidate.guardianEmail.trim()
                        ? ("recipient_missing" as const)
                        : null) ??
                      (candidate.acceptanceIssue === "acceptance_missing" &&
                      candidate.renewal
                        ? ("acceptance_missing_renewal" as const)
                        : candidate.acceptanceIssue) ??
                      (candidate.mailBlock === "stopped"
                        ? ("mail_stopped" as const)
                        : null) ??
                      (candidate.mailBlock === "address"
                        ? ("mail_blocked" as const)
                        : null) ??
                      ("conclusion_automatic" as const),
                  }
                : candidate,
            ),
            schools,
            modes,
          })),
      ]).catch((error: unknown) => {
        if (
          currentRequest === request.current &&
          error instanceof ApiError &&
          (error.status === 401 || error.status === 403)
        ) {
          setData(null);
          context.current = null;
          setSelected(new Set());
          setCreating(false);
        }
        throw error;
      });
      if (currentRequest !== request.current) return;
      context.current = metadata;
      setData((previous) => ({
        ...metadata,
        list: {
          ...list,
          contracts:
            offset > 0 && previous
              ? [...previous.list.contracts, ...list.contracts]
              : list.contracts,
        },
      }));
    },
    [status, schoolId, debouncedSearch],
  );

  const load = useCallback(
    async (offset = 0) => {
      const currentRequest = ++request.current;
      if (offset > 0) setLoadingMore(true);
      try {
        await readContracts(offset, currentRequest);
        if (currentRequest !== request.current) return;
        clearLoadError();
      } catch (err) {
        if (currentRequest !== request.current) return;
        logger.error("traeger_care_contracts_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        await showLoadError(err, {
          object: "die Verträge",
          retry: () => void load(offset),
        });
      } finally {
        if (currentRequest === request.current) setLoadingMore(false);
      }
    },
    [readContracts, clearLoadError, showLoadError],
  );

  const invalidateRequest = useCallback(() => {
    request.current++;
  }, []);

  useEffect(() => {
    setData(null);
    setSelected(new Set());
    void load();
    return invalidateRequest;
  }, [load, invalidateRequest]);

  // Reload after a send: a failure there is a load error above the
  // confirmed rows, not a failed send (#6652). A reload that a newer filter
  // overtook fails silently; the newer load answers for it. The reload
  // replaces a page that was still loading more.
  const afterWrite = useRefreshAfterWrite(
    "die Anzeige der Verträge",
    async () => {
      const currentRequest = ++request.current;
      try {
        await readContracts(0, currentRequest);
      } catch (err) {
        if (currentRequest === request.current) throw err;
      } finally {
        if (currentRequest === request.current) setLoadingMore(false);
      }
    },
  );

  const contracts = useMemo(() => [...(data?.list.contracts ?? [])], [data]);
  const schools = (data?.schools ?? []).map((school) => ({
    value: school.id,
    label: school.name,
  }));
  const total = Object.values(data?.list.counts ?? {}).reduce(
    (sum, count) => sum + count,
    0,
  );
  const drafts = data?.list.counts.entwurf ?? 0;
  const sentCount = data?.list.counts.versendet ?? 0;
  const activeCount = data?.list.counts.aktiv ?? 0;
  const candidates = data?.candidates ?? [];
  const selectedDrafts = contracts.filter(
    (contract) => selected.has(contract.id) && contract.status === "entwurf",
  );

  const sendSelected = async () => {
    if (selectedDrafts.length === 0) return;
    setSending(true);
    let result: Awaited<ReturnType<typeof sendCareContracts>>;
    try {
      result = await sendCareContracts(selectedDrafts.map((c) => c.id));
    } catch (err) {
      logger.error("traeger_care_contracts_send_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void actionError.show(err, { object: "die Verträge" });
      setSelected(new Set());
      await load();
      setSending(false);
      return;
    }
    if (result.sent.length > 0) {
      success(
        result.sent.length === 1
          ? "1 Vertrag ist unterwegs: per Mail und in die Eltern-App."
          : `${result.sent.length} Verträge sind unterwegs: per Mail und in die Eltern-App.`,
      );
    }
    for (const skip of result.skipped) {
      const contract = contracts.find((c) => c.id === skip.id);
      warning(
        `${contract ? childName(contract) : "Ein Vertrag"}: nicht gesendet, ${SEND_SKIP_LABELS[skip.reason]}.`,
      );
    }
    setSelected(new Set());
    await afterWrite.refresh();
    setSending(false);
  };

  const onCreated = async (result: CreateResult) => {
    setCreating(false);
    if (result.created.length > 0) {
      success(
        result.created.length === 1
          ? "1 Entwurf ist erstellt. Prüfen Sie ihn und senden Sie ihn dann."
          : `${result.created.length} Entwürfe sind erstellt. Prüfen Sie sie und senden Sie sie dann.`,
      );
    }
    if (result.skipped.length > 0) {
      warning(
        result.skipped.length === 1
          ? "1 Kind hatte inzwischen einen Vertrag oder keine Bedingungen."
          : `${result.skipped.length} Kinder hatten inzwischen einen Vertrag oder keine Bedingungen.`,
      );
    }
    context.current = null;
    // Die Entwürfe sind angelegt; ein Fehler beim Nachladen lässt die
    // bestätigte Liste stehen (#6652). Ein anderer Filter lädt neu.
    if (status === "entwurf") await afterWrite.refresh();
    else setStatus("entwurf");
  };

  const filters: FilterConfig[] = [
    {
      id: "status",
      label: "Status",
      type: "dropdown",
      value: status,
      onChange: (value) => setStatus(value as StatusChoice),
      options: [
        { value: "all", label: "Alle Status" },
        ...CONTRACT_STATUS_ORDER.filter(
          (key) => (data?.list.counts[key] ?? 0) > 0 || key === status,
        ).map((key) => ({
          value: key,
          label: `${CONTRACT_STATUS_LABELS[key]} (${data?.list.counts[key] ?? 0})`,
        })),
      ],
    },
    ...(schools.length > 1
      ? [
          {
            id: "school",
            label: "Betreuungsort",
            type: "dropdown" as const,
            value: schoolId,
            onChange: (value: string | string[]) =>
              setSchoolId(value as string),
            options: [{ value: "", label: "Alle Schulen" }, ...schools],
          },
        ]
      : []),
  ];
  const activeFilters: ActiveFilter[] = [
    ...(status === "all"
      ? []
      : [
          {
            id: "status",
            label: CONTRACT_STATUS_LABELS[status],
            onRemove: () => setStatus("all"),
          },
        ]),
    ...(schoolId
      ? [
          {
            id: "school",
            label:
              schools.find((school) => school.value === schoolId)?.label ??
              "Ausgewählte Schule",
            onRemove: () => setSchoolId(""),
          },
        ]
      : []),
  ];

  const createLabel =
    candidates.length > 0
      ? `Verträge erstellen (${candidates.length})`
      : "Verträge erstellen";

  return (
    <TenantPage
      title="Verträge"
      info="Hier sehen Sie die Betreuungsverträge aus den angenommenen Anmeldungen Ihrer Schulen. moto übernimmt Kind, Eltern, Beginn und die gebuchten Module je Wochentag. Gilt der Vertrag bei der Annahme, entsteht er von selbst. Sonst prüfen Sie den Entwurf und senden ihn per Mail und in die Eltern-App."
      tabs={careContractTabs("vertraege")}
      stats={
        total > 0 || candidates.length > 0 ? (
          <TenantPageStats
            items={statusFigures(
              total,
              drafts,
              sentCount,
              activeCount,
              candidates.length,
            )}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      search={{
        value: search,
        onChange: setSearch,
        placeholder: "Kind, Eltern oder Angebot suchen",
      }}
      filters={filters}
      activeFilters={activeFilters}
      onClearAllFilters={() => {
        setStatus("all");
        setSchoolId("");
      }}
      actions={
        <>
          {selectedDrafts.length > 0 ? (
            <Button
              type="button"
              variant="success"
              size="md"
              disabled={sending}
              onClick={() => void sendSelected()}
            >
              <Send className="mr-2 h-4 w-4" aria-hidden="true" />
              {sending
                ? "Wird gesendet …"
                : selectedDrafts.length === 1
                  ? "1 Entwurf senden"
                  : `${selectedDrafts.length} Entwürfe senden`}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!data}
            onClick={() => setCreating(true)}
          >
            <Plus className="mr-2 h-4 w-4" aria-hidden="true" />
            {createLabel}
          </Button>
        </>
      }
      error={pageLoad.error ?? (!data ? afterWrite.error : null)}
      loading={!pageLoad.error && !afterWrite.error && !data}
      overlays={
        data ? (
          <CareContractCreateModal
            isOpen={creating}
            candidates={data.candidates}
            onClose={() => setCreating(false)}
            onCreated={(result) => void onCreated(result)}
            onOpenConditions={() =>
              router.push(traegerPath("/traeger/vertraege/bedingungen"))
            }
          />
        ) : null
      }
    >
      <LoadErrorAlert error={afterWrite.error} className="mb-4" />
      {data && data.modes.length > 0 ? (
        <SectionCard
          title="Wann gilt der Vertrag?"
          description={conclusionDescription(data.modes)}
          action={
            hasPermission(session, "traeger_overview:read") ? (
              <ButtonLink
                href={traegerPath("/traeger/einstellungen")}
                variant="surface"
                size="compact"
              >
                In den Einstellungen ändern
              </ButtonLink>
            ) : null
          }
        >
          <ul
            aria-label="Abschluss je Schule"
            className="divide-y divide-gray-100"
          >
            {data.modes.map((school) => (
              <li
                key={school.schoolId}
                className="flex flex-wrap items-center justify-between gap-3 py-2.5 first:pt-0 last:pb-0"
              >
                <span className="min-w-0 text-sm font-medium wrap-anywhere text-gray-900">
                  {school.schoolName}
                </span>
                <StatusBadge
                  label={CONCLUSION_MODE_LABELS[school.mode]}
                  tone={CONCLUSION_MODE_TONES[school.mode]}
                />
              </li>
            ))}
          </ul>
        </SectionCard>
      ) : null}
      {data ? (
        <DataTable
          columns={COLUMNS}
          rows={contracts}
          getRowKey={(contract) => contract.id}
          stackedOnMobile
          stackedLayout="row"
          onRowClick={(contract) => router.push(contractHref(contract))}
          selection={{
            selectedKeys: selected,
            disabled: sending,
            rowLabel: (contract) => `Vertrag von ${childName(contract)}`,
            onChange: (keys, on) =>
              setSelected((current) => {
                const next = new Set(current);
                for (const key of keys) {
                  if (on) next.add(key);
                  else next.delete(key);
                }
                return next;
              }),
          }}
          emptyState={
            <EmptyState
              icon={
                <MotoConceptIcon
                  concept={careContractsConcept}
                  className="h-12 w-12"
                />
              }
              {...emptyText(
                total,
                candidates.length,
                data.modes.length > 0 &&
                  data.modes.every((school) => school.mode === "bei_annahme"),
              )}
            />
          }
        />
      ) : null}
      {data?.list.hasMore ? (
        <Button
          type="button"
          variant="outline"
          disabled={loadingMore || sending}
          isLoading={loadingMore}
          loadingText="Wird geladen …"
          onClick={() => void load(contracts.length)}
        >
          Weitere Verträge laden
        </Button>
      ) : null}
      {data && selected.size > 0 && selectedDrafts.length < selected.size ? (
        <p className="text-sm text-gray-500">
          Senden gilt nur für Entwürfe. Einen versendeten Vertrag senden Sie auf
          seiner Seite erneut.
        </p>
      ) : null}
    </TenantPage>
  );
}

function emptyText(
  contracts: number,
  candidates: number,
  allOnAcceptance: boolean,
): { title: string; description?: string } {
  if (contracts > 0) return { title: "Kein Vertrag passt zu Ihrer Suche." };
  if (allOnAcceptance) {
    return {
      title: "Noch kein Vertrag.",
      description:
        "Sobald eine Schule eine Anmeldung annimmt, erscheint hier der Vertrag. Er gilt dann schon.",
    };
  }
  if (candidates > 0) {
    return {
      title: "Noch kein Vertrag.",
      description: `${candidates === 1 ? "1 angenommene Anmeldung wartet" : `${candidates} angenommene Anmeldungen warten`} auf einen Vertrag. Erstellen Sie die Entwürfe oben mit „Verträge erstellen“.`,
    };
  }
  return {
    title: "Noch kein Vertrag.",
    description:
      "Sobald eine Schule eine Anmeldung annimmt, können Sie hier den Vertrag erstellen.",
  };
}

/**
 * The figures of the status line: at most three fit (TenantPageStats). The
 * states that hold contracts come first, active before sent before drafts;
 * without any of them the total, then the children still without one.
 */
function statusFigures(
  total: number,
  drafts: number,
  sent: number,
  active: number,
  candidates: number,
): { value: number; label: string }[] {
  const figures = [
    { value: active, label: "aktiv" },
    { value: sent, label: "versendet" },
    { value: drafts, label: drafts === 1 ? "Entwurf" : "Entwürfe" },
  ].filter((figure) => figure.value > 0);
  if (figures.length < 2) {
    figures.unshift({
      value: total,
      label: total === 1 ? "Vertrag" : "Verträge",
    });
  }
  if (candidates > 0)
    figures.push({ value: candidates, label: "ohne Vertrag" });
  return figures.slice(0, 3);
}
