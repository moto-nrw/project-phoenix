"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { ArrowRight, Newspaper, Plus, Users } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import {
  type Child,
  type ChildStatus,
  type EnrollmentChildStatus,
  type EnrollmentRequest,
  type ParentAnnouncement,
  listMyChildren,
  listMyEnrollments,
  listAnnouncements,
} from "~/lib/parent-api";
import {
  NewsCard,
  NewsDetailModal,
  isOpenPoll,
} from "~/components/parent/news/news-components";
import { NotificationPreferencesSection } from "~/components/settings/notification-preferences-section";
import { PushNotificationSection } from "~/components/settings/push-notification-section";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import { useParentNewsEnabled } from "~/lib/hooks/use-parent-news-enabled";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";

const logger = createLogger({ component: "ParentDashboard" });

const statusTone: Record<EnrollmentChildStatus | ChildStatus, StatusBadgeTone> =
  {
    submitted: "blue",
    under_review: "orange",
    approved: "green",
    waitlisted: "orange",
    rejected: "red",
    withdrawn: "gray",
    pending: "green",
    active: "green",
    inactive: "gray",
    alumnus: "gray",
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
  return formatLocalizedDate(iso, locale);
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
      <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
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
              <Link
                href="/parents/enroll"
                className="inline-flex h-10 items-center justify-center gap-2 rounded-lg bg-[#83CD2D] px-4 text-sm font-semibold text-white shadow-sm transition-colors hover:bg-[#74b827] focus-visible:ring-2 focus-visible:ring-[#83CD2D] focus-visible:ring-offset-2 focus-visible:outline-none"
              >
                <Plus className="h-4 w-4" aria-hidden="true" />
                {t("newEnrollment")}
              </Link>
            </div>
          </div>
        </div>
      </section>

      <StartNewsPanel />

      <NotificationPreferencesSection portal="parent" />
      <PushNotificationSection portal="parent" />
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
        <span className="shrink-0">
          <StatusBadge label={t("open")} tone={tone} />
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

/** Newest announcements as compact cards; the full feed lives at /parents/news. */
const NEWS_PANEL_LIMIT = 3;

function StartNewsPanel() {
  const t = useTranslations("parentDashboard");
  // Only render on the dashboard once a linked school broadcasts announcements
  // (the backend feed excludes disabled tenants). Rendered only in the parents
  // portal, so `enabled` is always true here. Keeps the panel from showing an
  // empty "Neuigkeiten" area when the feature is off for every linked school.
  const newsEnabled = useParentNewsEnabled(true);
  const [items, setItems] = useState<ParentAnnouncement[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

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

  // A read/ack was rejected because the announcement is no longer current;
  // refetch so a retracted item drops out and a corrected one refreshes.
  const refetchOnStale = useCallback(() => {
    void listAnnouncements()
      .then(setItems)
      .catch((err: unknown) => {
        logger.error("parent_news_refetch_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, []);

  // Open surveys first: the dashboard panel only shows a handful of items, and
  // a survey that scrolled out of the panel is a survey nobody answers (#1371).
  // Within each group the server order (newest published first) is preserved.
  const visible = [...items]
    .sort((a, b) => Number(isOpenPoll(b)) - Number(isOpenPoll(a)))
    .slice(0, NEWS_PANEL_LIMIT);
  const openItem = items.find((item) => item.id === openId) ?? null;

  if (!newsEnabled) return null;

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
        <>
          <ul className="mt-5 space-y-3">
            {visible.map((item) => (
              <li key={item.id}>
                <NewsCard
                  item={item}
                  onOpen={(opened) => setOpenId(opened.id)}
                />
              </li>
            ))}
          </ul>
          <Link
            href="/parents/news"
            className="mt-4 inline-flex items-center gap-1.5 text-sm font-medium text-gray-700 underline underline-offset-2 hover:text-gray-900"
          >
            {t("newsShowAll")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </Link>
        </>
      )}

      {openItem && (
        <NewsDetailModal
          item={openItem}
          onClose={() => setOpenId(null)}
          onUpdated={applyState}
          onStale={refetchOnStale}
        />
      )}
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
