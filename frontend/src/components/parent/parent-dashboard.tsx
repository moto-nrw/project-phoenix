"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  CalendarDays,
  CalendarClock,
  CheckCircle2,
  ChevronRight,
  HeartPulse,
  MessageCircle,
  MessageSquareText,
  Users,
} from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import {
  type Child,
  type ChildFeatures,
  type EnrollmentChildStatus,
  type EnrollmentRequest,
  type ParentAnnouncement,
  type ThreadSummary,
  listMyChildren,
  listMyEnrollments,
  listAnnouncements,
  listMessageThreads,
  getChildFeatures,
} from "~/lib/parent-api";
import {
  NewsCard,
  NewsDetailModal,
  isOpenPoll,
} from "~/components/parent/news/news-components";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import {
  getParentCalendar,
  type CalendarEvent,
} from "~/lib/personal-calendar-api";
import { berlinTodayISO, parseISODate } from "~/lib/date-helpers";
import { SectionCard } from "~/components/ui/section-card";
import { EmptyState } from "~/components/ui/empty-state";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
import { ChildRow, type ChildRowItem } from "~/components/parent/child-row";
import {
  ParentLinkAction,
  ParentLoadError,
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";

const logger = createLogger({ component: "ParentDashboard" });

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
    studentId: child.student_id,
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
  const [featuresByStudent, setFeaturesByStudent] = useState<
    Record<string, ChildFeatures>
  >({});
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
      const featureEntries = await Promise.all(
        childList.slice(0, CHILDREN_PREVIEW_LIMIT).map(async (child) => {
          try {
            return [
              child.student_id,
              await getChildFeatures(child.student_id),
            ] as const;
          } catch (err) {
            logger.warn("parent_dashboard_child_features_load_failed", {
              error: err instanceof Error ? err.message : String(err),
              student_id: child.student_id,
            });
            return null;
          }
        }),
      );
      setFeaturesByStudent(
        Object.fromEntries(featureEntries.filter((entry) => entry !== null)),
      );
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
        title={t("title")}
        description={t("description")}
        variant="plain"
      />

      <DashboardUpdatesPanel />

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
          <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,24rem),1fr))] gap-3">
            {previewItems.map((item) => (
              <DashboardChildItem
                key={item.key}
                item={item}
                features={
                  item.studentId ? featuresByStudent[item.studentId] : undefined
                }
              />
            ))}
          </div>
        )}
      </SectionCard>
    </ParentPage>
  );
}

function DashboardChildItem({
  item,
  features,
}: Readonly<{ item: ChildRowItem; features?: ChildFeatures }>) {
  const t = useTranslations("parentDashboard");

  if (!item.studentId || !features) return <ChildRow item={item} />;

  const actions = [
    features.sick_note_enabled
      ? {
          href: `/parents/children/${item.studentId}?action=sick`,
          label: t("actions.sick"),
          icon: HeartPulse,
        }
      : null,
    features.pickup_change_enabled
      ? {
          href: `/parents/children/${item.studentId}?action=pickup`,
          label: t("actions.pickup"),
          icon: CalendarClock,
        }
      : null,
    features.notes_enabled
      ? {
          href: `/parents/messages/${item.studentId}`,
          label: t("actions.message"),
          icon: MessageCircle,
        }
      : null,
  ].filter((action) => action !== null);

  return (
    <div className="rounded-2xl border border-gray-200 bg-white p-3 shadow-sm">
      <ChildRow item={item} variant="plain" />
      {actions.length > 0 && (
        <div className="mt-3 grid grid-cols-1 gap-2 sm:grid-cols-3">
          {actions.map((action) => {
            const Icon = action.icon;
            return (
              <ParentLinkAction
                key={action.href}
                href={action.href}
                variant="secondary"
                className="min-h-11 justify-start gap-2 px-3"
              >
                <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
                <span className="text-left text-pretty">{action.label}</span>
              </ParentLinkAction>
            );
          })}
        </div>
      )}
    </div>
  );
}

const UPDATES_PREVIEW_LIMIT = 3;

function DashboardUpdatesPanel() {
  const t = useTranslations("parentDashboard");
  const locale = useLocale();
  const [items, setItems] = useState<ParentAnnouncement[]>([]);
  const [threads, setThreads] = useState<ThreadSummary[]>([]);
  const [appointments, setAppointments] = useState<CalendarEvent[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);

  useEffect(() => {
    let active = true;
    let loadSequence = 0;
    const loadUpdates = async () => {
      const sequence = ++loadSequence;
      const calendarFrom = parseISODate(berlinTodayISO());
      const calendarTo = new Date(calendarFrom);
      calendarTo.setDate(calendarTo.getDate() + 90);
      const [announcementsResult, threadsResult, appointmentsResult] =
        await Promise.allSettled([
          listAnnouncements(),
          listMessageThreads(),
          getParentCalendar(calendarFrom, calendarTo),
        ]);
      if (!active || sequence !== loadSequence) return;
      let failed = false;
      if (announcementsResult.status === "fulfilled") {
        setItems(announcementsResult.value);
      } else {
        failed = true;
        logger.error("parent_news_load_failed", {
          error: String(announcementsResult.reason),
        });
      }
      if (threadsResult.status === "fulfilled") {
        setThreads(threadsResult.value);
      } else {
        failed = true;
        logger.error("parent_messages_load_failed", {
          error: String(threadsResult.reason),
        });
      }
      if (appointmentsResult.status === "fulfilled") {
        setAppointments(appointmentsResult.value.events);
      } else {
        failed = true;
        logger.error("parent_calendar_load_failed", {
          error: String(appointmentsResult.reason),
        });
      }
      setLoadFailed(failed);
      setLoaded(true);
    };
    const refresh = () => void loadUpdates();

    void loadUpdates();
    window.addEventListener("parent-threads-refresh", refresh);
    window.addEventListener("parent-news-unread-refresh", refresh);
    window.addEventListener("focus", refresh);
    return () => {
      active = false;
      window.removeEventListener("parent-threads-refresh", refresh);
      window.removeEventListener("parent-news-unread-refresh", refresh);
      window.removeEventListener("focus", refresh);
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

  const actionableAnnouncements = items.filter(
    (item) =>
      !item.read ||
      isOpenPoll(item) ||
      (item.requires_acknowledgement && !item.acknowledged),
  );
  const unreadThreads = threads
    .filter((thread) => thread.unread > 0)
    .sort(
      (a, b) =>
        new Date(b.last_message_at ?? 0).getTime() -
        new Date(a.last_message_at ?? 0).getTime(),
    );
  const prioritizedAnnouncements = [...actionableAnnouncements].sort(
    (a, b) => Number(isOpenPoll(b)) - Number(isOpenPoll(a)),
  );
  const pendingAppointments = appointments
    .filter(
      (appointment) =>
        appointment.source === "appointment" &&
        appointment.can_respond &&
        appointment.response_status === "pending" &&
        appointment.cancelled !== true,
    )
    .sort((a, b) => a.start_date.localeCompare(b.start_date));
  const openPolls = prioritizedAnnouncements.filter(isOpenPoll);
  const otherAnnouncements = prioritizedAnnouncements.filter(
    (item) => !isOpenPoll(item),
  );
  const updates = [
    ...openPolls.map((item) => ({
      type: "announcement" as const,
      item,
    })),
    ...pendingAppointments.map((appointment) => ({
      type: "appointment" as const,
      appointment,
    })),
    ...unreadThreads.map((thread) => ({
      type: "message" as const,
      thread,
    })),
    ...otherAnnouncements.map((item) => ({
      type: "announcement" as const,
      item,
    })),
  ].slice(0, UPDATES_PREVIEW_LIMIT);
  const openItem = items.find((item) => item.id === openId) ?? null;

  return (
    <SectionCard
      id="updates"
      icon={MessageSquareText}
      title={t("updatesTitle")}
      description={t("updatesDescription")}
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
      <div aria-live="polite" aria-busy={!loaded}>
        {!loaded ? (
          <div className="space-y-2" aria-label={t("updatesLoading")}>
            <Skeleton className="h-24 rounded-xl" />
            <Skeleton className="h-24 rounded-xl" />
          </div>
        ) : updates.length === 0 && !loadFailed ? (
          <EmptyState
            icon={<CheckCircle2 className="h-7 w-7" aria-hidden="true" />}
            title={t("updatesEmptyTitle")}
            description={t("updatesEmptyDescription")}
            variant="compact"
          />
        ) : (
          <>
            <ul className="space-y-2">
              {updates.map((update) => {
                if (update.type === "announcement") {
                  return (
                    <li key={`announcement-${update.item.id}`}>
                      <NewsCard
                        item={update.item}
                        onOpen={(opened) => setOpenId(opened.id)}
                      />
                    </li>
                  );
                }
                if (update.type === "appointment") {
                  return (
                    <li key={`appointment-${update.appointment.id}`}>
                      <DashboardAppointmentCard
                        appointment={update.appointment}
                        locale={locale}
                      />
                    </li>
                  );
                }
                return (
                  <li key={`message-${update.thread.thread_id}`}>
                    <DashboardMessageCard thread={update.thread} />
                  </li>
                );
              })}
            </ul>
            {loadFailed && (
              <div className={updates.length > 0 ? "mt-3" : undefined}>
                <Alert type="warning" message={t("updatesLoadError")} />
              </div>
            )}
          </>
        )}
      </div>

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

function DashboardAppointmentCard({
  appointment,
  locale,
}: Readonly<{ appointment: CalendarEvent; locale: string }>) {
  const t = useTranslations("parentDashboard");

  return (
    <Link
      href={`/parents/calendar?date=${encodeURIComponent(appointment.start_date)}`}
      className="group flex min-h-24 w-full items-center gap-3 rounded-xl border border-gray-300 bg-white p-4 text-left shadow-sm transition-colors hover:border-gray-400 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <span className="bg-moto-orange-soft text-moto-orange-strong flex h-10 w-10 shrink-0 items-center justify-center rounded-xl">
        <CalendarDays className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="text-moto-orange-strong text-xs font-semibold">
          {t("appointmentResponsePending")}
        </span>
        <span className="mt-1 block truncate text-sm font-semibold text-gray-900">
          {appointment.title}
        </span>
        <span className="mt-0.5 block truncate text-xs text-gray-500">
          {appointment.student_name
            ? t("appointmentForChild", { name: appointment.student_name })
            : appointment.school_name}
          {` · ${formatLocalizedDate(appointment.start_date, locale)}`}
        </span>
        <span className="mt-2 block text-sm font-semibold text-gray-900">
          {t("appointmentRespond")}
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700"
        aria-hidden="true"
      />
    </Link>
  );
}

function DashboardMessageCard({ thread }: Readonly<{ thread: ThreadSummary }>) {
  const t = useTranslations("parentDashboard");
  const preview =
    thread.last_message_kind === undefined ||
    thread.last_message_kind === "message"
      ? thread.last_message_body
      : undefined;

  return (
    <Link
      href={`/parents/messages/${thread.student_id}`}
      className="group flex min-h-24 w-full items-center gap-3 rounded-xl border border-gray-300 bg-white p-4 text-left shadow-sm transition-colors hover:border-gray-400 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
    >
      <span className="bg-moto-blue-soft text-moto-blue-strong flex h-10 w-10 shrink-0 items-center justify-center rounded-xl">
        <MessageCircle className="h-5 w-5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="text-moto-blue-strong text-xs font-semibold">
          {thread.unread === 1
            ? t("messageNewOne")
            : t("messageNewMany", { count: thread.unread })}
        </span>
        <span className="mt-1 block truncate text-sm font-semibold text-gray-900">
          {thread.counterpart_name}
        </span>
        <span className="mt-0.5 block truncate text-xs text-gray-500">
          {t("messageAboutChild", { name: thread.student_name })}
        </span>
        {preview && (
          <span className="mt-1.5 line-clamp-2 block text-sm leading-5 text-gray-600">
            {preview}
          </span>
        )}
        <span className="mt-2 block text-sm font-semibold text-gray-900">
          {t("messageRead")}
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700"
        aria-hidden="true"
      />
    </Link>
  );
}
