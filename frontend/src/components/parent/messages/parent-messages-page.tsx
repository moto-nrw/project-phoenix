"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useLocale, useTranslations } from "next-intl";
import { CaretRightIcon, ChecksIcon } from "@phosphor-icons/react/ssr";
import { Alert } from "~/components/ui/alert";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { OgsConversation } from "~/components/parent/ogs-conversation";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  type Child,
  type ThreadSummary,
  listMessageThreads,
  listMyChildren,
} from "~/lib/parent-api";
import { parentThreadPreviewI18nDescriptor } from "~/lib/messaging-status";
import { createLogger } from "~/lib/logger";
import { formatChatClockTime, formatChatDateTime } from "~/lib/date-helpers";
import { parentPath } from "~/lib/parent-url";
import {
  ParentPage,
  ParentPageHeader,
  ParentSectionSkeleton,
} from "~/components/parent/parent-page";

const logger = createLogger({ component: "ParentMessagesPage" });

/**
 * Nachrichten mit der OGS.
 *
 * Bei einem Kind IST diese Seite die Unterhaltung, ohne Zwischenschritt. Bei
 * mehreren steht hier eine Zeile je Kind mit letzter Nachricht, Zeitpunkt und
 * Ungelesen-Zaehler.
 */

interface ChildConversation {
  readonly studentId: string;
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
  const byStudent = new Map<string, ThreadSummary>();
  for (const thread of threads) byStudent.set(thread.student_id, thread);

  const rows = children.map((child): ChildConversation => {
    const thread = byStudent.get(child.student_id);
    return {
      studentId: child.student_id,
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
  const [children, setChildren] = useState<Child[]>([]);
  const [rows, setRows] = useState<ChildConversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  // Latest-wins: Fokus und SSE loesen beide einen stillen Nachladelauf aus,
  // ein aelterer darf einen neueren nie ueberschreiben.
  const loadSeqRef = useRef(0);
  const mountedRef = useRef(true);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
    };
  }, []);

  const load = useCallback(async (opts?: { silent?: boolean }) => {
    const silent = opts?.silent ?? false;
    const seq = ++loadSeqRef.current;
    if (!silent) setLoading(true);
    try {
      const childList = await listMyChildren();
      // Nur die Mehr-Kinder-Liste braucht die Uebersicht der Unterhaltungen.
      const nextRows =
        childList.length > 1
          ? buildRows(childList, await listMessageThreads())
          : [];
      if (!mountedRef.current || seq !== loadSeqRef.current) return;
      setChildren(childList);
      setRows(nextRows);
      setError(false);
    } catch (err) {
      logger.warn("parent_messages_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (!silent && mountedRef.current && seq === loadSeqRef.current) {
        setError(true);
      }
    } finally {
      if (mountedRef.current && seq === loadSeqRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const refreshThreads = useCallback(() => void load({ silent: true }), [load]);
  useMessagesActivity({
    eventName: "parent-threads-refresh",
    onMatch: refreshThreads,
    marksRead: false,
  });
  useEffect(() => {
    const refresh = () => void load({ silent: true });
    window.addEventListener("focus", refresh);
    return () => {
      window.removeEventListener("focus", refresh);
    };
  }, [load]);

  if (loading) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
        <ParentSectionSkeleton rows={3} showHeader={false} />
      </ParentPage>
    );
  }

  if (error) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
        <Alert type="error" message={t("loadError")} />
      </ParentPage>
    );
  }

  // Ein Kind: die Seite IST die Unterhaltung.
  if (children.length === 1) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
        <OgsConversation studentId={children[0]!.student_id} showChild />
      </ParentPage>
    );
  }

  if (children.length === 0) {
    return (
      <ParentPage>
        <ParentPageHeader
          kicker={t("kicker")}
          title={t("title")}
          description={t("description")}
        />
        <p className="moto-content-surface rounded-2xl border p-5 text-sm leading-6 text-gray-600 shadow-sm backdrop-blur-md">
          {t("noChildren")}
        </p>
      </ParentPage>
    );
  }

  return (
    <ParentPage>
      <ParentPageHeader
        kicker={t("kicker")}
        title={t("title")}
        description={t("description")}
      />
      <ul className="moto-content-surface divide-y divide-gray-200 overflow-hidden rounded-2xl border shadow-sm">
        {rows.map((row) => (
          <li key={row.studentId}>
            <ConversationRow row={row} />
          </li>
        ))}
      </ul>
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
    <Link
      href={parentPath(`/parents/messages/${row.studentId}`)}
      className="flex min-h-[88px] w-full items-center gap-3 px-4 py-3.5 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:-outline-offset-2 focus-visible:outline-none active:bg-gray-100 sm:px-5"
    >
      <ConceptIconTile
        concept="messages"
        variant="page"
        className="rounded-full bg-[#EDF3FD]"
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
            {t("ogsTeam", { school: row.schoolName })}
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
    </Link>
  );
}
