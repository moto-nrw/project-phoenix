"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Avatar } from "~/components/ui/avatar";
import { Skeleton } from "~/components/ui/skeleton";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { OgsConversation } from "~/components/parent/ogs-conversation";
import { ChevronRight } from "lucide-react";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  type Child,
  type ThreadSummary,
  listMessageThreads,
  listMyChildren,
} from "~/lib/parent-api";
import { parentThreadPreviewI18nDescriptor } from "~/lib/messaging-status";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";
import { parentPath } from "~/lib/parent-url";

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
  readonly lastMessageAt?: string;
  readonly lastSenderKind?: "guardian" | "staff";
  readonly lastMessageBody?: string;
  readonly lastMessageKind?: "message" | "event" | "request";
  readonly lastEventType?: string;
  readonly lastRequestType?: string;
  readonly lastRequestStatus?: string;
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
      lastMessageAt: thread?.last_message_at,
      lastSenderKind: thread?.last_sender_kind,
      lastMessageBody: thread?.last_message_body,
      lastMessageKind: thread?.last_message_kind,
      lastEventType: thread?.last_event_type,
      lastRequestType: thread?.last_request_type,
      lastRequestStatus: thread?.last_request_status,
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

  if (loading) return <Skeleton className="h-72 w-full rounded-2xl" />;

  if (error) return <Alert type="error" message={t("loadError")} />;

  // Ein Kind: die Seite IST die Unterhaltung.
  if (children.length === 1) {
    return <OgsConversation studentId={children[0]!.student_id} />;
  }

  if (children.length === 0) {
    return (
      <p className="rounded-2xl border border-gray-200 bg-white p-5 text-[17px] text-gray-600 shadow-sm">
        {t("noChildren")}
      </p>
    );
  }

  return (
    <div className="space-y-4">
      {/* Siehe Kalender: die Kopfzeile traegt den sichtbaren Titel. */}
      <h1 className="sr-only">{t("title")}</h1>
      <ul className="divide-y divide-gray-200 overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm">
        {rows.map((row) => (
          <li key={row.studentId}>
            <ConversationRow row={row} />
          </li>
        ))}
      </ul>
    </div>
  );
}

function ConversationRow({ row }: Readonly<{ row: ChildConversation }>) {
  const t = useTranslations("parentMessages");
  const tMsg = useTranslations("parentOgsMessaging");
  const timestamp = formatChatDateTime(row.lastMessageAt);
  // Systemtexte kommen deutsch von der Schnittstelle; aus den strukturierten
  // Feldern wird die Vorschau in der Sprache des Elternteils gebaut.
  const descriptor = parentThreadPreviewI18nDescriptor({
    last_message_kind: row.lastMessageKind,
    last_event_type: row.lastEventType,
    last_request_type: row.lastRequestType,
    last_request_status: row.lastRequestStatus,
  });
  const body = descriptor
    ? tMsg(descriptor.key, descriptor.values)
    : (row.lastMessageBody ?? "");
  const preview = body
    ? row.lastSenderKind === "staff"
      ? t("previewFromOgs", { text: body })
      : row.lastSenderKind === "guardian"
        ? t("previewFromYou", { text: body })
        : body
    : t("noMessagesYet");
  const unread = row.unread > 0;

  return (
    <Link
      href={parentPath(`/parents/messages/${row.studentId}`)}
      className="flex min-h-[72px] w-full items-center gap-3 px-4 py-3 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-[#5080D8] focus-visible:-outline-offset-2 focus-visible:outline-none active:bg-gray-100"
    >
      <Avatar
        name={row.studentName}
        decorative
        className="size-11 shrink-0 text-[15px]"
      />
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-baseline justify-between gap-2">
          <span
            className={`min-w-0 truncate text-[17px] text-gray-900 ${unread ? "font-bold" : "font-semibold"}`}
          >
            {row.studentName}
          </span>
          {timestamp && (
            <span className="shrink-0 text-[15px] text-gray-500">
              {timestamp}
            </span>
          )}
        </span>
        <span className="mt-0.5 flex items-center gap-2">
          <span
            className={`min-w-0 flex-1 truncate text-[15px] ${unread ? "text-gray-700" : "text-gray-500"}`}
          >
            {preview}
          </span>
          <UnreadBadge count={row.unread} />
        </span>
      </span>
      <ChevronRight
        className="h-5 w-5 shrink-0 text-gray-400"
        aria-hidden="true"
      />
    </Link>
  );
}
