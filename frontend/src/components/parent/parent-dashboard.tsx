"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
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
import type { StatusBadgeTone } from "~/components/ui/status-badge";
import { SectionCard } from "~/components/ui/section-card";
import { EmptyState } from "~/components/ui/empty-state";
import { ChildRow, type ChildRowItem } from "~/components/parent/child-row";
import {
  ParentLinkAction,
  ParentLoadError,
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";

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
): ChildRowItem[] {
  const items: ChildRowItem[] = children.map((child) => ({
    key: `child-${child.tenant_id}-${child.student_id}`,
    name: `${child.first_name} ${child.last_name}`,
    schoolName: child.school_name,
    detail: child.enrolled_from
      ? `${child.school_class ? `${child.school_class} · ` : ""}${t("careRange", { range: formatServiceRange(child.enrolled_from, child.enrolled_until, locale, t("notSet"), t("dateRangeConnector")) })}`
      : child.school_class || t("careRecorded"),
    href: `/parents/children/${child.student_id}`,
  }));

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
        detail: `${request.phase_name} · ${getEnrollmentOverviewStatus(child.status, t)}`,
        badgeLabel: t("open"),
        badgeTone: statusTone[child.status] ?? "blue",
        href: `/parents/enroll/status/${request.status_token}`,
      });
    }
  }

  return items;
}

/** How many children the dashboard lists before deferring to /parents/children. */
const CHILDREN_PREVIEW_LIMIT = 4;

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
    return <ParentPageSkeleton rows={2} />;
  }

  if (error) {
    return <ParentLoadError message={t("loadError")} />;
  }

  const previewItems = childOverviewItems.slice(0, CHILDREN_PREVIEW_LIMIT);
  const hasMore = childOverviewItems.length > previewItems.length;

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("eyebrow")}
        title={t("title")}
        description={t("description")}
        actions={
          <ParentLinkAction href="/parents/enroll">
            <Plus className="h-4 w-4" aria-hidden="true" />
            {t("newEnrollment")}
          </ParentLinkAction>
        }
      />

      <SectionCard
        icon={Users}
        title={t("childrenEyebrow")}
        description={t("childrenDescription")}
        actions={
          hasMore ? (
            <ParentLinkAction href="/parents/children" variant="secondary">
              {t("allChildren")}
              <ArrowRight className="h-4 w-4" aria-hidden="true" />
            </ParentLinkAction>
          ) : undefined
        }
      >
        {previewItems.length === 0 ? (
          <p className="text-sm leading-6 text-gray-600">
            {t("emptyChildren")}
          </p>
        ) : (
          <div className="grid gap-2 sm:grid-cols-2">
            {previewItems.map((item) => (
              <ChildRow key={item.key} item={item} />
            ))}
          </div>
        )}
      </SectionCard>

      <StartNewsPanel />

      <NotificationPreferencesSection portal="parent" />
      <PushNotificationSection portal="parent" />
    </ParentPage>
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
    <SectionCard
      id="news"
      icon={Newspaper}
      title={t("newsTitle")}
      description={t("newsDescription")}
      className="scroll-mt-24"
      actions={
        items.length > 0 ? (
          <ParentLinkAction href="/parents/news" variant="secondary">
            {t("newsShowAll")}
            <ArrowRight className="h-4 w-4" aria-hidden="true" />
          </ParentLinkAction>
        ) : undefined
      }
    >
      {loaded && items.length === 0 ? (
        <EmptyState
          icon={<Newspaper className="h-8 w-8" aria-hidden="true" />}
          title={t("noNewsTitle")}
          description={t("noNewsDescription")}
          className="py-8"
        />
      ) : (
        <ul className="space-y-2">
          {visible.map((item) => (
            <li key={item.id}>
              <NewsCard item={item} onOpen={(opened) => setOpenId(opened.id)} />
            </li>
          ))}
        </ul>
      )}

      {openItem && (
        <NewsDetailModal
          item={openItem}
          onClose={() => setOpenId(null)}
          onUpdated={applyState}
          onStale={refetchOnStale}
        />
      )}
    </SectionCard>
  );
}
