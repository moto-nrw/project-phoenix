"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Users } from "lucide-react";
import {
  type Child,
  type ChildStatus,
  type EnrollmentChildStatus,
  type EnrollmentRequest,
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

interface ChildOverviewItem {
  readonly key: string;
  readonly name: string;
  readonly schoolName: string;
  readonly detail: string;
  readonly status: EnrollmentChildStatus | ChildStatus;
  readonly statusLabel: string;
  readonly href?: string;
}

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

function normalizeChildIdentity(name: string, schoolName: string): string {
  return `${name.trim().toLowerCase()}::${schoolName.trim().toLowerCase()}`;
}

function getEnrollmentOverviewStatus(status: EnrollmentChildStatus): string {
  if (status === "submitted") return "Anmeldung abgesendet";
  if (status === "under_review") return "Wartet auf Rückmeldung";
  return enrollmentStatusLabel[status] ?? status;
}

function buildChildOverviewItems(
  children: readonly Child[],
  requests: readonly EnrollmentRequest[],
): ChildOverviewItem[] {
  const items: ChildOverviewItem[] = children.map((child) => {
    const name = `${child.first_name} ${child.last_name}`;
    return {
      key: `child-${child.tenant_id}-${child.student_id}`,
      name,
      schoolName: child.school_name,
      detail: child.enrolled_from
        ? `${child.school_class ? `${child.school_class} · ` : ""}Betreuung ${formatServiceRange(child.enrolled_from, child.enrolled_until)}`
        : child.school_class || "Betreuung bestätigt",
      status: child.status,
      statusLabel: childStatusLabel[child.status],
      href: `/parents/children/${child.student_id}`,
    };
  });

  const seen = new Set(
    items.map((item) => normalizeChildIdentity(item.name, item.schoolName)),
  );

  for (const request of requests) {
    for (const child of request.children) {
      const name = `${child.first_name} ${child.last_name}`;
      const identity = normalizeChildIdentity(name, request.school_name);
      if (seen.has(identity)) continue;
      seen.add(identity);
      items.push({
        key: `request-${request.request_id}-${child.child_id}`,
        name,
        schoolName: request.school_name,
        detail: `${request.phase_name} · ${formatServiceRange(
          request.service_start_date,
          request.service_end_date,
        )}`,
        status: child.status,
        statusLabel: getEnrollmentOverviewStatus(child.status),
      });
    }
  }

  return items;
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

  const childOverviewItems = useMemo(
    () => buildChildOverviewItems(children, requests),
    [children, requests],
  );
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
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.25fr)_minmax(20rem,0.75fr)]">
          <div className="p-5 sm:p-6 lg:p-8">
            <p className="text-xs font-semibold tracking-wide text-[#5080D8] uppercase">
              Elternportal
            </p>
            <div className="mt-2 max-w-3xl">
              <h1 className="text-2xl font-semibold text-balance text-gray-900 sm:text-3xl">
                Übersicht für Anmeldung und Betreuung
              </h1>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-600 sm:text-base">
                Ein ruhiger Einstieg in alles, was Eltern hier gerade brauchen:
                den aktuellen Stand sehen und die Kinder im Portal wiederfinden.
              </p>
            </div>
          </div>
          <div className="moto-dotted-background moto-dotted-background--split border-t border-gray-200 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="relative z-10 space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Aktueller Stand
                </p>
                <p className="mt-1 text-sm leading-6 text-gray-600">
                  Das Elternportal ist als schlanker Startpunkt gedacht. Weitere
                  Funktionen kommen nach und nach dazu.
                </p>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <StatusSummary label="Offen" value={openEnrollmentCount} />
                <StatusSummary label="Kinder" value={approvedChildCount} />
              </div>
            </div>
          </div>
        </div>
      </section>

      <ChildrenOverviewPanel items={childOverviewItems} />
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

function StatusSummary({
  label,
  value,
}: Readonly<{
  label: string;
  value: number;
}>) {
  return (
    <div className="rounded-xl border border-gray-200 bg-white/85 p-4 shadow-sm backdrop-blur-sm">
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </p>
      <p className="mt-1 text-2xl font-semibold text-gray-900 tabular-nums">
        {value}
      </p>
    </div>
  );
}

function ChildrenOverviewPanel({
  items,
}: Readonly<{ items: readonly ChildOverviewItem[] }>) {
  return (
    <section
      id="children"
      className="scroll-mt-24 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
    >
      <PanelHeader
        eyebrow="Kinder"
        title="Kinderübersicht"
        description={
          items.length === 0
            ? "Sobald eine Anmeldung vorliegt, erscheint das Kind hier."
            : pluralize(items.length, "Kind im Portal", "Kinder im Portal")
        }
      />

      {items.length === 0 ? (
        <ChildEmptyState />
      ) : (
        <div className="mt-5 grid gap-3 lg:grid-cols-2">
          {items.map((item) => (
            <ChildOverviewCard key={item.key} item={item} />
          ))}
        </div>
      )}
    </section>
  );
}

function ChildOverviewCard({ item }: Readonly<{ item: ChildOverviewItem }>) {
  const tone = statusTone[item.status] ?? statusTone.submitted;
  const content = (
    <div className="flex min-w-0 items-start gap-3">
      <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-[#83CD2D]/15 text-[#4A7A15]">
        <Users className="h-6 w-6" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <h3 className="text-base font-semibold break-words text-gray-900">
              {item.name}
            </h3>
            <p className="text-sm break-words text-gray-600">
              {item.schoolName}
            </p>
          </div>
          <span
            className="w-fit shrink-0 rounded-full px-2.5 py-1 text-xs font-semibold"
            style={{ backgroundColor: tone.bg, color: tone.text }}
          >
            {item.statusLabel}
          </span>
        </div>
        <div className="min-w-0">
          <p className="mt-2 text-sm leading-5 break-words text-gray-500">
            {item.detail}
          </p>
        </div>
      </div>
    </div>
  );

  if (!item.href) {
    return (
      <div className="rounded-2xl border border-gray-200 bg-gray-50/70 p-4">
        {content}
      </div>
    );
  }

  return (
    <Link
      href={item.href}
      className="group rounded-2xl border border-gray-200 bg-gray-50/70 p-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">{content}</div>
        <ArrowRight
          className="hidden h-4 w-4 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700 sm:block"
          aria-hidden="true"
        />
      </div>
    </Link>
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

function ChildEmptyState() {
  return (
    <div className="mt-5 rounded-xl border border-dashed border-gray-300 bg-gray-50 p-6 text-center">
      <span className="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm">
        <Users className="h-5 w-5" aria-hidden="true" />
      </span>
      <h3 className="mt-3 text-sm font-semibold text-gray-900">
        Noch keine Kinder
      </h3>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Sobald eine Anmeldung abgesendet oder bestätigt wurde, sehen Sie das
        Kind hier.
      </p>
    </div>
  );
}
