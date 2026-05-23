"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Newspaper, Users } from "lucide-react";
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
  approved: "Freigeschaltet",
  waitlisted: "Warteliste",
  rejected: "Abgelehnt",
  withdrawn: "Zurückgezogen",
};

const statusTone: Record<
  EnrollmentChildStatus | ChildStatus,
  { bg: string; text: string; dot: string }
> = {
  submitted: { bg: "#EEF2FF", text: "#3558A8", dot: "#5080D8" },
  under_review: { bg: "#FFF7ED", text: "#9A5B0A", dot: "#F78C10" },
  approved: { bg: "#83CD2D1F", text: "#4A7A15", dot: "#83CD2D" },
  waitlisted: { bg: "#FFF7ED", text: "#9A5B0A", dot: "#F78C10" },
  rejected: { bg: "#FEF2F2", text: "#B4232A", dot: "#D6373E" },
  withdrawn: { bg: "#F3F4F6", text: "#4B5563", dot: "#6B7280" },
  pending: { bg: "#83CD2D1F", text: "#4A7A15", dot: "#83CD2D" },
  active: { bg: "#83CD2D1F", text: "#4A7A15", dot: "#83CD2D" },
  inactive: { bg: "#F3F4F6", text: "#4B5563", dot: "#6B7280" },
  alumnus: { bg: "#F3F4F6", text: "#4B5563", dot: "#6B7280" },
};

interface ChildOverviewItem {
  readonly key: string;
  readonly name: string;
  readonly schoolName: string;
  readonly detail: string;
  readonly status: EnrollmentChildStatus | ChildStatus;
  readonly statusLabel?: string;
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

function normalizeChildIdentity(name: string, schoolName: string): string {
  return `${name.trim().toLowerCase()}::${schoolName.trim().toLowerCase()}`;
}

function getEnrollmentOverviewStatus(status: EnrollmentChildStatus): string {
  if (status === "submitted" || status === "under_review") {
    return "Wartet auf Bestätigung oder Rückmeldung";
  }
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
        : child.school_class || "Betreuung hinterlegt",
      status: child.status,
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
      logger.warn("parent_dashboard_load_failed", { error: message });
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
                Willkommen im Elternportal
              </h1>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-600 sm:text-base">
                Hier sehen Sie Ihre Kinder und öffnen die wichtigsten Bereiche
                rund um Betreuung, Nachrichten und Termine.
              </p>
            </div>
          </div>
          <div className="moto-dotted-background moto-dotted-background--split border-t border-gray-200 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="relative z-10 space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Meine Kinder
                </p>
                <p className="mt-1 text-sm leading-6 text-gray-600">
                  Direkt zur Kind-Ansicht wechseln.
                </p>
              </div>
              <HeroChildrenList items={childOverviewItems} />
            </div>
          </div>
        </div>
      </section>

      <StartNewsPanel />
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

function HeroChildrenList({
  items,
}: Readonly<{ items: readonly ChildOverviewItem[] }>) {
  const previewItems = items.slice(0, 3);

  if (previewItems.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-gray-300 bg-white/75 p-4 text-sm leading-6 text-gray-600 shadow-sm">
        Sobald ein Kind freigeschaltet ist, erscheint es hier.
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {previewItems.map((item) => (
        <HeroChildItem key={item.key} item={item} />
      ))}
      {items.length > previewItems.length ? (
        <Link
          href="/parents/children"
          className="inline-flex h-9 items-center rounded-lg px-2 text-sm font-semibold text-gray-700 transition-colors hover:bg-white/80 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          Alle Kinder anzeigen
        </Link>
      ) : null}
    </div>
  );
}

function HeroChildItem({ item }: Readonly<{ item: ChildOverviewItem }>) {
  const tone = statusTone[item.status] ?? statusTone.submitted;
  const content = (
    <div className="flex min-w-0 items-center gap-3">
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#83CD2D]/15 text-[#4A7A15]">
        <Users className="h-5 w-5" aria-hidden="true" />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-gray-900">
          {item.name}
        </p>
        <p className="truncate text-sm text-gray-600">{item.schoolName}</p>
      </div>
      {item.statusLabel ? (
        <span
          className="shrink-0 rounded-full px-2 py-1 text-[11px] font-semibold"
          style={{ backgroundColor: tone.bg, color: tone.text }}
        >
          Offen
        </span>
      ) : (
        <ArrowRight
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      )}
    </div>
  );

  if (!item.href) {
    return (
      <div className="rounded-xl border border-gray-200 bg-white/80 p-3 shadow-sm">
        {content}
      </div>
    );
  }

  return (
    <Link
      href={item.href}
      className="block rounded-xl border border-gray-200 bg-white/90 p-3 shadow-sm transition-colors hover:bg-white focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      {content}
    </Link>
  );
}

function StartNewsPanel() {
  return (
    <section
      id="news"
      className="scroll-mt-24 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
    >
      <PanelHeader
        eyebrow="Aktuelles"
        title="Neuigkeiten"
        description="Meldungen aus Betreuung und Elternportal erscheinen hier."
      />

      <div className="mt-5 rounded-xl border border-dashed border-gray-300 bg-gray-50 p-6">
        <div className="flex items-start gap-3">
          <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm ring-1 ring-gray-200">
            <Newspaper className="h-5 w-5" aria-hidden="true" />
          </span>
          <div className="min-w-0">
            <h3 className="text-sm font-semibold text-gray-900">
              Keine Neuigkeiten vorhanden
            </h3>
            <p className="mt-1 text-sm leading-6 text-gray-600">
              Sobald es neue Hinweise gibt, sehen Sie diese direkt auf der
              Startseite.
            </p>
          </div>
        </div>
      </div>
    </section>
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
