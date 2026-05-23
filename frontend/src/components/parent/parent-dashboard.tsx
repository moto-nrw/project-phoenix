"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  CalendarDays,
  ClipboardList,
  Clock3,
  FileText,
  MessageCircle,
  Plus,
  ShieldCheck,
  Users,
  type LucideIcon,
} from "lucide-react";
import {
  type Child,
  type ChildStatus,
  type EnrollmentChildStatus,
  type EnrollmentRequest,
  groupBySchool,
  listMyChildren,
  listMyEnrollments,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentDashboard" });

const enrollmentStatusLabel: Record<EnrollmentChildStatus, string> = {
  submitted: "Eingereicht",
  under_review: "In Prüfung",
  approved: "Bestätigt",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
};

const childStatusLabel: Record<ChildStatus, string> = {
  pending: "Bestätigt",
  active: "Aktiv",
  inactive: "Beendet",
  alumnus: "Ehemalig",
};

const statusTone: Record<
  EnrollmentChildStatus | ChildStatus,
  { bg: string; text: string; dot: string }
> = {
  submitted: { bg: "#5080D8", text: "#FFFFFF", dot: "#5080D8" },
  under_review: { bg: "#5080D8", text: "#FFFFFF", dot: "#5080D8" },
  approved: { bg: "#83CD2D", text: "#FFFFFF", dot: "#83CD2D" },
  waitlisted: { bg: "#F78C10", text: "#FFFFFF", dot: "#F78C10" },
  rejected: { bg: "#D6373E", text: "#FFFFFF", dot: "#D6373E" },
  withdrawn: { bg: "#6B7280", text: "#FFFFFF", dot: "#6B7280" },
  pending: { bg: "#5080D8", text: "#FFFFFF", dot: "#5080D8" },
  active: { bg: "#83CD2D", text: "#FFFFFF", dot: "#83CD2D" },
  inactive: { bg: "#6B7280", text: "#FFFFFF", dot: "#6B7280" },
  alumnus: { bg: "#6B7280", text: "#FFFFFF", dot: "#6B7280" },
};

function formatDate(iso: string | undefined): string {
  if (!iso) return "Nicht gesetzt";
  return new Intl.DateTimeFormat("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  }).format(new Date(iso));
}

function formatServiceRange(
  from: string | undefined,
  until: string | undefined,
) {
  if (!from && !until) return "Zeitraum noch offen";
  return `${formatDate(from)} bis ${formatDate(until)}`;
}

function pluralize(count: number, one: string, many: string): string {
  return count === 1 ? `1 ${one}` : `${count} ${many}`;
}

export function ParentDashboard() {
  const [requests, setRequests] = useState<EnrollmentRequest[]>([]);
  const [children, setChildren] = useState<Child[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [enrollmentList, childList] = await Promise.all([
        listMyEnrollments(),
        listMyChildren(),
      ]);
      setRequests(enrollmentList);
      setChildren(childList);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unbekannter Fehler";
      logger.error("parent_dashboard_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const childGroups = useMemo(() => groupBySchool(children), [children]);
  const openEnrollmentCount = useMemo(
    () =>
      requests.reduce(
        (sum, request) =>
          sum +
          request.children.filter((child) =>
            ["submitted", "under_review", "waitlisted"].includes(child.status),
          ).length,
        0,
      ),
    [requests],
  );
  const approvedChildCount = children.filter(
    (child) => child.status === "active" || child.status === "pending",
  ).length;

  if (loading) {
    return <ParentDashboardSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          Die Übersicht konnte nicht geladen werden. Bitte aktualisieren Sie die
          Seite oder versuchen Sie es später erneut.
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.35fr)_minmax(21rem,0.65fr)]">
          <div className="p-5 sm:p-6 lg:p-8">
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Elternportal
            </p>
            <div className="mt-2 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
              <div className="min-w-0">
                <h1 className="max-w-3xl text-2xl font-semibold text-balance text-gray-900 sm:text-3xl">
                  Übersicht für Anmeldung und Betreuung
                </h1>
                <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-600 sm:text-base">
                  Sehen Sie den Status Ihrer Anmeldungen, behalten Sie Ihre
                  Kinder im Blick und starten Sie neue Anmeldungen an passenden
                  Schulen.
                </p>
              </div>
              <Link
                href="/parents/enroll"
                className="inline-flex h-11 shrink-0 items-center justify-center gap-2 rounded-xl bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                Neue Anmeldung
              </Link>
            </div>
          </div>
          <div className="border-t border-gray-200 bg-gray-50/70 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
              <MetricTile
                icon={FileText}
                label="Anmeldungen"
                value={requests.length.toString()}
                hint={
                  requests.length === 1
                    ? "1 Vorgang im Portal"
                    : `${requests.length} Vorgänge im Portal`
                }
              />
              <MetricTile
                icon={Users}
                label="Kinder"
                value={children.length.toString()}
                hint={pluralize(approvedChildCount, "bestätigt", "bestätigt")}
              />
              <MetricTile
                icon={Clock3}
                label="Offen"
                value={openEnrollmentCount.toString()}
                hint="Warten auf Rückmeldung"
              />
            </div>
          </div>
        </div>
      </section>

      <section className="grid gap-4 lg:grid-cols-3">
        <QuickActionCard
          icon={ClipboardList}
          title="Anmeldung starten"
          description="Wählen Sie eine Schule und reichen Sie eine neue Anmeldung ein."
          href="/parents/enroll"
          action="Neue Anmeldung"
          active
        />
        <QuickActionCard
          icon={CalendarDays}
          title="Abholzeiten"
          description="Regelmäßige Abholzeiten je Kind verwalten."
          action="Bald verfügbar"
        />
        <QuickActionCard
          icon={MessageCircle}
          title="Nachrichten"
          description="Direkte Rückfragen und Mitteilungen mit der OGS austauschen."
          action="Bald verfügbar"
        />
      </section>

      <div className="grid gap-6 xl:grid-cols-[minmax(0,1.05fr)_minmax(22rem,0.95fr)]">
        <EnrollmentsPanel requests={requests} />
        <ChildrenPanel groups={childGroups} />
      </div>
    </div>
  );
}

function ParentDashboardSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      <div className="grid gap-4 lg:grid-cols-3">
        {[0, 1, 2].map((item) => (
          <div
            key={item}
            className="h-32 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm"
          />
        ))}
      </div>
      <div className="h-80 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
    </div>
  );
}

function MetricTile({
  icon: Icon,
  label,
  value,
  hint,
}: Readonly<{
  icon: LucideIcon;
  label: string;
  value: string;
  hint: string;
}>) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white p-4 shadow-sm">
      <div className="flex items-start justify-between gap-3">
        <div>
          <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
            {label}
          </p>
          <p className="mt-1 text-2xl font-semibold text-gray-900 tabular-nums">
            {value}
          </p>
        </div>
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-900 text-white">
          <Icon className="h-4.5 w-4.5" aria-hidden="true" />
        </span>
      </div>
      <p className="mt-2 text-sm text-gray-500">{hint}</p>
    </div>
  );
}

function QuickActionCard({
  icon: Icon,
  title,
  description,
  href,
  action,
  active = false,
}: Readonly<{
  icon: LucideIcon;
  title: string;
  description: string;
  href?: string;
  action: string;
  active?: boolean;
}>) {
  const content = (
    <>
      <div className="flex items-start gap-3">
        <span
          className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${active ? "bg-gray-900 text-white" : "bg-gray-100 text-gray-500"}`}
        >
          <Icon className="h-5 w-5" aria-hidden="true" />
        </span>
        <div className="min-w-0 flex-1">
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
        </div>
      </div>
      <div className="mt-4 flex items-center justify-between gap-3 text-sm font-semibold">
        <span className={active ? "text-gray-900" : "text-gray-400"}>
          {action}
        </span>
        {active ? (
          <ArrowRight className="h-4 w-4 text-gray-700" aria-hidden="true" />
        ) : (
          <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-500">
            Geplant
          </span>
        )}
      </div>
    </>
  );

  if (!href) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-white p-5 opacity-80 shadow-sm">
        {content}
      </div>
    );
  }

  return (
    <Link
      href={href}
      className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      {content}
    </Link>
  );
}

function EnrollmentsPanel({
  requests,
}: Readonly<{ requests: EnrollmentRequest[] }>) {
  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
      <PanelHeader
        eyebrow="Anmeldungen"
        title="Meine Anmeldungen"
        description={
          requests.length === 0
            ? "Noch keine Anmeldung eingereicht."
            : "Öffnen Sie eine Anmeldung, um Status und Details einzusehen."
        }
      />

      {requests.length === 0 ? (
        <EmptyState
          icon={ClipboardList}
          title="Noch keine Anmeldung"
          description="Starten Sie eine neue Anmeldung, sobald ein Zeitraum geöffnet ist."
          href="/parents/enroll"
          action="Anmeldung starten"
        />
      ) : (
        <div className="mt-5 space-y-3">
          {requests.map((request) => (
            <EnrollmentRequestCard key={request.request_id} request={request} />
          ))}
        </div>
      )}
    </section>
  );
}

function EnrollmentRequestCard({
  request,
}: Readonly<{ request: EnrollmentRequest }>) {
  return (
    <Link
      href={`/parents/enroll/status/${encodeURIComponent(request.status_token)}`}
      className="group block rounded-xl border border-gray-200 bg-gray-50/60 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <p className="text-sm font-semibold text-gray-900">
            {request.school_name}
          </p>
          <p className="mt-1 text-sm text-gray-600">
            {request.phase_name} ·{" "}
            {formatServiceRange(
              request.service_start_date,
              request.service_end_date,
            )}
          </p>
          <p className="mt-2 text-xs text-gray-500">
            Eingereicht am {formatDate(request.submitted_at)}
          </p>
        </div>
        <span className="inline-flex shrink-0 items-center gap-1.5 text-sm font-semibold text-gray-700 transition-colors group-hover:text-gray-900">
          Öffnen
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </span>
      </div>

      <div className="mt-4 space-y-2">
        {request.children.map((child) => (
          <StatusRow
            key={child.child_id}
            title={`${child.first_name} ${child.last_name}`}
            subtitle={child.status_reason || "Status der Anmeldung"}
            label={enrollmentStatusLabel[child.status] ?? child.status}
            status={child.status}
          />
        ))}
      </div>
    </Link>
  );
}

function ChildrenPanel({
  groups,
}: Readonly<{
  groups: ReturnType<typeof groupBySchool>;
}>) {
  const childCount = groups.reduce(
    (sum, group) => sum + group.children.length,
    0,
  );

  return (
    <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
      <PanelHeader
        eyebrow="Kinder"
        title="Übersicht Ihrer Kinder"
        description={
          childCount === 0
            ? "Bestätigte Kinder erscheinen hier."
            : pluralize(childCount, "Kind im Portal", "Kinder im Portal")
        }
      />

      {groups.length === 0 ? (
        <EmptyState
          icon={Users}
          title="Noch keine Kinder"
          description="Sobald eine Anmeldung bestätigt wurde, sehen Sie Ihr Kind hier."
        />
      ) : (
        <div className="mt-5 space-y-4">
          {groups.map((group) => (
            <SchoolChildrenGroup
              key={group.schoolSlug}
              schoolName={group.schoolName}
              childItems={group.children}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function SchoolChildrenGroup({
  schoolName,
  childItems,
}: Readonly<{
  schoolName: string;
  childItems: Child[];
}>) {
  return (
    <div className="rounded-xl border border-gray-200 bg-gray-50/60 p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-gray-900">
            {schoolName}
          </p>
          <p className="text-xs text-gray-500">
            {pluralize(childItems.length, "Kind", "Kinder")}
          </p>
        </div>
        <ShieldCheck className="h-5 w-5 text-[#83CD2D]" aria-hidden="true" />
      </div>
      <div className="mt-3 space-y-2">
        {childItems.map((child) => (
          <Link
            key={`${child.tenant_id}-${child.student_id}`}
            href={`/parents/children/${child.student_id}`}
            className="block rounded-xl border border-gray-200 bg-white p-3 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          >
            <StatusRow
              title={`${child.first_name} ${child.last_name}`}
              subtitle={
                child.enrolled_from
                  ? `${child.school_class ? `${child.school_class} · ` : ""}Betreuung ${formatServiceRange(child.enrolled_from, child.enrolled_until)}`
                  : child.school_class || "Anmeldung läuft"
              }
              label={childStatusLabel[child.status]}
              status={child.status}
            />
          </Link>
        ))}
      </div>
    </div>
  );
}

function StatusRow({
  title,
  subtitle,
  label,
  status,
}: Readonly<{
  title: string;
  subtitle: string;
  label: string;
  status: EnrollmentChildStatus | ChildStatus;
}>) {
  const tone = statusTone[status] ?? statusTone.submitted;
  return (
    <div className="flex min-w-0 flex-col items-start gap-2 sm:flex-row sm:items-center sm:justify-between sm:gap-3">
      <div className="flex min-w-0 items-center gap-3 self-stretch">
        <span
          className="h-2.5 w-2.5 shrink-0 rounded-full"
          style={{ backgroundColor: tone.dot }}
          aria-hidden="true"
        />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold text-gray-900">
            {title}
          </p>
          <p className="truncate text-xs text-gray-500">{subtitle}</p>
        </div>
      </div>
      <span
        className="shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold sm:self-auto"
        style={{ backgroundColor: tone.bg, color: tone.text }}
      >
        {label}
      </span>
    </div>
  );
}

function PanelHeader({
  eyebrow,
  title,
  description,
}: Readonly<{
  eyebrow: string;
  title: string;
  description: string;
}>) {
  return (
    <header>
      <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
        {eyebrow}
      </p>
      <h2 className="mt-1 text-xl font-semibold text-balance text-gray-900">
        {title}
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
    </header>
  );
}

function EmptyState({
  icon: Icon,
  title,
  description,
  href,
  action,
}: Readonly<{
  icon: LucideIcon;
  title: string;
  description: string;
  href?: string;
  action?: string;
}>) {
  return (
    <div className="mt-5 rounded-xl border border-dashed border-gray-300 bg-gray-50 p-6 text-center">
      <span className="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
        <Icon className="h-5 w-5" aria-hidden="true" />
      </span>
      <h3 className="mt-3 text-sm font-semibold text-gray-900">{title}</h3>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
      {href && action && (
        <Link
          href={href}
          className="mt-4 inline-flex h-10 items-center justify-center gap-2 rounded-xl bg-gray-900 px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          {action}
        </Link>
      )}
    </div>
  );
}
