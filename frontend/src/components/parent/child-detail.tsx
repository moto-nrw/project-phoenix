"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { ArrowLeft, Clock } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import {
  type Child,
  type ThreadSummary,
  listChildThreads,
  listMyChildren,
} from "~/lib/parent-api";
import { parentThreadPreviewI18nDescriptor } from "~/lib/messaging-status";
import { createLogger } from "~/lib/logger";
import { formatLocalizedDate } from "~/lib/localized-date-format";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import {
  type ChildCare,
  PickupTimeModal,
  SickNoteModal,
  SickStatusSummary,
  type TodayPickup,
  useChildCare,
} from "~/components/parent/child-care";
import RelatedAccountsPanel from "~/components/parent/related-accounts-panel";
import GuardiansPanel from "~/components/parent/guardians-panel";
import { Button } from "~/components/ui/button";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { formatChatDateTime } from "~/lib/date-helpers";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";

// Quick-actions that are wired to real backend flows. The rest remain
// "coming soon" stubs until their features ship.
// Modal-backed quick actions. "message" is handled separately — it opens the
// new-conversation modal rather than a per-action modal.
const SUPPORTED_ACTIONS: Record<string, "sick" | "pickup"> = {
  sick: "sick",
  pickupTime: "pickup",
};

// An action is usable only when it's wired AND the child's school has the
// matching feature enabled — otherwise the backend would reject it with 403.
// Pickup changes are the exception: existing guardian-authored rows must stay
// clearable even after the school disables new parent changes.
function isActionEnabled(actionKey: string, care: ChildCare): boolean {
  // Messaging shares the operations.parent_notes_enabled gate.
  if (actionKey === "message") return care.features.notes_enabled;
  const target = SUPPORTED_ACTIONS[actionKey];
  if (target === "sick") return care.features.sick_note_enabled;
  if (target === "pickup") {
    return (
      care.features.pickup_change_enabled ||
      care.careExceptions.some((entry) => entry.source === "guardian")
    );
  }
  return false;
}

const logger = createLogger({ component: "ChildDetail" });

// Neutral icon tile shared across the quick actions and the "Heute" panel:
// gray-100 surface, no border/shadow. Keeps the parents portal calm and
// consistent with the app-wide gray-tile pattern instead of the old
// per-action pastel/white tiles. Add size + radius per call site.
const ACTION_TILE_CLASS =
  "flex shrink-0 items-center justify-center bg-gray-100";

const CHILD_ACTIONS = [
  { key: "sick", concept: "sick" },
  { key: "pickupTime", concept: "pickup" },
  { key: "message", concept: "parentConversations" },
  { key: "pickupPermission", concept: "permissions" },
  { key: "people", concept: "parents" },
  { key: "news", concept: "news" },
] as const;

interface Props {
  readonly studentId: string;
}

type ChildAction = (typeof CHILD_ACTIONS)[number];
type ChildDetailTranslator = ReturnType<
  typeof useTranslations<"parentChildDetail">
>;

function formatDate(
  iso: string | undefined,
  locale: string,
  t: ChildDetailTranslator,
): string {
  if (!iso) return t("open");
  return formatLocalizedDate(iso, locale);
}

function getServiceRange(
  child: Child,
  locale: string,
  t: ChildDetailTranslator,
): string {
  if (!child.enrolled_from && !child.enrolled_until) return t("open");
  return t("range", {
    from: formatDate(child.enrolled_from, locale, t),
    to: formatDate(child.enrolled_until, locale, t),
  });
}

function getInitials(child: Child): string {
  return `${child.first_name.at(0) ?? ""}${child.last_name.at(0) ?? ""}`.toUpperCase();
}

export function ChildDetail({ studentId }: Props) {
  const t = useTranslations("parentChildDetail");
  const [child, setChild] = useState<Child | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const list = await listMyChildren();
      const match = list.find((c) => c.student_id === studentId) ?? null;
      setChild(match);
    } catch (err) {
      const message = err instanceof Error ? err.message : t("unknownError");
      logger.warn("child_detail_load_failed", {
        error: message,
        student_id: studentId,
      });
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [studentId, t]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) {
    return <ChildDetailSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="border-moto-red/20 bg-moto-red/10 text-moto-red-strong rounded-2xl border p-5 text-sm shadow-sm">
          {t("loadError")}
        </div>
      </div>
    );
  }

  if (!child) {
    return (
      <div className="mx-auto max-w-7xl">
        <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
          <BackBar />
          <div className="p-6 text-sm leading-6 text-gray-600">
            {t("notFound")}
          </div>
        </div>
      </div>
    );
  }

  return <ChildDetailContent child={child} />;
}

function ChildDetailContent({ child }: Readonly<{ child: Child }>) {
  const t = useTranslations("parentChildDetail");
  const tMd = useTranslations("parentMasterData");
  const locale = useLocale();
  const fullName = `${child.first_name} ${child.last_name}`;
  useSetBreadcrumb({ pageTitle: fullName });
  const care = useChildCare(child.student_id);
  const router = useRouter();
  const [modal, setModal] = useState<null | "sick" | "pickup">(null);
  const [threads, setThreads] = useState<ThreadSummary[]>([]);

  // Monotonic request counter so only the latest in-flight thread fetch may
  // write state. Navigating between children reuses this component (the route
  // param changes), so without this guard a late response for child A — or a
  // failed request for child B after viewing A — could leave A's conversation
  // preview/unread count rendered on B's page.
  const threadsRequestRef = useRef(0);

  // The child's conversation (this child only), fetched via the per-child
  // endpoint so we don't pull the guardian's whole cross-tenant inbox just to
  // render one child. Chat model: at most one conversation per child.
  const reloadThreads = useCallback(() => {
    const requestStudentId = child.student_id;
    const seq = ++threadsRequestRef.current;
    listChildThreads(requestStudentId)
      .then((result) => {
        // Drop a response that lost the race to a newer request (child switch or
        // a concurrent SSE-triggered reload) so stale data never renders.
        if (threadsRequestRef.current !== seq) return;
        setThreads(result);
      })
      .catch((err) => {
        if (threadsRequestRef.current !== seq) return;
        logger.warn("child_threads_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: requestStudentId,
        });
      });
  }, [child.student_id]);

  useEffect(() => {
    // Reset on child change so child A's preview never shows on child B while B's
    // request is in flight; the seq guard then keeps a late A response from
    // overwriting B.
    setThreads([]);
    reloadThreads();
  }, [reloadThreads]);

  // Real-time: the portal-wide ParentRealtimeBridge owns the single parents-app
  // SSE connection and dispatches `parent-conversation-refresh` (carrying the
  // affected studentId) on every parent_message. Subscribe via the shared hook,
  // like OgsConversation and the parent messages list, so this card refreshes its
  // unread count + last-message preview without a remount. Listen ONLY to the
  // filtered per-conversation event (not the unfiltered `parent-threads-refresh`
  // the bridge also fires for the same message): the bridge dispatches both on
  // every event, so the filtered one already covers all cases — and listening to
  // both reloaded this child twice per own-child message and once per other-child
  // message. The hook skips a refetch when the event names a DIFFERENT child.
  // marksRead: false — reloadThreads only re-reads thread summaries; it never
  // advances a read cursor (that happens when the conversation itself is opened),
  // so it should refresh even in a background tab rather than deferring to focus.
  useMessagesActivity({
    eventName: "parent-conversation-refresh",
    studentId: child.student_id,
    onMatch: reloadThreads,
    marksRead: false,
  });

  const openAction = useCallback(
    (actionKey: string) => {
      if (!isActionEnabled(actionKey, care)) return;
      if (actionKey === "message") {
        router.push(`/parents/messages/${child.student_id}`);
        return;
      }
      const target = SUPPORTED_ACTIONS[actionKey];
      if (target) setModal(target);
    },
    [care, router, child.student_id],
  );

  const openConversation = useCallback(() => {
    router.push(`/parents/messages/${child.student_id}`);
  }, [router, child.student_id]);

  const summaryItems = useMemo(
    () => [
      { label: t("schoolLabel"), value: child.school_name },
      { label: t("classLabel"), value: child.school_class || t("notSet") },
      { label: t("careLabel"), value: getServiceRange(child, locale, t) },
    ],
    [child, locale, t],
  );

  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <MobileChildAppView
        child={child}
        fullName={fullName}
        care={care}
        threads={threads}
        onAction={openAction}
      />

      <section className="moto-content-surface hidden overflow-hidden rounded-2xl border shadow-sm lg:block">
        <div className="grid gap-0 lg:grid-cols-[minmax(0,1.25fr)_minmax(20rem,0.75fr)] xl:grid-cols-[minmax(0,1.35fr)_minmax(22rem,0.8fr)]">
          <div>
            <BackBar />
            <div className="p-5 sm:p-6 lg:p-8">
              <div className="flex min-w-0 items-start gap-4">
                <span className="bg-moto-green/15 text-moto-green-strong flex h-16 w-16 shrink-0 items-center justify-center rounded-2xl text-lg font-semibold">
                  {getInitials(child) || (
                    <MotoConceptIcon concept="children" size={30} />
                  )}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    {t("childEyebrow")}
                  </p>
                  <h1 className="mt-1 text-3xl font-semibold break-words text-gray-900 sm:text-4xl">
                    {fullName}
                  </h1>
                  <p className="mt-2 text-sm leading-6 break-words text-gray-600 sm:text-base">
                    {child.school_name}
                    {child.school_class ? `, ${child.school_class}` : ""}
                  </p>
                </div>
              </div>
              <div className="mt-7 grid max-w-3xl grid-cols-[repeat(auto-fit,minmax(11rem,1fr))] gap-3">
                {CHILD_ACTIONS.slice(0, 3).map((action) => (
                  <DesktopQuickAction
                    key={action.key}
                    action={action}
                    onClick={
                      isActionEnabled(action.key, care)
                        ? () => openAction(action.key)
                        : undefined
                    }
                  />
                ))}
              </div>
            </div>
          </div>
          <div className="moto-dotted-background moto-dotted-background--split border-t border-gray-200 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <TodayPanel care={care} threads={threads} />
          </div>
        </div>
      </section>

      <div className="grid gap-6 lg:grid-cols-[minmax(0,0.9fr)_minmax(24rem,0.8fr)]">
        <section className="moto-content-surface min-w-0 rounded-2xl border shadow-sm max-lg:hidden">
          <div className="p-5 sm:p-6">
            <PanelHeader
              eyebrow={t("masterDataEyebrow")}
              title={t("masterDataTitle")}
              description={t("masterDataDescription")}
            />
            {care.features.has_open_change_request && (
              <div className="mt-3">
                <OpenRequestBadge />
              </div>
            )}
          </div>
          <dl className="divide-y divide-gray-100 border-t border-gray-100">
            {summaryItems.map((item) => (
              <InfoRow key={item.label} label={item.label} value={item.value} />
            ))}
          </dl>
          <div className="border-t border-gray-100 px-5 py-4 sm:px-6">
            <Link
              href={`/parents/children/${child.student_id}/stammdaten`}
              className="bg-moto-green hover:bg-moto-green-hover inline-flex h-9 items-center gap-2 rounded-lg px-4 text-sm font-semibold text-gray-950 shadow-sm transition-colors"
            >
              {tMd("openLink")}
            </Link>
          </div>
        </section>

        <div className="grid min-w-0 gap-6 max-lg:hidden">
          <ChildMessagesPanel
            studentId={child.student_id}
            threads={threads}
            composeDisabled={!care.features.notes_enabled}
            onCompose={openConversation}
          />
          <GuardiansPanel studentId={child.student_id} />
          <RelatedAccountsPanel
            studentId={child.student_id}
            canInvite={care.features.related_accounts_invite_enabled}
            canRemove={care.features.related_accounts_remove_enabled}
          />
        </div>
      </div>

      {modal === "sick" && (
        <SickNoteModal
          onClose={() => setModal(null)}
          onSubmit={care.reportSick}
          excusedRequiresApproval={care.features.excused_requires_approval}
        />
      )}
      {modal === "pickup" && (
        <PickupTimeModal
          careExceptions={care.careExceptions}
          careExceptionsLoaded={care.careExceptionsLoaded}
          pickupChangeEnabled={care.features.pickup_change_enabled}
          onClose={() => setModal(null)}
          onSubmit={care.saveCareException}
          onRemove={care.removeCareException}
        />
      )}
    </div>
  );
}

function MobileChildAppView({
  child,
  fullName,
  care,
  threads,
  onAction,
}: Readonly<{
  child: Child;
  fullName: string;
  care: ChildCare;
  threads: ThreadSummary[];
  onAction: (actionKey: string) => void;
}>) {
  const t = useTranslations("parentChildDetail");
  const tMd = useTranslations("parentMasterData");
  const locale = useLocale();
  const primaryActions = CHILD_ACTIONS.slice(0, 3);

  return (
    <div className="space-y-5 lg:hidden">
      <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
        <BackBar />
        <div className="p-5">
          <div className="flex min-w-0 items-center gap-4">
            <span className="bg-moto-green/15 text-moto-green-strong flex h-14 w-14 shrink-0 items-center justify-center rounded-3xl text-lg font-semibold">
              {getInitials(child) || (
                <MotoConceptIcon concept="children" size={26} />
              )}
            </span>
            <div className="min-w-0">
              <h1 className="text-xl font-semibold break-words text-gray-900">
                {fullName}
              </h1>
              <p className="mt-0.5 text-sm break-words text-gray-600">
                {child.school_name}
                {child.school_class ? `, ${child.school_class}` : ""}
              </p>
              <span className="bg-moto-green/15 text-moto-green-strong mt-3 inline-flex max-w-full rounded-full px-3 py-1 text-xs font-semibold">
                {t("careRecorded")}
              </span>
            </div>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {primaryActions.map((action) => (
          <MobileQuickAction
            key={action.key}
            action={action}
            onClick={
              isActionEnabled(action.key, care)
                ? () => onAction(action.key)
                : undefined
            }
          />
        ))}
      </div>

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.sickLabel")}
        </p>
        <div className="mt-2">
          <SickStatusSummary
            sickDays={care.sickDays}
            excusedRequests={care.excusedRequests}
            onWithdraw={care.withdrawExcused}
          />
        </div>
      </section>

      <ChildMessagesPanel
        studentId={child.student_id}
        threads={threads}
        composeDisabled={!care.features.notes_enabled}
        onCompose={() => onAction("message")}
        mobile
      />

      <GuardiansPanel studentId={child.student_id} mobile />

      <RelatedAccountsPanel
        studentId={child.student_id}
        canInvite={care.features.related_accounts_invite_enabled}
        canRemove={care.features.related_accounts_remove_enabled}
        mobile
      />

      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm">
        <div className="flex flex-wrap items-center gap-2">
          <h2 className="text-lg font-semibold text-gray-900">
            {t("careLabel")}
          </h2>
          {care.features.has_open_change_request && <OpenRequestBadge />}
        </div>
        <dl className="mt-4 space-y-3">
          <CompactInfoRow
            label={t("periodLabel")}
            value={getServiceRange(child, locale, t)}
          />
          <CompactInfoRow
            label={t("classLabel")}
            value={child.school_class || t("notSet")}
          />
        </dl>
        <Link
          href={`/parents/children/${child.student_id}/stammdaten`}
          className="bg-moto-green hover:bg-moto-green-hover mt-4 inline-flex h-10 w-full items-center justify-center gap-2 rounded-xl px-4 text-sm font-semibold text-gray-950 shadow-sm transition-colors"
        >
          {tMd("openLink")}
        </Link>
      </section>
    </div>
  );
}

function MobileQuickAction({
  action,
  onClick,
}: Readonly<{ action: ChildAction; onClick?: () => void }>) {
  const t = useTranslations("parentChildDetail");
  const enabled = Boolean(onClick);
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!enabled}
      className={`flex aspect-square min-h-28 flex-col items-center justify-center gap-3 rounded-3xl border p-3 text-center shadow-sm transition-colors ${
        enabled
          ? "border-gray-200 bg-white hover:bg-gray-50"
          : "cursor-not-allowed border-gray-100 bg-white"
      }`}
    >
      <span className={`${ACTION_TILE_CLASS} h-11 w-11 rounded-xl`}>
        <MotoConceptIcon concept={action.concept as MotoConceptKey} size={26} />
      </span>
      <span className="text-xs leading-4 font-semibold text-gray-900">
        {t(`actions.${action.key}.label`)}
      </span>
    </button>
  );
}

function DesktopQuickAction({
  action,
  onClick,
}: Readonly<{ action: ChildAction; onClick?: () => void }>) {
  const t = useTranslations("parentChildDetail");
  const label = t(`actions.${action.key}.label`);
  const enabled = Boolean(onClick);
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={!enabled}
      className={`flex min-h-24 items-center gap-3 rounded-xl border p-4 text-left transition-colors ${
        enabled
          ? "border-gray-200 bg-white hover:bg-gray-50"
          : "cursor-not-allowed border-gray-200 bg-gray-50/70 opacity-80"
      }`}
      aria-label={enabled ? label : t("comingSoonAria", { label })}
    >
      <span className={`${ACTION_TILE_CLASS} h-10 w-10 rounded-lg`}>
        <MotoConceptIcon concept={action.concept as MotoConceptKey} size={22} />
      </span>
      <span className="min-w-0 [overflow-wrap:anywhere]">
        <span className="block text-sm font-semibold text-gray-900">
          {label}
        </span>
        <span className="mt-0.5 block text-sm leading-5 text-gray-600">
          {t(`actions.${action.key}.description`)}
        </span>
      </span>
    </button>
  );
}

// Renders today's real pickup state (never a fabricated value, #1725): the
// effective time (with a "geändert" marker when a same-day override differs
// from the base plan), "Heute abgemeldet" on an absence, "Keine Abholung heute"
// when nothing is configured, and a neutral dash when the plan couldn't load.
function TodayPickupValue({
  pickup,
  t,
}: Readonly<{ pickup: TodayPickup; t: ChildDetailTranslator }>) {
  switch (pickup.kind) {
    case "time":
      return (
        <>
          {t("today.pickupTime", { time: pickup.time })}
          {pickup.changed && (
            <span className="ml-1 font-medium text-gray-500">
              · {t("today.pickupChanged")}
            </span>
          )}
        </>
      );
    case "absent":
      return <>{t("today.pickupAbsent")}</>;
    case "none":
      return <>{t("today.pickupNone")}</>;
    default:
      return <>—</>;
  }
}

function TodayPanel({
  care,
  threads,
}: Readonly<{ care: ChildCare; threads: ThreadSummary[] }>) {
  const t = useTranslations("parentChildDetail");
  const unread = threads.reduce((sum, thread) => sum + thread.unread, 0);
  return (
    <div className="relative z-10 space-y-4">
      <div>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("today.title")}
        </p>
        <p className="mt-1 text-sm leading-6 text-gray-600">
          {t("today.description")}
        </p>
      </div>
      <div className="space-y-2">
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className={`${ACTION_TILE_CLASS} h-10 w-10 rounded-lg`}>
            <MotoConceptIcon concept="sick" size={22} />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.sickLabel")}
            </p>
            <div className="mt-0.5">
              <SickStatusSummary
                sickDays={care.sickDays}
                excusedRequests={care.excusedRequests}
                onWithdraw={care.withdrawExcused}
              />
            </div>
          </div>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className={`${ACTION_TILE_CLASS} h-10 w-10 rounded-lg`}>
            <MotoConceptIcon concept="pickup" size={22} />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.pickupLabel")}
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              <TodayPickupValue pickup={care.todayPickup} t={t} />
            </p>
          </div>
        </div>
        <div className="flex items-center gap-3 rounded-xl border border-gray-200 bg-white/85 p-3 shadow-sm">
          <span className={`${ACTION_TILE_CLASS} h-10 w-10 rounded-lg`}>
            <MotoConceptIcon concept="parentConversations" size={22} />
          </span>
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
              {t("today.messagesLabel")}
            </p>
            <p className="mt-0.5 text-sm font-semibold text-gray-900">
              {threads.length === 0
                ? t("today.noConversations")
                : unread > 0
                  ? t("today.unreadCount", { count: unread })
                  : t("today.conversationsCount", { count: threads.length })}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}

// Shows this child's OGS conversations (filtered to the child) with an unread
// pill and last activity. Each row opens the thread; "Neue Nachricht" starts a
// new conversation pre-selected to this child. Mirrors the staff-side
// ParentMessagesCard.
function ChildMessagesPanel({
  studentId,
  threads,
  onCompose,
  composeDisabled = false,
  mobile = false,
}: Readonly<{
  studentId: string;
  threads: ThreadSummary[];
  onCompose: () => void;
  composeDisabled?: boolean;
  mobile?: boolean;
}>) {
  const t = useTranslations("parentChildDetail");
  const tMsg = useTranslations("parentOgsMessaging");
  const router = useRouter();
  // Chat model: at most one conversation per child.
  const conversation = threads[0];
  // System-generated bodies (request titles, decision/withdrawal events) are
  // German on the wire; localize the preview from the structured last-message
  // fields, falling back to the language-neutral body for plain messages.
  const previewDescriptor = conversation
    ? parentThreadPreviewI18nDescriptor({
        last_message_kind: conversation.last_message_kind,
        last_event_type: conversation.last_event_type,
        last_request_type: conversation.last_request_type,
        last_request_status: conversation.last_request_status,
      })
    : null;
  const previewBody = previewDescriptor
    ? tMsg(previewDescriptor.key, previewDescriptor.values)
    : conversation?.last_message_body;
  return (
    <section
      className={
        mobile
          ? "min-w-0 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm"
          : "min-w-0 rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
      }
    >
      <div className="flex min-w-0 items-start justify-between gap-4">
        <PanelHeader
          eyebrow={t("messages.eyebrow")}
          title={t("messages.title")}
          description={t("messages.description")}
        />
        {!composeDisabled && (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={onCompose}
            className="shrink-0"
          >
            <MotoConceptIcon
              concept="parentConversations"
              size={18}
              className="mr-1.5"
            />
            {t("messages.compose")}
          </Button>
        )}
      </div>
      <div className="mt-4">
        {!conversation || !conversation.last_message_at ? (
          <p className="text-sm leading-6 text-gray-600">
            {t("messages.empty")}
          </p>
        ) : (
          <button
            type="button"
            onClick={() => router.push(`/parents/messages/${studentId}`)}
            className="flex w-full items-start justify-between gap-3 rounded-xl border border-gray-200 bg-gray-50/70 px-3 py-2 text-left transition-colors hover:bg-gray-100"
          >
            <span className="min-w-0 flex-1">
              <span className="flex min-w-0 items-center gap-2">
                <span className="truncate text-sm font-medium text-gray-900">
                  {conversation.counterpart_name}
                </span>
                <UnreadBadge count={conversation.unread} />
              </span>
              {previewBody && (
                <span className="mt-0.5 block truncate text-sm text-gray-600">
                  {previewBody}
                </span>
              )}
            </span>
            <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
              {formatChatDateTime(conversation.last_message_at)}
            </span>
          </button>
        )}
      </div>
    </section>
  );
}

function CompactInfoRow({
  label,
  value,
}: Readonly<{ label: string; value: string }>) {
  return (
    <div className="flex items-start justify-between gap-4 rounded-2xl bg-gray-50 px-4 py-3">
      <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        {label}
      </dt>
      <dd className="text-right text-sm font-semibold break-words text-gray-900">
        {value}
      </dd>
    </div>
  );
}

// "Anfrage offen" pill for the child overview's Stammdaten entry: a pending
// change request (master data or care schedule) is awaiting an OGS decision.
// The details live on the Stammdaten page; this only signals that one exists so
// the parent knows to look without opening it.
function OpenRequestBadge() {
  const tMd = useTranslations("parentMasterData");
  return (
    <span className="bg-moto-amber/15 text-moto-amber-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold">
      <Clock className="h-3 w-3" aria-hidden="true" />
      {tMd("careSchedule.pendingBadge")}
    </span>
  );
}

function BackBar() {
  const t = useTranslations("parentChildDetail");
  return (
    <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
      <Link
        href="/parents/children"
        className="inline-flex h-8 items-center gap-2 rounded-lg px-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-50 hover:text-gray-900 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      >
        <ArrowLeft className="h-4 w-4" aria-hidden="true" />
        {t("back")}
      </Link>
    </div>
  );
}

function InfoRow({ label, value }: Readonly<{ label: string; value: string }>) {
  return (
    <div className="grid gap-1 px-5 py-4 sm:grid-cols-[12rem_minmax(0,1fr)] sm:px-6">
      <dt className="text-xs font-semibold tracking-wide text-gray-500 uppercase sm:pt-0.5">
        {label}
      </dt>
      <dd className="mt-1 text-sm font-medium break-words text-gray-900">
        {value}
      </dd>
    </div>
  );
}

function PanelHeader({
  eyebrow,
  title,
  description,
  concept,
}: Readonly<{
  eyebrow: string;
  title: string;
  description: string;
  concept?: MotoConceptKey;
}>) {
  return (
    <header className={concept ? "flex min-w-0 items-start gap-3" : undefined}>
      {concept ? (
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10">
          <MotoConceptIcon concept={concept} size={20} />
        </span>
      ) : null}
      <div className="min-w-0">
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {eyebrow}
        </p>
        <h2 className="mt-1 text-xl font-semibold text-balance text-gray-900">
          {title}
        </h2>
        <p className="mt-1 text-sm leading-6 text-gray-600">{description}</p>
      </div>
    </header>
  );
}

function ChildDetailSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      <div className="grid gap-6 xl:grid-cols-2">
        <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
        <div className="h-64 animate-pulse rounded-2xl border border-gray-200 bg-white shadow-sm" />
      </div>
    </div>
  );
}
