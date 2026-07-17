"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  CalendarRange,
  Check,
  ChevronDown,
  ClipboardList,
  Eye,
  ExternalLink,
  FileText,
  LockKeyhole,
  MessageSquare,
  type LucideIcon,
  Settings2,
} from "lucide-react";
import {
  type AdminEnrollmentChangeRequest,
  type AdminRequestSummary,
  type ChildStatus,
  listAdminEnrollmentChangeRequests,
  listAdminRequests,
} from "~/lib/enrollment-admin-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { listCareOfferings } from "~/lib/care-offering-api";
import {
  latestSchemasByName,
  listSchemas,
  type FormSchema,
} from "~/lib/enrollment-form-schema-api";
import { fetchSettingsSchema } from "~/lib/settings-api";
import { Alert } from "~/components/ui/alert";
import { DataTableStatusBadge } from "~/components/ui/data-table";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { useEnrollmentPublicUrl } from "~/lib/enrollment-public-url";
import { useTenantAwarePath } from "~/lib/tenant-path";
import { PublicLinkCopyButton } from "~/components/enrollment/public-link-copy-button";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentsList" });

type SetupStepStatus = "done" | "todo" | "blocked";

interface SetupStep {
  readonly title: string;
  readonly description: string;
  readonly href: string;
  readonly action: string;
  readonly status: SetupStepStatus;
  readonly meta: string;
  readonly icon: LucideIcon;
  readonly requiredForPublish: boolean;
}

interface CareOfferingStats {
  readonly total: number;
  readonly activeInActivePhases: number;
}

export function AdminEnrollmentsList() {
  const tenantSlug = useTenantSlugSafe();
  const tenantPath = useTenantAwarePath();
  const [phases, setPhases] = useState<Phase[]>([]);
  const [schemas, setSchemas] = useState<FormSchema[]>([]);
  const [careOfferingStats, setCareOfferingStats] = useState<CareOfferingStats>(
    { total: 0, activeInActivePhases: 0 },
  );
  const [enrollmentEnabled, setEnrollmentEnabled] = useState<boolean | null>(
    null,
  );
  const [allRequests, setAllRequests] = useState<AdminRequestSummary[]>([]);
  const [changeRequests, setChangeRequests] = useState<
    AdminEnrollmentChangeRequest[]
  >([]);
  const [changeRequestsError, setChangeRequestsError] = useState<string | null>(
    null,
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const latestSchemas = useMemo(() => latestSchemasByName(schemas), [schemas]);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      setChangeRequestsError(null);
      try {
        const [
          phasesData,
          allRequestsData,
          changeRequestsResult,
          schemasData,
          settingsData,
        ] = await Promise.all([
          listPhases(),
          listAdminRequests(),
          listAdminEnrollmentChangeRequests()
            .then((data) => ({ data, error: null as string | null }))
            .catch((err) => {
              const message =
                err instanceof Error ? err.message : "Unbekannter Fehler";
              logger.error("admin_change_requests_load_failed", {
                error: message,
              });
              return {
                data: [] as AdminEnrollmentChangeRequest[],
                error: message,
              };
            }),
          listSchemas().catch(() => [] as FormSchema[]),
          fetchSettingsSchema().catch(() => null),
        ]);
        const offeringLists = await Promise.all(
          phasesData.map((phase) =>
            listCareOfferings(phase.id).catch(() => []),
          ),
        );
        if (cancelled) return;
        setPhases(phasesData);
        setAllRequests(allRequestsData);
        setChangeRequests(changeRequestsResult.data);
        setChangeRequestsError(changeRequestsResult.error);
        setSchemas(schemasData);
        const activePhaseIds = new Set(
          phasesData
            .filter((phase) => phase.is_active)
            .map((phase) => phase.id),
        );
        const careOfferings = offeringLists.flat();
        setCareOfferingStats({
          total: careOfferings.length,
          activeInActivePhases: careOfferings.filter(
            (offering) =>
              offering.is_active && activePhaseIds.has(offering.phase_id),
          ).length,
        });
        setEnrollmentEnabled(readEnrollmentEnabled(settingsData));
      } catch (err) {
        if (cancelled) return;
        const message =
          err instanceof Error ? err.message : "Unbekannter Fehler";
        logger.error("admin_enrollments_load_failed", { error: message });
        setError(message);
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void load();
    return () => {
      cancelled = true;
    };
  }, []);

  if (loading) {
    return (
      <p className="text-sm text-gray-500">Anmeldungen werden geladen...</p>
    );
  }

  return (
    <div className="mt-4 space-y-4">
      <EnrollmentSetupGuide
        enrollmentEnabled={enrollmentEnabled}
        phaseCount={phases.length}
        activePhaseCount={phases.filter((phase) => phase.is_active).length}
        activePhaseUsingBaseFormCount={
          phases.filter((phase) => phase.is_active && !phase.form_schema_id)
            .length
        }
        activePhaseUsingCustomFormCount={
          phases.filter((phase) => phase.is_active && phase.form_schema_id)
            .length
        }
        schemaCount={latestSchemas.length}
        careOfferingCount={careOfferingStats.total}
        activeCareOfferingCount={careOfferingStats.activeInActivePhases}
        requestCount={allRequests.length}
      />

      <ChangeRequestsOverview
        error={changeRequestsError}
        requests={changeRequests}
        tenantPath={tenantPath}
      />

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}

      <EnrollmentPhaseOverview
        phases={phases}
        requests={allRequests}
        tenantSlug={tenantSlug}
        tenantPath={tenantPath}
      />
    </div>
  );
}

function ChangeRequestsOverview({
  error,
  requests,
  tenantPath,
}: Readonly<{
  error: string | null;
  requests: AdminEnrollmentChangeRequest[];
  tenantPath: (path: string) => string;
}>) {
  const pending = requests.filter((row) => row.status === "pending_review");
  const waitingForParent = requests.filter(
    (row) => row.status === "needs_parent_response",
  );
  const openCount = pending.length + waitingForParent.length;

  return (
    <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
        <div className="flex gap-3">
          <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-gray-50 text-gray-600 shadow-sm">
            <MessageSquare className="h-5 w-5" aria-hidden="true" />
          </span>
          <div>
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Änderungen
            </p>
            <h2 className="mt-1 text-base font-semibold text-gray-900">
              Änderungsanfragen
            </h2>
            <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
              Familien können nach einer Entscheidung Korrekturen einreichen.
              Offene Anfragen prüfst du gesammelt in der Review-Ansicht.
            </p>
          </div>
        </div>
        <Link
          href={tenantPath("/admin/enrollments/change-requests")}
          className="inline-flex h-9 w-full shrink-0 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:w-auto"
        >
          Anfragen prüfen
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Link>
      </div>
      {error ? (
        <div className="mt-4">
          <Alert
            type="error"
            message="Änderungsanfragen konnten nicht geladen werden. Die Zahlen sind unbekannt."
          />
        </div>
      ) : (
        <div className="mt-4 grid gap-2 sm:grid-cols-3">
          <PhaseStat label="Offen" value={pending.length} />
          <PhaseStat label="Rückfragen" value={waitingForParent.length} />
          <PhaseStat label="Gesamt" value={requests.length} />
        </div>
      )}
      {!error && openCount === 0 ? (
        <p className="mt-3 text-sm text-gray-500">
          Aktuell wartet keine Änderungsanfrage auf Bearbeitung.
        </p>
      ) : null}
    </section>
  );
}

function readEnrollmentEnabled(
  settings: Awaited<ReturnType<typeof fetchSettingsSchema>>,
): boolean | null {
  if (!settings) return null;
  for (const tab of settings.tabs) {
    if (tab.key !== "enrollment") continue;
    for (const category of tab.categories) {
      for (const item of category.items) {
        if (item.key === "enrollment.enabled") {
          return item.value === true;
        }
      }
    }
  }
  return null;
}

function formatPhaseDate(value: string): string {
  return new Date(`${value}T00:00:00`).toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

const OPEN_STATUSES = new Set<ChildStatus>([
  "submitted",
  "under_review",
  "pending_admin_review",
]);

function EnrollmentPhaseOverview({
  phases,
  requests,
  tenantSlug,
  tenantPath,
}: Readonly<{
  phases: Phase[];
  requests: AdminRequestSummary[];
  tenantSlug: string | null;
  tenantPath: (path: string) => string;
}>) {
  const phaseStats = useMemo(() => {
    const stats = new Map<
      string,
      { total: number; open: number; approved: number; rejected: number }
    >();
    for (const phase of phases) {
      stats.set(phase.id, { total: 0, open: 0, approved: 0, rejected: 0 });
    }
    for (const request of requests) {
      const entry = stats.get(request.phase_id);
      if (!entry) continue;
      for (const child of request.children) {
        entry.total += 1;
        if (OPEN_STATUSES.has(child.status)) entry.open += 1;
        if (child.status === "approved") entry.approved += 1;
        if (child.status === "rejected") entry.rejected += 1;
      }
    }
    return stats;
  }, [phases, requests]);

  const sortedPhases = useMemo(
    () =>
      [...phases].sort((a, b) => {
        if (a.is_active !== b.is_active) return a.is_active ? -1 : 1;
        return b.service_start_date.localeCompare(a.service_start_date);
      }),
    [phases],
  );

  if (phases.length === 0) {
    return (
      <section className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md">
        <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
          Überblick
        </p>
        <h2 className="mt-1 text-base font-semibold text-gray-900">
          Noch keine Anmeldephase
        </h2>
        <p className="mt-2 text-sm leading-6 text-gray-600">
          Lege zuerst eine Anmeldephase an. Danach siehst du hier, welche Phasen
          laufen und wie viele Anmeldungen eingegangen sind.
        </p>
        <Link
          href="/enrollment-phases"
          className="mt-4 inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          Anmeldephase anlegen
        </Link>
      </section>
    );
  }

  return (
    <section
      id="enrollment-phase-overview"
      className="moto-content-surface rounded-2xl border p-5 shadow-sm backdrop-blur-md"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
            Überblick
          </p>
          <h2 className="mt-1 text-base font-semibold text-gray-900">
            Anmeldephasen und Eingänge
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-gray-600">
            Wähle eine Phase aus, um die eingegangenen Anmeldungen zu prüfen und
            zu bearbeiten.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <a
            href="/enroll"
            target="_blank"
            rel="noreferrer"
            className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Elternansicht öffnen
            <ExternalLink className="h-4 w-4" aria-hidden="true" />
          </a>
          <Link
            href="/enrollment-phases"
            className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            Anmeldephasen verwalten
          </Link>
        </div>
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-2">
        {sortedPhases.map((phase) => {
          const stats = phaseStats.get(phase.id) ?? {
            total: 0,
            open: 0,
            approved: 0,
            rejected: 0,
          };
          return (
            <article
              key={phase.id}
              className="rounded-2xl border border-gray-200 bg-white p-4 shadow-sm"
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex flex-wrap items-center gap-2">
                    <h3 className="truncate text-sm font-semibold text-gray-900">
                      {phase.name}
                    </h3>
                    <DataTableStatusBadge active={phase.is_active} />
                  </div>
                  <p className="mt-1 text-xs text-gray-500">
                    {formatPhaseDate(phase.service_start_date)} bis{" "}
                    {formatPhaseDate(phase.service_end_date)}
                  </p>
                </div>
                <div className="flex shrink-0 flex-wrap justify-end gap-2">
                  <PhasePublicLinkActions
                    phase={phase}
                    tenantSlug={tenantSlug}
                    enrollmentsHref={tenantPath(
                      `/admin/enrollments/phases/${encodeURIComponent(phase.id)}`,
                    )}
                  />
                </div>
              </div>
              <div className="mt-4 grid grid-cols-4 gap-2">
                <PhaseStat label="Gesamt" value={stats.total} />
                <PhaseStat label="Offen" value={stats.open} />
                <PhaseStat label="Bestätigt" value={stats.approved} />
                <PhaseStat label="Abgelehnt" value={stats.rejected} />
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}

function PhasePublicLinkActions({
  phase,
  tenantSlug,
  enrollmentsHref,
}: Readonly<{
  phase: Phase;
  tenantSlug: string | null;
  enrollmentsHref: string;
}>) {
  const phaseUrl = useEnrollmentPublicUrl({ tenantSlug, phaseId: phase.id });

  return (
    <>
      <a
        href={`/enroll/${encodeURIComponent(phase.id)}`}
        target="_blank"
        rel="noreferrer"
        className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        Formular ansehen
        <ExternalLink className="h-3.5 w-3.5" aria-hidden="true" />
      </a>
      <Link
        href={enrollmentsHref}
        className="inline-flex h-8 items-center justify-center rounded-lg bg-gray-900 px-3 text-xs font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        Anmeldungen ansehen
      </Link>
      {phaseUrl ? (
        <PublicLinkCopyButton
          url={phaseUrl}
          componentId={`AdminEnrollmentsList:${phase.id}`}
        />
      ) : null}
    </>
  );
}

function PhaseStat({
  label,
  value,
}: Readonly<{ label: string; value: number }>) {
  return (
    <div className="rounded-xl bg-gray-50 px-3 py-2">
      <span className="block text-sm font-semibold text-gray-900">{value}</span>
      <span className="block text-[11px] font-medium text-gray-500">
        {label}
      </span>
    </div>
  );
}

interface EnrollmentSetupGuideProps {
  readonly enrollmentEnabled: boolean | null;
  readonly phaseCount: number;
  readonly activePhaseCount: number;
  readonly activePhaseUsingBaseFormCount: number;
  readonly activePhaseUsingCustomFormCount: number;
  readonly schemaCount: number;
  readonly careOfferingCount: number;
  readonly activeCareOfferingCount: number;
  readonly requestCount: number;
}

function EnrollmentSetupGuide({
  enrollmentEnabled,
  phaseCount,
  activePhaseCount,
  activePhaseUsingBaseFormCount,
  activePhaseUsingCustomFormCount,
  schemaCount,
  careOfferingCount,
  activeCareOfferingCount,
  requestCount,
}: EnrollmentSetupGuideProps) {
  const readyForPreview =
    enrollmentEnabled === true &&
    activePhaseCount > 0 &&
    activeCareOfferingCount > 0;
  const setupComplete = readyForPreview;
  const [expanded, setExpanded] = useState(!setupComplete);

  useEffect(() => {
    if (!setupComplete) setExpanded(true);
  }, [setupComplete]);

  const activePhaseWithResolvedFormCount =
    activePhaseUsingBaseFormCount + activePhaseUsingCustomFormCount;
  const formStepStatus: SetupStepStatus =
    activePhaseCount === 0
      ? "blocked"
      : activePhaseWithResolvedFormCount > 0
        ? "done"
        : "todo";
  const formStepMeta =
    activePhaseCount === 0
      ? "Wartet auf Phase"
      : activePhaseUsingCustomFormCount > 0
        ? "Eigene Vorlage"
        : activePhaseUsingBaseFormCount > 0
          ? "Basisformular"
          : schemaCount > 0
            ? "In Phase auswählen"
            : "Basisformular wählen";
  const formStepAction =
    activePhaseCount === 0 ? "Anmeldephase anlegen" : "In Phase festlegen";

  const steps = [
    {
      title: "Online-Anmeldung aktivieren",
      description:
        "Schaltet den Elternlink frei und zeigt ihn in den Settings.",
      href: "/settings?tab=enrollment&highlight=enrollment.enabled",
      action: enrollmentEnabled ? "Einstellungen prüfen" : "Aktivieren",
      status: enrollmentEnabled ? "done" : "todo",
      meta: enrollmentEnabled ? "Aktiv" : "Nicht aktiv",
      icon: Settings2,
      requiredForPublish: true,
    },
    {
      title: "Anmeldephase anlegen",
      description: "Zum Beispiel ein Halbjahr oder Schuljahr mit Anmeldefrist.",
      href: "/enrollment-phases",
      action: phaseCount > 0 ? "Anmeldephasen prüfen" : "Anlegen",
      status: activePhaseCount > 0 ? "done" : "todo",
      meta:
        phaseCount === 0
          ? "Keine angelegt"
          : activePhaseCount === 0
            ? "Keine aktiv"
            : `${activePhaseCount} aktiv`,
      icon: CalendarRange,
      requiredForPublish: true,
    },
    {
      title: "Betreuungsangebote ergänzen",
      description: "Eltern wählen daraus die passende Betreuung je Kind.",
      href: "/care-offerings",
      action:
        careOfferingCount > 0
          ? "Betreuungsangebote prüfen"
          : "Betreuungsangebot anlegen",
      status: activeCareOfferingCount > 0 ? "done" : "todo",
      meta:
        activePhaseCount === 0
          ? "Wartet auf Phase"
          : activeCareOfferingCount === 0
            ? "Keine aktiv"
            : `${activeCareOfferingCount} aktiv`,
      icon: ClipboardList,
      requiredForPublish: true,
    },
    {
      title: "Anmeldeformular festlegen",
      description:
        "Wähle in der Anmeldephase das Basisformular oder eine eigene Vorlage aus.",
      href: activePhaseCount === 0 ? "/enrollment-phases" : "/enrollment-form",
      action: formStepAction,
      status: formStepStatus,
      meta: formStepMeta,
      icon: FileText,
      requiredForPublish: false,
    },
    {
      title: "Elternansicht testen",
      description:
        "Öffne die öffentliche Anmeldung und sende eine Testanmeldung ab.",
      href: "/enroll",
      action: "Preview öffnen",
      status: readyForPreview ? "done" : "blocked",
      meta: readyForPreview ? "Bereit" : "Nicht bereit",
      icon: Eye,
      requiredForPublish: false,
    },
  ] satisfies SetupStep[];

  const completedSteps = steps.filter((step) =>
    isStepComplete(step.status),
  ).length;
  const progressPercent = Math.round((completedSteps / steps.length) * 100);
  const setupSummary = `${completedSteps} von ${steps.length} Schritten abgeschlossen`;

  const nextActionStep =
    steps.find((step) => step.status === "todo") ??
    steps.find((step) => step.status === "blocked") ??
    steps[steps.length - 1];
  const contentExpanded = !setupComplete || expanded;

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      {setupComplete && (
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          aria-expanded={expanded}
          className="group flex w-full items-center justify-between gap-4 px-5 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <span className="flex min-w-0 items-center gap-3.5">
            <span
              className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#83CD2D]"
              aria-hidden="true"
            />
            <span className="min-w-0">
              <span className="block text-xs font-medium tracking-wide text-gray-500 uppercase">
                Einrichtung
              </span>
              <span className="mt-0.5 block text-base font-semibold text-gray-900">
                Online-Anmeldung eingerichtet
              </span>
              <span className="mt-0.5 block text-sm text-gray-500">
                {setupSummary}. Für neue Zeiträume arbeitest du unten mit
                Anmeldephasen.
              </span>
            </span>
          </span>
          <span
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-gray-400 transition-colors group-hover:bg-gray-100 group-hover:text-gray-700"
            aria-hidden="true"
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
              aria-hidden="true"
            />
          </span>
        </button>
      )}
      <div
        className="grid transition-[grid-template-rows] duration-200 ease-out motion-reduce:transition-none"
        style={{ gridTemplateRows: contentExpanded ? "1fr" : "0fr" }}
      >
        <div className="min-h-0 overflow-hidden">
          <div
            className={`h-px bg-gray-100 transition-opacity duration-150 ${setupComplete && expanded ? "opacity-100" : "opacity-0"}`}
            aria-hidden="true"
          />
          <div className="grid lg:grid-cols-[minmax(0,1fr)_20rem]">
            <div className="p-4 sm:p-5">
              <div className="border-b border-gray-100 pb-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                      Einrichtung
                    </p>
                    <h2 className="mt-1 text-base font-semibold text-gray-900">
                      {setupComplete
                        ? "Einrichtung abgeschlossen"
                        : "Online-Anmeldung vorbereiten"}
                    </h2>
                    <p className="mt-1 max-w-2xl text-sm text-gray-600">
                      {setupComplete
                        ? "Die grundlegende Online-Anmeldung ist eingerichtet. Für neue Halbjahre, Ferienbetreuung oder andere Zeiträume legst du unten eine neue Anmeldephase an oder bearbeitest eine bestehende."
                        : "Starte hier, wenn du neue Halbjahresanmeldungen, Ferienbetreuung oder andere Anmeldezeiträume konfigurieren möchtest. Das Basisformular ist immer vorhanden."}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2 text-xs">
                    <StatusPill
                      label={
                        readyForPreview ? "Eingerichtet" : "In Vorbereitung"
                      }
                      tone={readyForPreview ? "success" : "neutral"}
                    />
                    <StatusPill
                      label={`${requestCount} Anmeldungen`}
                      tone={requestCount > 0 ? "info" : "neutral"}
                    />
                  </div>
                </div>
                <div className="mt-4">
                  <div className="flex items-center justify-between gap-3 text-xs text-gray-500">
                    <span>
                      {completedSteps} von {steps.length} Schritten erledigt
                    </span>
                    <span>{progressPercent}%</span>
                  </div>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100">
                    <div
                      className="h-full rounded-full bg-gray-900 transition-[width]"
                      style={{ width: `${progressPercent}%` }}
                    />
                  </div>
                </div>
              </div>

              <ol className="mt-2 divide-y divide-gray-100">
                {steps.map((step) => {
                  const StepIcon = step.icon;
                  return (
                    <li key={step.title}>
                      <Link
                        href={step.href}
                        className="group grid gap-3 py-3 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:grid-cols-[2.25rem_1fr_auto] sm:items-center sm:px-2"
                      >
                        <span
                          className={`flex h-9 w-9 items-center justify-center rounded-full border ${getStepIconClass(step.status)}`}
                          aria-hidden="true"
                        >
                          {isStepComplete(step.status) ? (
                            <Check className="h-4 w-4" />
                          ) : step.status === "blocked" ? (
                            <LockKeyhole className="h-4 w-4" />
                          ) : (
                            <StepIcon className="h-4 w-4" />
                          )}
                        </span>
                        <span className="min-w-0">
                          <span className="block text-sm font-medium text-gray-900">
                            {step.title}
                          </span>
                          <span className="mt-0.5 block text-sm text-gray-600">
                            {step.description}
                          </span>
                        </span>
                        <span className="flex flex-wrap items-center gap-2 sm:justify-end">
                          <StepMeta label={step.meta} status={step.status} />
                          <span className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-700 transition-colors group-hover:bg-gray-100">
                            {step.action}
                            <ArrowRight
                              className="h-3 w-3"
                              aria-hidden="true"
                            />
                          </span>
                        </span>
                      </Link>
                    </li>
                  );
                })}
              </ol>
            </div>

            <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-4 sm:p-5 lg:border-t-0 lg:border-l">
              <div className="relative z-10 space-y-4">
                <div>
                  <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    Startpunkt
                  </p>
                  <h3 className="mt-1 text-base font-semibold text-gray-900">
                    {setupComplete
                      ? "Mit Anmeldephasen arbeiten"
                      : nextActionStep?.status === "blocked"
                        ? "Voraussetzungen fehlen"
                        : nextActionStep?.title}
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    {setupComplete
                      ? "Die Einrichtung bleibt als Referenz erhalten. Der laufende Alltag passiert unten in der Phasenübersicht."
                      : nextActionStep?.status === "blocked"
                        ? "Lege zuerst eine aktive Anmeldephase und ein Betreuungsangebot an."
                        : nextActionStep?.description}
                  </p>
                </div>

                {setupComplete ? (
                  <a
                    href="#enrollment-phase-overview"
                    className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    Zur Phasenübersicht
                  </a>
                ) : nextActionStep ? (
                  <Link
                    href={nextActionStep.href}
                    className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    {nextActionStep.status === "blocked"
                      ? "Setup prüfen"
                      : nextActionStep.action}
                  </Link>
                ) : null}

                <div className="moto-content-surface rounded-xl border p-3">
                  <p className="text-xs font-semibold text-gray-900">
                    Prozessstatus
                  </p>
                  <div className="mt-3 space-y-2">
                    {steps.map((step) => (
                      <Link
                        key={step.title}
                        href={step.href}
                        className="flex items-center justify-between gap-3 rounded-lg px-2 py-1.5 text-xs transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                      >
                        <span className="min-w-0 truncate text-gray-700">
                          {step.title}
                        </span>
                        <span
                          className={`h-2.5 w-2.5 shrink-0 rounded-full ${getStepDotClass(step.status)}`}
                          aria-hidden="true"
                        />
                      </Link>
                    ))}
                  </div>
                </div>

                <div className="text-xs leading-relaxed text-gray-500">
                  Für den Elternlink sind Aktivierung, Anmeldephase und
                  Betreuungsangebote entscheidend. Jede Anmeldephase nutzt
                  entweder das Basisformular oder eine eigene Vorlage. Lege
                  zuerst eine Phase an und prüfe dort die Formularauswahl.
                </div>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </section>
  );
}

function isStepComplete(status: SetupStepStatus) {
  return status === "done";
}

function getStepIconClass(status: SetupStepStatus) {
  if (status === "done") {
    return "border-[#83CD2D]/30 bg-[#83CD2D]/15 text-[#5A8B1F]";
  }
  return "border-gray-200 bg-white text-gray-500";
}

function getStepDotClass(status: SetupStepStatus) {
  if (isStepComplete(status)) return "bg-[#83CD2D]";
  return "bg-gray-300";
}

function StatusPill({
  label,
  tone,
}: Readonly<{
  label: string;
  tone: "success" | "info" | "neutral";
}>) {
  const className =
    tone === "success"
      ? "bg-[#83CD2D]/15 text-[#5A8B1F]"
      : tone === "info"
        ? "bg-[#5080D8]/10 text-[#4070C8]"
        : "bg-gray-100 text-gray-600";
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${className}`}
    >
      {label}
    </span>
  );
}

function StepMeta({
  label,
  status,
}: Readonly<{ label: string; status: SetupStepStatus }>) {
  const className =
    status === "done"
      ? "text-[#5A8B1F]"
      : status === "blocked"
        ? "text-gray-400"
        : "text-gray-500";
  return (
    <span
      className={`inline-flex items-center gap-1.5 text-[11px] font-medium whitespace-nowrap ${className}`}
    >
      <span
        className={`h-1.5 w-1.5 rounded-full ${getStepDotClass(status)}`}
        aria-hidden="true"
      />
      {label}
    </span>
  );
}
