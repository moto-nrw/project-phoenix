"use client";

import { useMemo, useState } from "react";
import { useSession } from "next-auth/react";
import { Send } from "lucide-react";
import type {
  PhaseResponseChild,
  PhaseResponseExclusion,
  PhaseResponseOverview as PhaseResponseOverviewData,
} from "~/lib/enrollment-phase-api";
import type { ChildStatus } from "~/lib/enrollment-admin-api";
import { hasPermission } from "~/lib/auth-utils";
import { useSettingsSchema } from "~/lib/hooks/use-settings-schema";
import { getSettingValue } from "~/lib/settings-api";
import { useTenantRouter } from "~/lib/tenant-router";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import {
  ANNOUNCEMENT_PREFILL_PARAM,
  ANNOUNCEMENT_PREFILL_STUDENTS,
  ANNOUNCEMENT_PREFILL_TOKEN_PARAM,
  stashAnnouncementStudents,
} from "~/lib/announcement-prefill";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import NavigationLink from "~/components/ui/navigation-link";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  CHILD_STATUS_LABELS,
  ChildStatusBadge,
} from "~/components/enrollment/child-status-badge";

type ResponseView = "missing" | "responded";

const EXCLUSION_TEXT: Record<
  PhaseResponseExclusion["reason"],
  (count: number) => string
> = {
  care_ending: (count) =>
    `${count} ${count === 1 ? "Kind" : "Kinder"} mit eingetragenem Betreuungsende`,
  graduating: (count) =>
    `${count} ${count === 1 ? "Kind" : "Kinder"} im letzten Jahrgang`,
  not_in_scope: (count) =>
    `${count} ${count === 1 ? "Kind" : "Kinder"} aus anderen Klassen oder Jahrgängen`,
};

function isChildStatus(value: string | null): value is ChildStatus {
  return value !== null && value in CHILD_STATUS_LABELS;
}

function fullName(child: PhaseResponseChild): string {
  return `${child.firstName} ${child.lastName}`.trim();
}

/**
 * Rücklauf der bestehenden Kinder zu einer Anmeldephase (#3379): wer hat die
 * Anmeldung abgegeben, wer fehlt noch. „Abgegeben" heißt abgeschickt, nicht
 * bestätigt; der Bearbeitungsstand der Schule steht als eigener Status in der
 * Zeile. Die Eltern-App ist bewusst eine eigene Spalte und fließt nie in die
 * Zahl ein: Sie sagt nur, ob eine Mitteilung die Familie erreicht.
 */
export function PhaseResponseOverview({
  overview,
  search,
}: Readonly<{
  overview: PhaseResponseOverviewData;
  /** Suchtext der Kopfkarte; filtert nach Kind und Klasse. */
  search: string;
}>) {
  const [view, setView] = useState<ResponseView>("missing");
  const tenantPath = useTenantAwarePath();
  const router = useTenantRouter();
  const tenantSlug = useTenantSlugSafe();
  const { data: session } = useSession();

  // Dieselbe Bedingung wie der Eintrag „Mitteilungen" in der Seitenleiste:
  // Der Knopf führt nie auf eine Seite, die der Person fehlt.
  const canAnnounce = hasPermission(session, "admin:*");
  const { data: settingsSchema } = useSettingsSchema(canAnnounce, {
    revalidateOnFocus: false,
    revalidateOnReconnect: false,
    shouldRetryOnError: false,
  });
  const announcementsAvailable =
    canAnnounce &&
    getSettingValue(settingsSchema, "operations.parent_news_enabled") === true;

  const missing = useMemo(
    () => overview.children.filter((child) => !child.responded),
    [overview.children],
  );
  const responded = useMemo(
    () => overview.children.filter((child) => child.responded),
    [overview.children],
  );
  const reachable = useMemo(
    () => missing.filter((child) => child.hasParentApp),
    [missing],
  );
  const unreachableCount = missing.length - reachable.length;
  const hasNoCountedChildren =
    overview.expected === 0 &&
    overview.excluded.some((entry) => entry.count > 0);

  const rows = useMemo(() => {
    const term = search.trim().toLowerCase();
    const source = view === "missing" ? missing : responded;
    if (term === "") return source;
    return source.filter(
      (child) =>
        fullName(child).toLowerCase().includes(term) ||
        child.schoolClass.toLowerCase().includes(term),
    );
  }, [missing, responded, search, view]);

  const columns: DataTableColumn<PhaseResponseChild>[] = [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      sortValue: (child) =>
        `${child.lastName} ${child.firstName}`.toLowerCase(),
      render: (child) => (
        <NavigationLink
          href={tenantPath(`/students/${encodeURIComponent(child.studentId)}`)}
          className="font-medium text-gray-900 underline-offset-2 hover:underline"
        >
          {fullName(child)}
        </NavigationLink>
      ),
    },
    {
      key: "class",
      header: "Klasse",
      stacked: "meta",
      sortValue: (child) => child.schoolClass.toLowerCase(),
      render: (child) => child.schoolClass || "–",
    },
    {
      key: "status",
      header: "Anmeldung",
      render: (child) => {
        const requestId = child.requestId ?? child.pendingRequestId;
        return (
          <span className="inline-flex flex-wrap items-center gap-2">
            {isChildStatus(child.childStatus) ? (
              <ChildStatusBadge status={child.childStatus} />
            ) : (
              <StatusBadge label="Fehlt noch" tone="orange" />
            )}
            {requestId ? (
              <NavigationLink
                href={tenantPath(
                  `/admin/enrollments/${encodeURIComponent(requestId)}`,
                )}
                className="text-sm text-gray-600 underline underline-offset-2 hover:text-gray-900"
              >
                Ansehen
              </NavigationLink>
            ) : null}
          </span>
        );
      },
    },
    {
      key: "app",
      header: "Eltern-App",
      sortValue: (child) => (child.hasParentApp ? 0 : 1),
      render: (child) => (child.hasParentApp ? "Ja" : "Nein"),
    },
  ];

  const handleRemind = () => {
    const token = tenantSlug
      ? stashAnnouncementStudents(
          tenantSlug,
          reachable.map((child) => ({
            id: child.studentId,
            name: fullName(child),
          })),
        )
      : null;
    // Ohne Ablage öffnet sich die Mitteilungsliste; die Kinder lassen sich
    // dort von Hand wählen.
    router.push(
      token
        ? `/parent-announcements?${ANNOUNCEMENT_PREFILL_PARAM}=${ANNOUNCEMENT_PREFILL_STUDENTS}&${ANNOUNCEMENT_PREFILL_TOKEN_PARAM}=${encodeURIComponent(token)}`
        : "/parent-announcements",
    );
  };

  const exclusions = overview.excluded
    .map((entry) => EXCLUSION_TEXT[entry.reason](entry.count))
    .join(", ");

  return (
    <SectionCard
      title={`${overview.responded} von ${overview.expected} Kindern haben die Anmeldung abgegeben`}
      description={
        exclusions === "" ? undefined : `Nicht mitgezählt: ${exclusions}.`
      }
      action={
        // Nur bei „Fehlt noch": Neben „Abgegeben (3)" läse sich ein Knopf mit
        // einer Zahl als Mitteilung an die Familien, die schon geantwortet
        // haben. „erinnern" sagt, an wen es geht.
        view === "missing" && announcementsAvailable && reachable.length > 0 ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={handleRemind}
            className="inline-flex items-center gap-2"
          >
            <Send className="h-4 w-4" aria-hidden="true" />
            {reachable.length} {reachable.length === 1 ? "Familie" : "Familien"}{" "}
            erinnern
          </Button>
        ) : undefined
      }
    >
      <div className="flex flex-col gap-4">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <SegmentedControl<ResponseView>
            ariaLabel="Kinder nach Anmeldung"
            value={view}
            onChange={setView}
            items={[
              { value: "missing", label: `Fehlt noch (${missing.length})` },
              {
                value: "responded",
                label: `Abgegeben (${responded.length})`,
              },
            ]}
          />
          {view === "missing" && unreachableCount > 0 ? (
            <p className="text-sm text-gray-600">
              {unreachableCount}{" "}
              {unreachableCount === 1 ? "Familie hat" : "Familien haben"} keine
              Eltern-App. Bitte anrufen.
            </p>
          ) : null}
        </div>

        <DataTable
          columns={columns}
          rows={rows}
          getRowKey={(child) => child.studentId}
          defaultSortKey="child"
          defaultSortDirection="asc"
          stackedOnMobile
          emptyState={
            <ResponseEmptyState
              view={view}
              filtered={search.trim() !== ""}
              total={view === "missing" ? missing.length : responded.length}
              hasNoCountedChildren={hasNoCountedChildren}
            />
          }
        />
      </div>
    </SectionCard>
  );
}

function ResponseEmptyState({
  view,
  filtered,
  total,
  hasNoCountedChildren,
}: Readonly<{
  view: ResponseView;
  filtered: boolean;
  total: number;
  hasNoCountedChildren: boolean;
}>) {
  if (filtered && total > 0) {
    return (
      <EmptyState
        variant="compact"
        title="Kein Kind für diese Suche gefunden"
      />
    );
  }
  if (view === "missing") {
    if (hasNoCountedChildren) {
      return (
        <EmptyState
          variant="compact"
          title="Keine Kinder im Rücklauf"
          description="Alle Kinder werden nicht mitgezählt. Die Gründe stehen oben."
        />
      );
    }
    return (
      <EmptyState
        variant="compact"
        title="Alle Kinder sind angemeldet"
        description="Für jedes Kind liegt eine Anmeldung vor."
      />
    );
  }
  return (
    <EmptyState
      variant="compact"
      title="Noch keine Anmeldung abgegeben"
      description="Sobald Eltern die Anmeldung abschicken, erscheinen die Kinder hier."
    />
  );
}
