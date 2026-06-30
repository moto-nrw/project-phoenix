"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, Newspaper, Users } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import {
  type Child,
  type ChildStatus,
  type EnrollmentChildStatus,
  type EnrollmentRequest,
  type ParentAnnouncement,
  acknowledgeAnnouncement,
  listMyChildren,
  listMyEnrollments,
  listAnnouncements,
  markAnnouncementRead,
} from "~/lib/parent-api";
import { formatDate as formatGermanDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentDashboard" });

const statusTone: Record<
  EnrollmentChildStatus | ChildStatus,
  { bg: string; text: string; dot: string }
> = {
  submitted: { bg: "#5080D814", text: "#3558A8", dot: "#5080D8" },
  under_review: { bg: "#F78C1014", text: "#9A5B0A", dot: "#F78C10" },
  approved: { bg: "#83CD2D1F", text: "#5A8E1F", dot: "#83CD2D" },
  waitlisted: { bg: "#F78C1014", text: "#9A5B0A", dot: "#F78C10" },
  rejected: { bg: "#FF313014", text: "#CC2626", dot: "#FF3130" },
  withdrawn: { bg: "#F3F4F6", text: "#4B5563", dot: "#6B7280" },
  pending: { bg: "#83CD2D1F", text: "#5A8E1F", dot: "#83CD2D" },
  active: { bg: "#83CD2D1F", text: "#5A8E1F", dot: "#83CD2D" },
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

function formatDate(
  iso: string | undefined,
  locale: string,
  empty: string,
): string {
  if (!iso) return empty;
  return new Intl.DateTimeFormat(locale, {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  }).format(new Date(iso));
}

function formatServiceRange(
  from: string | undefined,
  until: string | undefined,
  locale: string,
  empty: string,
  connector: string,
) {
  if (!from && !until) return empty;
  return `${formatDate(from, locale, empty)} ${connector} ${formatDate(until, locale, empty)}`;
}

function normalizeChildIdentity(name: string, schoolName: string): string {
  return `${name.trim().toLowerCase()}::${schoolName.trim().toLowerCase()}`;
}

function getEnrollmentOverviewStatus(
  status: EnrollmentChildStatus,
  t: ReturnType<typeof useTranslations<"parentDashboard">>,
): string {
  if (status === "submitted" || status === "under_review") {
    return t("pendingDescription");
  }
  return t(`status.${status}`);
}

function buildChildOverviewItems(
  children: readonly Child[],
  requests: readonly EnrollmentRequest[],
  locale: string,
  t: ReturnType<typeof useTranslations<"parentDashboard">>,
): ChildOverviewItem[] {
  const items: ChildOverviewItem[] = children.map((child) => {
    const name = `${child.first_name} ${child.last_name}`;
    return {
      key: `child-${child.tenant_id}-${child.student_id}`,
      name,
      schoolName: child.school_name,
      detail: child.enrolled_from
        ? `${child.school_class ? `${child.school_class} · ` : ""}${t("careRange", { range: formatServiceRange(child.enrolled_from, child.enrolled_until, locale, t("notSet"), t("dateRangeConnector")) })}`
        : child.school_class || t("careRecorded"),
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
          locale,
          t("rangeOpen"),
          t("dateRangeConnector"),
        )}`,
        status: child.status,
        statusLabel: getEnrollmentOverviewStatus(child.status, t),
        href: `/parents/enroll/status/${request.status_token}`,
      });
    }
  }

  return items;
}

export function ParentDashboard() {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();
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
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.warn("parent_dashboard_load_failed", { error: message });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [t]);

  useEffect(() => {
    void load();
  }, [load]);

  const childOverviewItems = useMemo(
    () => buildChildOverviewItems(children, requests, locale, t),
    [children, requests, locale, t],
  );
  if (loading) {
    return <ParentDashboardSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="rounded-2xl border border-[#FF3130]/20 bg-[#FF3130]/10 p-5 text-sm text-[#CC2626] shadow-sm">
          {t("loadError")}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <section className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.25fr)_minmax(20rem,0.75fr)]">
          <div className="p-5 sm:p-6 lg:p-8">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("eyebrow")}
            </p>
            <div className="mt-2 max-w-3xl">
              <h1 className="text-2xl font-semibold text-balance text-gray-900 sm:text-3xl">
                {t("title")}
              </h1>
              <p className="mt-2 max-w-2xl text-sm leading-6 text-gray-600 sm:text-base">
                {t("description")}
              </p>
            </div>
          </div>
          <div className="moto-dotted-background moto-dotted-background--split border-t border-gray-200 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="relative z-10 space-y-4">
              <div>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  {t("childrenEyebrow")}
                </p>
                <p className="mt-1 text-sm leading-6 text-gray-600">
                  {t("childrenDescription")}
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
  const t = useTranslations("parentDashboard");
  const previewItems = items.slice(0, 3);

  if (previewItems.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-gray-300 bg-white/75 p-4 text-sm leading-6 text-gray-600 shadow-sm">
        {t("emptyChildren")}
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
          {t("allChildren")}
        </Link>
      ) : null}
    </div>
  );
}

function HeroChildItem({ item }: Readonly<{ item: ChildOverviewItem }>) {
  const t = useTranslations("parentDashboard");
  const tone = statusTone[item.status] ?? statusTone.submitted;
  const content = (
    <div className="flex min-w-0 items-center gap-3">
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-[#83CD2D]/15 text-[#5A8E1F]">
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
          {t("open")}
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
  const t = useTranslations("parentDashboard");
  const [items, setItems] = useState<ParentAnnouncement[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);
  const [pendingId, setPendingId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    void listAnnouncements()
      .then((list) => {
        if (active) setItems(list);
      })
      .catch((err: unknown) => {
        logger.error("parent_news_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => {
        if (active) setLoaded(true);
      });
    return () => {
      active = false;
    };
  }, []);

  const applyState = useCallback(
    (id: string, patch: Partial<ParentAnnouncement>) => {
      setItems((prev) =>
        prev.map((item) => (item.id === id ? { ...item, ...patch } : item)),
      );
    },
    [],
  );

  const handleMarkRead = useCallback(
    async (id: string) => {
      setPendingId(id);
      setActionError(null);
      try {
        await markAnnouncementRead(id);
        applyState(id, { read: true });
      } catch (err: unknown) {
        logger.error("parent_news_mark_read_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setActionError(t("newsActionError"));
      } finally {
        setPendingId(null);
      }
    },
    [applyState, t],
  );

  const handleAcknowledge = useCallback(
    async (id: string) => {
      setPendingId(id);
      setActionError(null);
      try {
        await acknowledgeAnnouncement(id);
        applyState(id, { read: true, acknowledged: true });
      } catch (err: unknown) {
        logger.error("parent_news_acknowledge_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setActionError(t("newsActionError"));
      } finally {
        setPendingId(null);
      }
    },
    [applyState, t],
  );

  return (
    <section
      id="news"
      className="scroll-mt-24 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
    >
      <PanelHeader
        eyebrow={t("newsEyebrow")}
        title={t("newsTitle")}
        description={t("newsDescription")}
      />

      {actionError && (
        <p className="mt-4 rounded-lg bg-[#FF31301A] px-3 py-2 text-sm text-[#CC2626]">
          {actionError}
        </p>
      )}

      {loaded && items.length === 0 ? (
        <div className="mt-5 rounded-xl border border-dashed border-gray-300 bg-gray-50 p-6">
          <div className="flex items-start gap-3">
            <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-white text-gray-500 shadow-sm ring-1 ring-gray-200">
              <Newspaper className="h-5 w-5" aria-hidden="true" />
            </span>
            <div className="min-w-0">
              <h3 className="text-sm font-semibold text-gray-900">
                {t("noNewsTitle")}
              </h3>
              <p className="mt-1 text-sm leading-6 text-gray-600">
                {t("noNewsDescription")}
              </p>
            </div>
          </div>
        </div>
      ) : (
        <ul className="mt-5 space-y-3">
          {items.map((item) => (
            <AnnouncementCard
              key={item.id}
              item={item}
              busy={pendingId === item.id}
              onMarkRead={handleMarkRead}
              onAcknowledge={handleAcknowledge}
            />
          ))}
        </ul>
      )}
    </section>
  );
}

function AnnouncementCard({
  item,
  busy,
  onMarkRead,
  onAcknowledge,
}: Readonly<{
  item: ParentAnnouncement;
  busy: boolean;
  onMarkRead: (id: string) => void;
  onAcknowledge: (id: string) => void;
}>) {
  const t = useTranslations("parentDashboard");
  const unread = !item.read;
  const needsAck = item.requires_acknowledgement && !item.acknowledged;

  return (
    <li
      className={`rounded-xl border bg-white p-4 shadow-sm ${
        unread ? "border-gray-300" : "border-gray-200"
      }`}
    >
      <div className="flex flex-wrap items-center gap-2">
        {unread && (
          <span className="inline-flex items-center rounded-full bg-gray-900 px-2 py-0.5 text-xs font-semibold text-white">
            {t("newsNew")}
          </span>
        )}
        {item.priority === "important" && (
          <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-700">
            {t("newsImportant")}
          </span>
        )}
        {item.acknowledged && (
          <span className="inline-flex items-center gap-1 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-600">
            <Check className="h-3 w-3" aria-hidden="true" />
            {t("newsAcknowledged")}
          </span>
        )}
      </div>

      <h3 className="mt-2 text-sm font-semibold text-gray-900">{item.title}</h3>
      <p className="mt-0.5 text-xs text-gray-500">
        {item.school_name}
        {item.published_at ? ` · ${formatGermanDate(item.published_at)}` : ""}
      </p>
      <p className="mt-2 text-sm leading-6 whitespace-pre-line text-gray-700">
        {item.body}
      </p>

      {(needsAck || unread) && (
        <div className="mt-3 flex flex-wrap gap-2">
          {needsAck ? (
            <button
              type="button"
              disabled={busy}
              onClick={() => onAcknowledge(item.id)}
              className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-gray-800 disabled:opacity-60"
            >
              <Check className="h-4 w-4" aria-hidden="true" />
              {t("newsAcknowledge")}
            </button>
          ) : (
            unread && (
              <button
                type="button"
                disabled={busy}
                onClick={() => onMarkRead(item.id)}
                className="inline-flex items-center rounded-lg border border-gray-300 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:opacity-60"
              >
                {t("newsMarkRead")}
              </button>
            )
          )}
        </div>
      )}
    </li>
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
      <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {eyebrow}
      </p>
      <h2 className="mt-1 text-xl font-semibold text-balance text-gray-900">
        {title}
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
    </header>
  );
}
