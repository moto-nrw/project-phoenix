"use client";

// Personalakten des Trägers über alle Schulen (#4705, ADR 0089): die Akten
// mit ihren Einsatzorten, die Vorschläge, welche Beschäftigungen dieselbe
// Person sind, und das Personal, das noch keine Akte hat. Ein Vorschlag oder
// eine Person öffnet den Dialog zum Verknüpfen; eine Akte ihre Seite.

import {
  IdentificationCardIcon,
  LinkSimpleIcon,
  LockKeyIcon,
} from "@phosphor-icons/react";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import { StaffLinkModal } from "~/components/traeger/staff/staff-link-modal";
import {
  STAFF_MATCH_LABELS,
  STAFF_MATCH_TONES,
} from "~/components/traeger/staff/staff-record-format";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { IconTile } from "~/components/ui/icon-tile";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { useApiLoadError, useToast } from "~/contexts/ToastContext";
import { peopleConcept } from "~/lib/concepts/people";
import { staffConcept } from "~/lib/concepts/staff";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  fetchStaffOverview,
  staffName,
  type StaffCandidate,
  type StaffLinkInput,
  type StaffOverview,
  type StaffRecordSummary,
  type StaffSuggestion,
} from "~/lib/traeger-staff-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerStaffRecordsPage" });

type Tab = "vorschlaege" | "akten" | "ohne-akte";

const INFO =
  "Hier führen Sie jede Person, die Ihr Träger beschäftigt, mit einer Akte über alle Schulen. moto schlägt vor, welche Beschäftigungen an verschiedenen Schulen dieselbe Person sind. Sie prüfen jeden Vorschlag und verknüpfen ihn. Die Akte führt die Personalnummer des Trägers und die Bank- und Steuerdaten. Die Schulen arbeiten weiter mit ihrem Personal wie bisher. Lehrkräfte des Landes und Kräfte anderer Träger bekommen keine Akte.";

const RECORD_COLUMNS: DataTableColumn<StaffRecordSummary>[] = [
  {
    key: "name",
    header: "Name",
    render: (record) => (
      <span className="block min-w-0">
        <span className="block truncate font-medium text-gray-900">
          {staffName(record)}
        </span>
        {record.birthday ? (
          <span className="block text-sm text-gray-500 tabular-nums">
            geboren am {formatDate(record.birthday)}
          </span>
        ) : null}
      </span>
    ),
    sortValue: (record) =>
      `${record.lastName} ${record.firstName}`.toLowerCase(),
    stacked: "title",
  },
  {
    key: "number",
    header: "Personalnummer",
    render: (record) => (
      <span className="font-mono text-sm text-gray-700 tabular-nums">
        {record.personnelNumber ?? "–"}
      </span>
    ),
    sortValue: (record) => record.personnelNumber ?? "",
    stacked: "field",
  },
  {
    key: "employments",
    header: "Einsatzorte",
    render: (record) => (
      <span className="flex max-w-full min-w-0 flex-wrap gap-1.5">
        {record.employments.map((employment) => (
          <StatusBadge
            key={`${employment.schoolId}-${employment.staffId}`}
            label={
              employment.active
                ? employment.schoolName
                : `${employment.schoolName} (beendet)`
            }
            tone={employment.active ? "outline" : "gray"}
            wrap
          />
        ))}
      </span>
    ),
    sortValue: (record) => record.employments.length,
    stacked: "field",
  },
  {
    key: "financial",
    header: "Bank und Steuer",
    render: (record) =>
      record.hasFinancial ? (
        <StatusBadge label="Hinterlegt" tone="green" />
      ) : (
        <span className="text-sm text-gray-500">–</span>
      ),
    sortValue: (record) => (record.hasFinancial ? 1 : 0),
    stacked: "hidden",
  },
];

function suggestionColumns(
  records: ReadonlyMap<string, StaffRecordSummary>,
): DataTableColumn<StaffSuggestion>[] {
  return [
    {
      key: "people",
      header: "Wer",
      render: (suggestion) => {
        const record = suggestion.recordId
          ? records.get(suggestion.recordId)
          : undefined;
        return (
          <span className="block min-w-0 space-y-0.5">
            {record ? (
              <span className="block truncate font-medium text-gray-900">
                Akte {staffName(record)}
              </span>
            ) : null}
            {suggestion.members.map((member) => (
              <span
                key={`${member.schoolId}-${member.staffId}`}
                className="block truncate text-sm text-gray-700"
              >
                <span className="font-medium text-gray-900">
                  {staffName(member)}
                </span>
                {" · "}
                {member.schoolName}
              </span>
            ))}
          </span>
        );
      },
      stacked: "title",
    },
    {
      key: "reason",
      header: "Warum",
      render: (suggestion) => (
        <span className="flex flex-wrap gap-1.5">
          <StatusBadge
            label={STAFF_MATCH_LABELS[suggestion.reason]}
            tone={STAFF_MATCH_TONES[suggestion.reason]}
          />
          {suggestion.recordId ? (
            <StatusBadge label="Passt zu einer Akte" tone="purple" />
          ) : null}
        </span>
      ),
      stacked: "field",
    },
  ];
}

const CANDIDATE_COLUMNS: DataTableColumn<StaffCandidate>[] = [
  {
    key: "name",
    header: "Name",
    render: (candidate) => (
      <span className="block truncate font-medium text-gray-900">
        {staffName(candidate)}
      </span>
    ),
    sortValue: (candidate) =>
      `${candidate.lastName} ${candidate.firstName}`.toLowerCase(),
    stacked: "title",
  },
  {
    key: "school",
    header: "Schule",
    render: (candidate) => (
      <span className="text-sm text-gray-700">{candidate.schoolName}</span>
    ),
    sortValue: (candidate) => candidate.schoolName,
    stacked: "field",
  },
  {
    key: "number",
    header: "Personalnummer",
    render: (candidate) => (
      <span className="font-mono text-sm text-gray-700 tabular-nums">
        {candidate.personnelNumber ?? "–"}
      </span>
    ),
    sortValue: (candidate) => candidate.personnelNumber ?? "",
    stacked: "hidden",
  },
];

const linkRequestOf = (suggestion: StaffSuggestion) => ({
  recordId: suggestion.recordId,
  members: suggestion.members.map((member) => ({
    schoolId: member.schoolId,
    staffId: member.staffId,
  })),
});

export default function TraegerStaffRecordsPage() {
  const router = useRouter();
  const { success } = useToast();
  const [overview, setOverview] = useState<StaffOverview | null>(null);
  const [tab, setTab] = useState<Tab | null>(null);
  const [linking, setLinking] = useState<Omit<
    StaffLinkInput,
    "choices"
  > | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    try {
      setOverview(await fetchStaffOverview());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_staff_records_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Personalakten",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const records = useMemo(
    () =>
      new Map((overview?.records ?? []).map((record) => [record.id, record])),
    [overview],
  );
  const proposals = useMemo(
    () =>
      (overview?.suggestions ?? []).filter(
        (suggestion) =>
          suggestion.members.length > 1 || suggestion.recordId !== null,
      ),
    [overview],
  );
  const singles = useMemo(
    () =>
      (overview?.suggestions ?? [])
        .filter(
          (suggestion) =>
            suggestion.members.length === 1 && suggestion.recordId === null,
        )
        .flatMap((suggestion) => suggestion.members),
    [overview],
  );
  const columns = useMemo(() => suggestionColumns(records), [records]);
  const released = overview?.release != null;
  const active: Tab = tab ?? (proposals.length > 0 ? "vorschlaege" : "akten");

  return (
    <TenantPage
      title="Personalakten"
      info={INFO}
      stats={
        overview && released ? (
          <TenantPageStats
            items={[
              {
                value: overview.records.length,
                label: overview.records.length === 1 ? "Akte" : "Akten",
              },
              {
                value: proposals.length,
                label: proposals.length === 1 ? "Vorschlag" : "Vorschläge",
              },
              {
                value: overview.schools.length,
                label: overview.schools.length === 1 ? "Schule" : "Schulen",
              },
            ]}
          />
        ) : null
      }
      statsLoading={!overview && !pageLoad.error}
      error={pageLoad.error}
      loading={!overview && !pageLoad.error}
      tabs={
        released
          ? {
              value: active,
              onChange: (value) => setTab(value as Tab),
              label: "Bereiche der Personalakten",
              items: [
                {
                  value: "vorschlaege",
                  label: "Vorschläge",
                  badge: proposals.length,
                },
                { value: "akten", label: "Akten" },
                { value: "ohne-akte", label: "Ohne Akte" },
              ],
            }
          : undefined
      }
      overlays={
        <StaffLinkModal
          request={linking}
          onClose={() => setLinking(null)}
          onLinked={(record) => {
            setLinking(null);
            success(`Die Akte von ${staffName(record)} ist verknüpft.`);
            void load();
            router.push(traegerPath(`/traeger/personal/${record.id}`));
          }}
        />
      }
    >
      {overview && !released ? (
        <SectionCard>
          <EmptyState
            icon={<LockKeyIcon className="h-12 w-12" aria-hidden="true" />}
            title="Personal beim Träger ist noch nicht freigeschaltet."
            description="Das moto-Team schaltet es frei, sobald Ihr Träger bestätigt hat: Es gibt eine Vereinbarung mit Betriebsrat oder Mitarbeitervertretung, oder es gibt keine Vertretung. Bis dahin führt jede Schule ihr Personal selbst."
          />
        </SectionCard>
      ) : null}

      {overview && released && active === "vorschlaege" ? (
        <SectionCard
          leading={
            <IconTile
              size="section"
              tone="neutral"
              icon={<LinkSimpleIcon className="h-5 w-5" aria-hidden="true" />}
            />
          }
          title="Vorschläge zum Verknüpfen"
          description="moto hat diese Beschäftigungen gefunden, die wohl dieselbe Person sind. Öffnen Sie einen Vorschlag, prüfen Sie die Werte und legen Sie die Akte an."
        >
          <DataTable
            columns={columns}
            rows={[...proposals]}
            getRowKey={(suggestion) =>
              suggestion.members
                .map((member) => `${member.schoolId}-${member.staffId}`)
                .join("+")
            }
            stackedOnMobile
            onRowClick={(suggestion) => setLinking(linkRequestOf(suggestion))}
            emptyState={
              <EmptyState
                icon={
                  <LinkSimpleIcon className="h-12 w-12" aria-hidden="true" />
                }
                title="Kein offener Vorschlag."
                description="Sobald dieselbe Person an zwei Schulen arbeitet, steht der Vorschlag hier."
              />
            }
          />
        </SectionCard>
      ) : null}

      {overview && released && active === "akten" ? (
        <SectionCard
          concept={staffConcept}
          title="Akten"
          description="Jede Akte gilt für alle Schulen. Öffnen Sie eine Akte für Personalnummer, Bank und Steuer und den Verlauf."
        >
          <DataTable
            columns={RECORD_COLUMNS}
            rows={[...overview.records]}
            getRowKey={(record) => record.id}
            stackedOnMobile
            onRowClick={(record) =>
              router.push(traegerPath(`/traeger/personal/${record.id}`))
            }
            emptyState={
              <EmptyState
                icon={
                  <IdentificationCardIcon
                    className="h-12 w-12"
                    aria-hidden="true"
                  />
                }
                title="Noch keine Akte."
                description="Verknüpfen Sie einen Vorschlag, oder legen Sie unter „Ohne Akte“ eine Akte für eine Person an."
              />
            }
          />
        </SectionCard>
      ) : null}

      {overview && released && active === "ohne-akte" ? (
        <SectionCard
          concept={peopleConcept}
          title="Personal ohne Akte"
          description="Diese Personen arbeiten an einer Schule und haben noch keine Akte. Öffnen Sie eine Person, um ihre Akte anzulegen."
        >
          <DataTable
            columns={CANDIDATE_COLUMNS}
            rows={[...singles]}
            getRowKey={(candidate) =>
              `${candidate.schoolId}-${candidate.staffId}`
            }
            stackedOnMobile
            onRowClick={(candidate) =>
              setLinking({
                recordId: null,
                members: [
                  { schoolId: candidate.schoolId, staffId: candidate.staffId },
                ],
              })
            }
            emptyState={
              <EmptyState
                title="Alle Personen haben eine Akte."
                description="Neues Personal einer Schule erscheint hier."
              />
            }
          />
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
