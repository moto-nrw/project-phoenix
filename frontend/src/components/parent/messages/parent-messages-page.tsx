"use client";

import { useCallback, useMemo, useEffect } from "react";
import { useLocale, useTranslations } from "next-intl";
import { CaretRightIcon, ChecksIcon } from "@phosphor-icons/react/ssr";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import NavigationLink from "~/components/ui/navigation-link";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { SharedParentFeaturesProvider } from "~/components/parent/child-requests-context";
import { OgsConversation } from "~/components/parent/ogs-conversation";
import { GroupThreadsSection } from "~/components/parent/messages/group-threads-section";
import {
  type Child,
  type ThreadSummary,
  listGroupThreads,
  listMessageThreads,
} from "~/lib/parent-api";
import {
  type MessageAudience,
  childConversationPath,
  parentThreadPreviewI18nDescriptor,
} from "~/lib/messaging-status";
import { useSWRAuth, useInvalidateMatching } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";
import {
  useParentChildren,
  PARENT_RESOURCE_OPTIONS,
} from "~/lib/hooks/use-parent-resources";
import { formatChatClockTime, formatChatDateTime } from "~/lib/date-helpers";
import { parentPath } from "~/lib/parent-url";
import {
  ParentPage,
  ParentPageHeader,
  ParentSectionSkeleton,
} from "~/components/parent/parent-page";
import { messagesConcept } from "~/lib/concepts/messages";

/**
 * Nachrichten mit der OGS.
 *
 * Bei einem Kind IST diese Seite die Unterhaltung, ohne Zwischenschritt. Bei
 * mehreren steht hier eine Zeile je Kind mit letzter Nachricht, Zeitpunkt und
 * Ungelesen-Zaehler.
 */

const EMPTY_CHILDREN: Child[] = [];

interface ChildConversation {
  readonly studentId: string;
  /** Team or leadership only (#5200); the leadership row exists once begun. */
  readonly audience: MessageAudience;
  readonly studentName: string;
  readonly schoolName: string;
  readonly lastMessageAt?: string;
  readonly lastSenderKind?: "guardian" | "staff";
  readonly lastMessageBody?: string;
  readonly lastMessageKind?: "message" | "event" | "request";
  readonly lastEventType?: string;
  readonly lastRequestType?: string;
  readonly lastRequestStatus?: string;
  readonly lastMessagePayload?: Record<string, unknown>;
  readonly lastMessageReadByStaff: boolean;
  readonly unread: number;
}

function buildRows(
  children: readonly Child[],
  threads: readonly ThreadSummary[],
): ChildConversation[] {
  // Je Kind die Team-Unterhaltung, auch noch ohne Nachricht, und die mit der
  // Leitung, sobald es sie gibt (#5200).
  const teamByStudent = new Map<string, ThreadSummary>();
  const leadership: ThreadSummary[] = [];
  for (const thread of threads) {
    if (thread.audience === "leadership") leadership.push(thread);
    else teamByStudent.set(thread.student_id, thread);
  }
  const childById = new Map(children.map((child) => [child.student_id, child]));
  const entries: { child: Child; thread?: ThreadSummary }[] = [
    ...children.map((child) => ({
      child,
      thread: teamByStudent.get(child.student_id),
    })),
    ...leadership.flatMap((thread) => {
      const child = childById.get(thread.student_id);
      return child ? [{ child, thread }] : [];
    }),
  ];

  const rows = entries.map(({ child, thread }): ChildConversation => {
    return {
      studentId: child.student_id,
      audience: thread?.audience ?? "team",
      studentName: `${child.first_name} ${child.last_name}`.trim(),
      schoolName: thread?.school_name ?? child.school_name,
      lastMessageAt: thread?.last_message_at,
      lastSenderKind: thread?.last_sender_kind,
      lastMessageBody: thread?.last_message_body,
      lastMessageKind: thread?.last_message_kind,
      lastEventType: thread?.last_event_type,
      lastRequestType: thread?.last_request_type,
      lastRequestStatus: thread?.last_request_status,
      lastMessagePayload: thread?.last_message_payload,
      lastMessageReadByStaff: thread?.last_message_read_by_staff ?? false,
      unread: thread?.unread ?? 0,
    };
  });

  // Neueste Aktivitaet zuerst; Kinder ohne Unterhaltung stehen alphabetisch
  // unten. Zeitstempel als Millisekunden vergleichen, nicht als Text: Go
  // schneidet nachlaufende Nullen der Sekundenbruchteile ab.
  return rows.sort((a, b) => {
    if (a.lastMessageAt && b.lastMessageAt) {
      return (
        new Date(b.lastMessageAt).getTime() -
        new Date(a.lastMessageAt).getTime()
      );
    }
    if (a.lastMessageAt) return -1;
    if (b.lastMessageAt) return 1;
    return a.studentName.localeCompare(b.studentName);
  });
}

export function ParentMessagesPage() {
  const t = useTranslations("parentMessages");
  const tGroupThreads = useTranslations("parentGroupThreads");
  const childResource = useParentChildren();
  const children = childResource.data ?? EMPTY_CHILDREN;
  const threads = useSWRAuth(
    children.length > 1 ? parentKeys.messages() : null,
    listMessageThreads,
    PARENT_RESOURCE_OPTIONS,
  );
  const groups = useSWRAuth(
    parentKeys.groupThreads(),
    listGroupThreads,
    PARENT_RESOURCE_OPTIONS,
  );
  const groupThreads = groups.data ?? [];
  const rows = useMemo(
    () => buildRows(children, threads.data ?? []),
    [children, threads.data],
  );
  const loading =
    (!childResource.data && !childResource.error) ||
    (children.length > 1 && !threads.data && !threads.error);
  const {
    error,
    show: showLoadError,
    clear: clearLoadError,
  } = useApiLoadError();
  const {
    error: groupThreadsError,
    show: showGroupThreadsLoadError,
    clear: clearGroupThreadsLoadError,
  } = useApiLoadError();
  const invalidate = useInvalidateMatching();
  const refreshThreads = useCallback(() => {
    void invalidate(parentKeys.isResource);
  }, [invalidate]);
  const failure = childResource.error ?? threads.error;
  useEffect(() => {
    if (!failure) {
      clearLoadError();
      return;
    }
    void showLoadError(failure, {
      object: t("errorObjectList"),
      retry: refreshThreads,
    });
  }, [failure, clearLoadError, showLoadError, refreshThreads, t]);
  useEffect(() => {
    if (!groups.error) {
      clearGroupThreadsLoadError();
      return;
    }
    void showGroupThreadsLoadError(groups.error, {
      object: tGroupThreads("errorObjectList"),
      retry: refreshThreads,
    });
  }, [
    groups.error,
    clearGroupThreadsLoadError,
    showGroupThreadsLoadError,
    refreshThreads,
    tGroupThreads,
  ]);

  if (loading) {
    return (
      <ParentPage>
        <div data-parent-tour="messages">
          <ParentPageHeader
            kicker={t("kicker")}
            title={t("title")}
            description={t("description")}
          />
        </div>
        <ParentSectionSkeleton rows={3} showHeader={false} />
      </ParentPage>
    );
  }

  if (error) {
    return (
      <ParentPage>
        <div data-parent-tour="messages">
          <ParentPageHeader
            kicker={t("kicker")}
            title={t("title")}
            description={t("description")}
          />
        </div>
        <LoadErrorAlert error={error} />
      </ParentPage>
    );
  }

  // Ein Kind: die Seite IST die Unterhaltung.
  if (children.length === 1) {
    return (
      <ParentPage>
        {/* Bei offener Handy-Tastatur braucht der Chat den Platz, sonst liegt
            das Schreibfeld unter der Tastatur (#3664). */}
        <div
          className="in-data-chat-keyboard:hidden"
          data-parent-tour="messages"
        >
          <ParentPageHeader
            kicker={t("kicker")}
            title={t("title")}
            description={t("description")}
          />
        </div>
        {/* Gruppennachrichten der OGS (#4784) stehen kompakt ueber dem Chat,
            der den Rest des Bildschirms fuellt. */}
        {(groupThreads.length > 0 || groupThreadsError) && (
          <div className="in-data-chat-keyboard:hidden">
            <GroupThreadsSection
              threads={groupThreads}
              compact
              error={groupThreadsError}
            />
          </div>
        )}
        <SharedParentFeaturesProvider linkedChildren={children}>
          <OgsConversation studentId={children[0]!.student_id} showChild />
        </SharedParentFeaturesProvider>
      </ParentPage>
    );
  }

  if (children.length === 0) {
    return (
      <ParentPage>
        <div data-parent-tour="messages">
          <ParentPageHeader
            kicker={t("kicker")}
            title={t("title")}
            description={t("description")}
          />
        </div>
        <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
          {t("noChildren")}
        </p>
      </ParentPage>
    );
  }

  return (
    <ParentPage>
      <div data-parent-tour="messages">
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
      </div>
      <ul className="moto-content-surface divide-y divide-gray-200 overflow-hidden rounded-2xl border shadow-sm">
        {rows.map((row) => (
          <li key={`${row.studentId}:${row.audience}`}>
            <ConversationRow row={row} />
          </li>
        ))}
      </ul>
      <GroupThreadsSection threads={groupThreads} error={groupThreadsError} />
    </ParentPage>
  );
}

function ConversationRow({ row }: Readonly<{ row: ChildConversation }>) {
  const t = useTranslations("parentMessages");
  const tMsg = useTranslations("parentOgsMessaging");
  const locale = useLocale();
  const timestamp = formatChatDateTime(row.lastMessageAt, locale);
  const compactTimestamp = row.lastMessageAt
    ? formatChatClockTime(row.lastMessageAt, locale)
    : "";
  // Systemtexte kommen deutsch von der Schnittstelle; aus den strukturierten
  // Feldern wird die Vorschau in der Sprache des Elternteils gebaut.
  const descriptor = parentThreadPreviewI18nDescriptor(
    {
      last_message_kind: row.lastMessageKind,
      last_event_type: row.lastEventType,
      last_request_type: row.lastRequestType,
      last_request_status: row.lastRequestStatus,
      last_message_payload: row.lastMessagePayload,
    },
    locale,
  );
  const body = descriptor
    ? tMsg(descriptor.key, descriptor.values)
    : (row.lastMessageBody ?? "");
  const preview = body
    ? row.lastSenderKind === "staff"
      ? body
      : row.lastSenderKind === "guardian"
        ? t("previewFromYou", { text: body })
        : body
    : t("noMessagesYet");
  const unread = row.unread > 0;
  const ownLastMessage = row.lastSenderKind === "guardian";

  return (
    <NavigationLink
      href={parentPath(childConversationPath(row.studentId, row.audience))}
      className="flex min-h-[88px] w-full items-center gap-3 px-4 py-3.5 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:-outline-offset-2 focus-visible:outline-none active:bg-gray-100 sm:px-5"
    >
      <ConceptIconTile
        concept={messagesConcept}
        variant="page"
        className="bg-moto-blue-soft rounded-full"
      />
      <span className="min-w-0 flex-1 space-y-0.5">
        <span className="flex min-w-0 items-center justify-between gap-2">
          <span className="min-w-0 truncate text-xs font-medium text-gray-500">
            {t("aboutChild", { name: row.studentName })}
          </span>
          {timestamp && (
            <span
              aria-label={timestamp}
              className={`shrink-0 text-xs tabular-nums ${unread ? "text-moto-blue font-semibold" : "text-gray-500"}`}
            >
              <span aria-hidden="true" className="sm:hidden">
                {compactTimestamp}
              </span>
              <span aria-hidden="true" className="hidden sm:inline">
                {timestamp}
              </span>
            </span>
          )}
        </span>
        <span className="flex min-w-0 items-center justify-between gap-2">
          <span
            className={`min-w-0 truncate text-base text-gray-900 ${unread ? "font-bold" : "font-semibold"}`}
          >
            {t(row.audience === "leadership" ? "ogsLeadership" : "ogsTeam", {
              school: row.schoolName,
            })}
          </span>
          <UnreadBadge count={row.unread} />
        </span>
        <span className="flex min-w-0 items-center gap-1.5">
          {ownLastMessage && (
            <span
              className={
                row.lastMessageReadByStaff
                  ? "text-moto-blue shrink-0"
                  : "shrink-0 text-gray-500"
              }
              role="img"
              aria-label={
                row.lastMessageReadByStaff ? t("readByOgs") : t("sent")
              }
              title={row.lastMessageReadByStaff ? t("readByOgs") : t("sent")}
            >
              <ChecksIcon size={17} weight="bold" aria-hidden="true" />
            </span>
          )}
          <span
            className={`min-w-0 flex-1 truncate text-sm ${unread ? "text-gray-700" : "text-gray-500"}`}
          >
            {preview}
          </span>
        </span>
      </span>
      <CaretRightIcon
        size={20}
        weight="bold"
        className="shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </NavigationLink>
  );
}
