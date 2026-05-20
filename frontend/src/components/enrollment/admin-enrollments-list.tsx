"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  CalendarRange,
  Check,
  ClipboardList,
  Eye,
  FileText,
  LockKeyhole,
  type LucideIcon,
  Settings2,
} from "lucide-react";
import {
  type AdminRequestSummary,
  type ChildStatus,
  listAdminRequests,
} from "~/lib/enrollment-admin-api";
import { listPhases, type Phase } from "~/lib/enrollment-phase-api";
import { listCareOfferings } from "~/lib/care-offering-api";
import { listSchemas, type FormSchema } from "~/lib/enrollment-form-schema-api";
import { fetchSettingsSchema } from "~/lib/settings-api";
import { useTenantSlugSafe } from "~/components/tenant/tenant-provider";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AdminEnrollmentsList" });

const STATUS_LABELS: Record<ChildStatus, string> = {
  submitted: "Eingegangen",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
  pending_renewal: "Wartet auf Verlängerung",
  auto_renewed: "Vorgemerkt",
  pending_admin_review: "Manuelle Prüfung",
};

const STATUS_COLORS: Record<ChildStatus, { bg: string; text: string }> = {
  submitted: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  under_review: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  approved: { bg: LOCATION_COLORS.GROUP_ROOM, text: "#FFFFFF" },
  waitlisted: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  rejected: { bg: LOCATION_COLORS.HOME, text: "#FFFFFF" },
  withdrawn: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
  pending_renewal: { bg: LOCATION_COLORS.SCHOOLYARD, text: "#FFFFFF" },
  auto_renewed: { bg: LOCATION_COLORS.OTHER_ROOM, text: "#FFFFFF" },
  pending_admin_review: { bg: LOCATION_COLORS.UNKNOWN, text: "#FFFFFF" },
};

const ALL_FILTER_VALUE = "all";
type SetupStepStatus = "done" | "todo" | "optional" | "blocked";

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
  const [phases, setPhases] = useState<Phase[]>([]);
  const [schemas, setSchemas] = useState<FormSchema[]>([]);
  const [careOfferingStats, setCareOfferingStats] = useState<CareOfferingStats>(
    { total: 0, activeInActivePhases: 0 },
  );
  const [enrollmentEnabled, setEnrollmentEnabled] = useState<boolean | null>(
    null,
  );
  const [requests, setRequests] = useState<AdminRequestSummary[]>([]);
  const [phaseFilter, setPhaseFilter] = useState<string>(ALL_FILTER_VALUE);
  const [statusFilter, setStatusFilter] = useState<string>(ALL_FILTER_VALUE);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    async function load() {
      setLoading(true);
      setError(null);
      try {
        const [phasesData, requestsData, schemasData, settingsData] =
          await Promise.all([
            listPhases(),
            listAdminRequests({
              phaseId:
                phaseFilter !== ALL_FILTER_VALUE ? phaseFilter : undefined,
              childStatus:
                statusFilter !== ALL_FILTER_VALUE
                  ? (statusFilter as ChildStatus)
                  : undefined,
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
        setRequests(requestsData);
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
  }, [phaseFilter, statusFilter]);

  const totals = useMemo(() => {
    const counts: Partial<Record<ChildStatus, number>> = {};
    for (const r of requests) {
      for (const c of r.children) {
        counts[c.status] = (counts[c.status] ?? 0) + 1;
      }
    }
    return counts;
  }, [requests]);

  const columns: DataTableColumn<AdminRequestSummary>[] = useMemo(
    () => [
      {
        key: "guardian",
        header: "Eltern",
        sortValue: (request) =>
          `${request.guardian_last_name} ${request.guardian_first_name}`,
        render: (request) => (
          <div className="min-w-0">
            <p className="truncate font-medium text-gray-900">
              {request.guardian_first_name} {request.guardian_last_name}
            </p>
            <p className="truncate text-xs text-gray-500">
              {request.guardian_email}
            </p>
          </div>
        ),
      },
      {
        key: "phase",
        header: "Anmeldephase",
        sortValue: (request) => request.phase_name || "",
        render: (request) => (
          <span className="text-sm text-gray-700">
            {request.phase_name || "Nicht zugeordnet"}
          </span>
        ),
      },
      {
        key: "submitted",
        header: "Eingegangen",
        sortValue: (request) => new Date(request.submitted_at).getTime(),
        render: (request) => (
          <span className="text-xs text-gray-600">
            {new Date(request.submitted_at).toLocaleString("de-DE", {
              day: "2-digit",
              month: "2-digit",
              year: "numeric",
              hour: "2-digit",
              minute: "2-digit",
            })}
          </span>
        ),
      },
      {
        key: "children",
        header: "Kinder",
        render: (request) => (
          <ul className="space-y-1.5">
            {request.children.map((child) => (
              <li
                key={child.id}
                className="flex min-w-0 flex-wrap items-center gap-2 text-xs"
              >
                <span className="truncate text-gray-900">
                  {child.first_name} {child.last_name}
                </span>
                <StatusBadge status={child.status} />
              </li>
            ))}
          </ul>
        ),
      },
      {
        key: "actions",
        header: "Aktionen",
        align: "right",
        render: (request) => (
          <Link
            href={
              tenantSlug
                ? `/${tenantSlug}/admin/enrollments/${request.id}`
                : `/admin/enrollments/${request.id}`
            }
            className="inline-flex rounded-lg px-2 py-1 text-xs font-medium text-[#5080D8] transition-colors hover:bg-[#5080D8]/10 focus-visible:ring-2 focus-visible:ring-[#5080D8]/40 focus-visible:outline-none"
          >
            Details
          </Link>
        ),
      },
    ],
    [tenantSlug],
  );

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
        schemaCount={schemas.length}
        careOfferingCount={careOfferingStats.total}
        activeCareOfferingCount={careOfferingStats.activeInActivePhases}
        requestCount={requests.length}
      />

      {error && (
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-4 text-sm text-[#CC2626]">
          {error}
        </div>
      )}

      <div className="moto-content-surface flex flex-wrap items-center gap-3 rounded-2xl border p-4 shadow-sm backdrop-blur-md">
        <label className="text-sm" htmlFor="enrollment-phase-filter">
          <span className="mr-2 font-medium text-gray-700">Anmeldephase</span>
          <select
            id="enrollment-phase-filter"
            name="phase"
            value={phaseFilter}
            onChange={(e) => setPhaseFilter(e.target.value)}
            className="moto-select moto-content-surface rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value={ALL_FILTER_VALUE}>Alle Anmeldephasen</option>
            {phases.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name}
              </option>
            ))}
          </select>
        </label>
        <label className="text-sm" htmlFor="enrollment-status-filter">
          <span className="mr-2 font-medium text-gray-700">Status</span>
          <select
            id="enrollment-status-filter"
            name="status"
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="moto-select moto-content-surface rounded-lg border px-3 py-2 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <option value={ALL_FILTER_VALUE}>Alle</option>
            {(Object.keys(STATUS_LABELS) as ChildStatus[]).map((s) => (
              <option key={s} value={s}>
                {STATUS_LABELS[s]}
              </option>
            ))}
          </select>
        </label>

        <div className="ml-auto flex flex-wrap gap-2 text-xs text-gray-600">
          {(Object.keys(STATUS_LABELS) as ChildStatus[]).map((s) => {
            const count = totals[s] ?? 0;
            if (count === 0) return null;
            return (
              <span
                key={s}
                className="rounded-full px-2 py-0.5 font-medium"
                style={{
                  backgroundColor: STATUS_COLORS[s].bg,
                  color: STATUS_COLORS[s].text,
                }}
              >
                {STATUS_LABELS[s]}: {count}
              </span>
            );
          })}
        </div>
      </div>

      <DataTable
        columns={columns}
        rows={requests}
        getRowKey={(request) => request.id}
        emptyState="Keine Anmeldungen für diese Filter."
        defaultSortKey="submitted"
        defaultSortDirection="desc"
      />
    </div>
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

interface EnrollmentSetupGuideProps {
  readonly enrollmentEnabled: boolean | null;
  readonly phaseCount: number;
  readonly activePhaseCount: number;
  readonly schemaCount: number;
  readonly careOfferingCount: number;
  readonly activeCareOfferingCount: number;
  readonly requestCount: number;
}

function EnrollmentSetupGuide({
  enrollmentEnabled,
  phaseCount,
  activePhaseCount,
  schemaCount,
  careOfferingCount,
  activeCareOfferingCount,
  requestCount,
}: EnrollmentSetupGuideProps) {
  const readyForPreview =
    enrollmentEnabled === true &&
    activePhaseCount > 0 &&
    activeCareOfferingCount > 0;

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
        "Das Basisformular reicht oft aus. Zusatzfelder sind optional.",
      href: "/enrollment-form",
      action: schemaCount > 0 ? "Formular prüfen" : "Basisformular prüfen",
      status: schemaCount > 0 ? "done" : "optional",
      meta: schemaCount === 0 ? "Basisformular" : `${schemaCount} Versionen`,
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

  const nextActionStep =
    steps.find((step) => step.status === "todo") ??
    steps.find((step) => step.status === "blocked") ??
    steps[steps.length - 1];

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      <div className="grid lg:grid-cols-[minmax(0,1fr)_20rem]">
        <div className="p-4 sm:p-5">
          <div className="border-b border-gray-100 pb-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div>
                <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
                  Konfigurator
                </p>
                <h2 className="mt-1 text-base font-semibold text-gray-900">
                  Online-Anmeldung vorbereiten
                </h2>
                <p className="mt-1 max-w-2xl text-sm text-gray-600">
                  Starte hier, wenn du neue Halbjahresanmeldungen,
                  Ferienbetreuung oder andere Anmeldezeiträume konfigurieren
                  möchtest. Das Basisformular ist immer vorhanden.
                </p>
              </div>
              <div className="flex flex-wrap gap-2 text-xs">
                <StatusPill
                  label={
                    readyForPreview ? "Bereit für Test" : "In Vorbereitung"
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
                        <ArrowRight className="h-3 w-3" aria-hidden="true" />
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
                {nextActionStep?.status === "blocked"
                  ? "Voraussetzungen fehlen"
                  : nextActionStep?.title}
              </h3>
              <p className="mt-1 text-sm text-gray-600">
                {nextActionStep?.status === "blocked"
                  ? "Lege zuerst eine aktive Anmeldephase und ein Betreuungsangebot an."
                  : nextActionStep?.description}
              </p>
            </div>

            {nextActionStep && (
              <Link
                href={nextActionStep.href}
                className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                {nextActionStep.status === "blocked"
                  ? "Setup prüfen"
                  : nextActionStep.action}
              </Link>
            )}

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
              Betreuungsangebote entscheidend. Das Basisformular ist schon
              vorhanden, danach kannst du die Elternansicht testen.
            </div>
          </div>
        </aside>
      </div>
    </section>
  );
}

function isStepComplete(status: SetupStepStatus) {
  return status === "done" || status === "optional";
}

function getStepIconClass(status: SetupStepStatus) {
  if (status === "done" || status === "optional") {
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

function StatusBadge({ status }: Readonly<{ status: ChildStatus }>) {
  const styles = STATUS_COLORS[status];
  return (
    <span
      className="inline-flex rounded-full px-2 py-0.5 text-[10px] font-medium"
      style={{ backgroundColor: styles.bg, color: styles.text }}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}
