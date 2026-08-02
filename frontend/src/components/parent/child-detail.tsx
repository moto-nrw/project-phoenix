"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  CalendarClock,
  ClipboardList,
  Clock,
  HeartPulse,
  MessageCircle,
  UserRound,
} from "lucide-react";
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
import { SectionCard } from "~/components/ui/section-card";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { formatChatDateTime } from "~/lib/date-helpers";
import {
  ChildAvatar,
  initialsFromFullName,
} from "~/components/parent/child-row";
import {
  ParentField,
  ParentFieldGrid,
  ParentLinkAction,
  ParentLoadError,
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
  ParentStatusRow,
} from "~/components/parent/parent-page";

const logger = createLogger({ component: "ChildDetail" });

// The three actions a guardian actually performs from this page. The former
// six-tile grid also carried three "coming soon" stubs (Abholrecht, Personen,
// Neuigkeiten) that were permanently disabled and whose real content already
// lives further down the page or in the sidebar — they were pure noise.
const CHILD_ACTIONS = [
  { key: "sick", icon: HeartPulse },
  { key: "pickupTime", icon: CalendarClock },
  { key: "message", icon: MessageCircle },
] as const;

// Modal-backed quick actions. "message" is handled separately — it navigates to
// the conversation rather than opening a modal.
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
    return <ParentPageSkeleton rows={3} />;
  }

  if (error) {
    return <ParentLoadError message={t("loadError")} />;
  }

  if (!child) {
    return (
      <ParentPage>
        <ParentPageHeader
          backHref="/parents/children"
          backLabel={t("back")}
          title={t("masterDataTitle")}
          description={t("notFound")}
        />
      </ParentPage>
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

  const subtitle = useMemo(
    () =>
      child.school_class
        ? `${child.school_name}, ${child.school_class}`
        : child.school_name,
    [child.school_name, child.school_class],
  );

  return (
    <ParentPage>
      <ParentPageHeader
        backHref="/parents/children"
        backLabel={t("back")}
        kicker={t("childEyebrow")}
        title={fullName}
        description={subtitle}
        media={
          <ChildAvatar initials={initialsFromFullName(fullName)} size="lg" />
        }
      />

      <SectionCard
        icon={ClipboardList}
        title={t("today.title")}
        description={t("today.description")}
        bodyClassName="mt-4 space-y-4"
      >
        <div className="grid gap-2 sm:grid-cols-3">
          <ParentStatusRow icon={HeartPulse} label={t("today.sickLabel")}>
            <SickStatusSummary
              sickDays={care.sickDays}
              excusedRequests={care.excusedRequests}
              onWithdraw={care.withdrawExcused}
            />
          </ParentStatusRow>
          <ParentStatusRow icon={CalendarClock} label={t("today.pickupLabel")}>
            <TodayPickupValue pickup={care.todayPickup} t={t} />
          </ParentStatusRow>
          <ParentStatusRow
            icon={MessageCircle}
            label={t("today.messagesLabel")}
          >
            <MessagesSummary threads={threads} t={t} />
          </ParentStatusRow>
        </div>

        <div className="flex flex-wrap gap-2">
          {CHILD_ACTIONS.map((action) => (
            <QuickAction
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
      </SectionCard>

      <SectionCard
        icon={UserRound}
        kicker={t("masterDataEyebrow")}
        title={t("masterDataTitle")}
        description={t("masterDataDescription")}
        actions={
          <ParentLinkAction
            href={`/parents/children/${child.student_id}/stammdaten`}
          >
            {tMd("openLink")}
          </ParentLinkAction>
        }
        bodyClassName="mt-4 space-y-3"
      >
        {care.features.has_open_change_request && <OpenRequestBadge />}
        <ParentFieldGrid className="sm:grid-cols-3">
          <ParentField label={t("schoolLabel")}>
            {child.school_name}
          </ParentField>
          <ParentField label={t("classLabel")}>
            {child.school_class || t("notSet")}
          </ParentField>
          <ParentField label={t("careLabel")}>
            {getServiceRange(child, locale, t)}
          </ParentField>
        </ParentFieldGrid>
      </SectionCard>

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
    </ParentPage>
  );
}

/**
 * One quick action. A labelled button, not a tile: these are three ordinary
 * actions, and the old aspect-square icon grid gave them the visual weight of
 * a dashboard while wrapping their labels over three lines on mobile.
 */
function QuickAction({
  action,
  onClick,
}: Readonly<{ action: ChildAction; onClick?: () => void }>) {
  const t = useTranslations("parentChildDetail");
  const Icon = action.icon;
  const label = t(`actions.${action.key}.label`);
  const enabled = Boolean(onClick);
  return (
    <Button
      type="button"
      variant={enabled ? "outline" : "secondary"}
      size="md"
      onClick={onClick}
      disabled={!enabled}
      className="max-sm:w-full"
      aria-label={enabled ? label : t("comingSoonAria", { label })}
    >
      <Icon className="mr-2 h-4 w-4 shrink-0" aria-hidden="true" />
      {label}
    </Button>
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
            <span className="ml-1 font-normal text-gray-500">
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

function MessagesSummary({
  threads,
  t,
}: Readonly<{ threads: ThreadSummary[]; t: ChildDetailTranslator }>) {
  const unread = threads.reduce((sum, thread) => sum + thread.unread, 0);
  if (threads.length === 0) return <>{t("today.noConversations")}</>;
  if (unread > 0) return <>{t("today.unreadCount", { count: unread })}</>;
  return <>{t("today.conversationsCount", { count: threads.length })}</>;
}

// Shows this child's OGS conversation (filtered to the child) with an unread
// pill and last activity. The row opens the thread; "Neue Nachricht" starts a
// new conversation pre-selected to this child. Mirrors the staff-side
// ParentMessagesCard.
function ChildMessagesPanel({
  studentId,
  threads,
  onCompose,
  composeDisabled = false,
}: Readonly<{
  studentId: string;
  threads: ThreadSummary[];
  onCompose: () => void;
  composeDisabled?: boolean;
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
    <SectionCard
      icon={MessageCircle}
      title={t("messages.title")}
      description={t("messages.description")}
      actions={
        composeDisabled ? undefined : (
          <Button type="button" variant="primary" size="md" onClick={onCompose}>
            <MessageCircle className="mr-1.5 h-4 w-4" aria-hidden="true" />
            {t("messages.compose")}
          </Button>
        )
      }
    >
      {!conversation || !conversation.last_message_at ? (
        <p className="text-sm leading-6 text-gray-600">{t("messages.empty")}</p>
      ) : (
        <button
          type="button"
          onClick={() => router.push(`/parents/messages/${studentId}`)}
          className="flex w-full items-start justify-between gap-3 rounded-xl border border-gray-200 bg-white px-3 py-2.5 text-left transition-colors hover:border-gray-300 hover:bg-gray-50"
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
          <span className="shrink-0 text-xs whitespace-nowrap text-gray-400">
            {formatChatDateTime(conversation.last_message_at)}
          </span>
        </button>
      )}
    </SectionCard>
  );
}

// "Anfrage offen" pill for the child overview's Stammdaten entry: a pending
// change request (master data or care schedule) is awaiting an OGS decision.
// The details live on the Stammdaten page; this only signals that one exists so
// the parent knows to look without opening it.
function OpenRequestBadge() {
  const tMd = useTranslations("parentMasterData");
  return (
    <span className="inline-flex items-center gap-1 rounded-full bg-[#EAB308]/15 px-2 py-0.5 text-xs font-semibold text-[#92710b]">
      <Clock className="h-3 w-3" aria-hidden="true" />
      {tMd("careSchedule.pendingBadge")}
    </span>
  );
}
