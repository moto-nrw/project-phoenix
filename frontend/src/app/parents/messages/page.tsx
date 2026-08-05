"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useTranslations } from "next-intl";
import { ArrowRight } from "lucide-react";
import { OgsConversation } from "~/components/parent/ogs-conversation";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { Alert } from "~/components/ui/alert";
import { Skeleton } from "~/components/ui/skeleton";
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
import { ConceptPageHeader } from "~/components/ui/concept-section-header";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

const logger = createLogger({ component: "ParentMessagesPage" });

// One entry per child for parents with more than one child — picking a child
// opens that child's conversation with the OGS.
interface ChildConversation {
  readonly studentId: string;
  readonly studentName: string;
  readonly counterpart: string;
  readonly lastMessageAt?: string;
  readonly lastSenderKind?: "guardian" | "staff";
  readonly lastMessageBody?: string;
  // Structured last-message fields so a request title / decision / withdrawal
  // preview is localized from fields instead of the German lastMessageBody.
  readonly lastMessageKind?: "message" | "event" | "request";
  readonly lastEventType?: string;
  readonly lastRequestType?: string;
  readonly lastRequestStatus?: string;
  readonly unread: number;
}

// previewLine prefixes the (already-localized) last-message body with who sent
// it. `body` is the localized preview the caller resolved from the structured
// fields, falling back to the raw lastMessageBody for plain messages.
function previewLine(row: ChildConversation, body: string): string {
  if (!body) return "";
  const prefix =
    row.lastSenderKind === "staff"
      ? "OGS: "
      : row.lastSenderKind === "guardian"
        ? "Sie: "
        : "";
  return `${prefix}${body}`;
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
      counterpart:
        thread?.counterpart_name ?? `OGS ${child.school_name}`.trim(),
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

  // Most recent activity first; children without a conversation sort to the
  // bottom, alphabetically. Compare timestamps as epoch millis, not lexically:
  // a lexical compare only holds while every value shares one exact string format,
  // but Go's JSON trims trailing fractional-second zeros (and offsets vary), so a
  // whole-second timestamp can mis-sort against a sub-second one.
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

export default function ParentMessagesPage() {
  const [children, setChildren] = useState<Child[]>([]);
  const [rows, setRows] = useState<ChildConversation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Latest-wins guard: focus and the SSE parent-threads-refresh both fire
  // load({silent:true}), so two background fetches can overlap. Without a token
  // an earlier-started-but-later-resolving run would clobber newer data with a
  // stale snapshot. Each run claims the next token; only the most-recently-started
  // one may setState. Mirrors OgsConversation / useChildCare.
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
      // Only the multi-child picker needs the thread list. The single-child
      // path renders <OgsConversation>, which fetches its own conversation, so
      // skip the cross-tenant ListThreadsForGuardianTenants query (admin tx +
      // multi-join) for the common single-child guardian — it was fetched in
      // parallel and discarded before.
      const nextRows =
        childList.length > 1
          ? buildRows(childList, await listMessageThreads())
          : [];
      // Apply only if this is still the latest run (and we're mounted), so a
      // slow run resolving after a newer one can't overwrite fresher data.
      if (!mountedRef.current || seq !== loadSeqRef.current) return;
      setChildren(childList);
      setRows(nextRows);
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Unknown error";
      logger.warn("parent_messages_load_failed", { error: message });
      // A silent (background) refresh must not replace the visible list with an
      // error screen — keep the current data and just log.
      if (!silent && mountedRef.current && seq === loadSeqRef.current) {
        setError(message);
      }
    } finally {
      // Release the initial skeleton once the LATEST run settles, regardless of
      // whether it was silent. If a silent refresh (SSE/focus) starts before the
      // initial non-silent load resolves, it bumps loadSeqRef: the initial run
      // then skips this finally (stale seq) and the silent run would skip it too
      // (silent), leaving `loading` stuck true and the skeleton rendered forever
      // even though the silent run already populated children/rows. Clearing on
      // the latest run regardless of silent fixes that; a redundant setLoading(false)
      // on a silent run is a no-op since loading is only ever set true by !silent.
      if (mountedRef.current && seq === loadSeqRef.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  // Keep the list fresh without a full skeleton flash: the portal-wide SSE
  // bridge dispatches parent-threads-refresh on new activity. Route it through
  // the shared useMessagesActivity hook (eventName override) like OgsConversation
  // does, instead of a hand-rolled addEventListener — one place owns the
  // matching + backgrounded-tab deferral.
  const refreshThreads = useCallback(() => void load({ silent: true }), [load]);
  useMessagesActivity({
    eventName: "parent-threads-refresh",
    onMatch: refreshThreads,
    // Refetch-only multi-child list (never advances a read cursor), so fire even
    // in a background tab instead of deferring to focus.
    marksRead: false,
  });
  // Also refetch when the window regains focus (the hook covers SSE, not focus).
  useEffect(() => {
    const refresh = () => void load({ silent: true });
    window.addEventListener("focus", refresh);
    return () => {
      window.removeEventListener("focus", refresh);
    };
  }, [load]);

  if (loading) {
    return <ParentMessagesSkeleton />;
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl">
        <Alert
          type="error"
          message="Die Nachrichten konnten nicht geladen werden."
        />
      </div>
    );
  }

  // Single child (the common case): "Nachrichten" IS the conversation with the
  // OGS — no child list, no child as the counterpart.
  if (children.length === 1) {
    return <OgsConversation studentId={children[0]!.student_id} />;
  }

  if (children.length === 0) {
    return (
      <div className="mx-auto w-full max-w-7xl space-y-6">
        <Hero />
        <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
          <EmptyMessages />
        </section>
      </div>
    );
  }

  // Several children: pick which child's conversation with the OGS to open.
  return (
    <div className="mx-auto w-full max-w-7xl space-y-6">
      <Hero />
      <section className="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6">
        <h2 className="mb-3 text-sm font-semibold text-gray-900">
          Für welches Kind?
        </h2>
        <ul className="divide-y divide-gray-100">
          {rows.map((row) => (
            <ChildRow key={row.studentId} row={row} />
          ))}
        </ul>
      </section>
    </div>
  );
}

function Hero() {
  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="p-5 sm:p-6 lg:p-8">
        <ConceptPageHeader
          title="Nachrichten"
          eyebrow="Austausch mit der OGS"
          concept="parentConversations"
          subtitle="Schreiben Sie der OGS und lesen Sie die Antworten des Teams. Pro Kind gibt es eine Unterhaltung."
        />
      </div>
    </section>
  );
}

function ChildRow({ row }: Readonly<{ row: ChildConversation }>) {
  const t = useTranslations("parentOgsMessaging");
  const timestamp = formatChatDateTime(row.lastMessageAt);
  // System-generated bodies (request titles, decision/withdrawal events) are
  // German on the wire; render them localized from the structured fields,
  // falling back to the language-neutral body for plain messages.
  const previewDescriptor = parentThreadPreviewI18nDescriptor({
    last_message_kind: row.lastMessageKind,
    last_event_type: row.lastEventType,
    last_request_type: row.lastRequestType,
    last_request_status: row.lastRequestStatus,
  });
  const previewBody = previewDescriptor
    ? t(previewDescriptor.key, previewDescriptor.values)
    : (row.lastMessageBody ?? "");
  const preview = previewLine(row, previewBody);
  const unread = row.unread > 0;
  return (
    <li>
      <Link
        href={`/parents/messages/${row.studentId}`}
        className="group flex items-start gap-4 px-1 py-4 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:rounded-xl sm:px-3"
      >
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100">
          <MotoConceptIcon concept="parentConversations" size={22} />
        </span>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-start justify-between gap-2">
            <h3
              className={`min-w-0 truncate text-base text-gray-900 ${
                unread ? "font-bold" : "font-semibold"
              }`}
            >
              {row.studentName}
            </h3>
            <div className="flex shrink-0 items-center gap-2">
              <UnreadBadge count={row.unread} />
              {timestamp ? (
                <span className="text-xs text-gray-400">{timestamp}</span>
              ) : null}
            </div>
          </div>
          {preview ? (
            <p
              className={`mt-0.5 truncate text-sm ${
                unread ? "text-gray-700" : "text-gray-500"
              }`}
            >
              {preview}
            </p>
          ) : (
            <p className="mt-0.5 truncate text-sm text-gray-400">
              Noch keine Nachrichten. Tippen, um zu schreiben.
            </p>
          )}
        </div>
        <ArrowRight
          className="mt-1 hidden h-4 w-4 shrink-0 text-gray-400 transition-colors group-hover:text-gray-700 sm:block"
          aria-hidden="true"
        />
      </Link>
    </li>
  );
}

function EmptyMessages() {
  return (
    <div className="rounded-xl border border-dashed border-gray-300 bg-gray-50 p-8 text-center">
      <span className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-gray-100">
        <MotoConceptIcon concept="parentConversations" size={22} />
      </span>
      <h2 className="mt-3 text-sm font-semibold text-gray-900">
        Für Ihr Konto ist noch kein Kind hinterlegt
      </h2>
      <p className="mt-1 text-sm leading-6 text-gray-600">
        Sobald ein Kind verknüpft ist, können Sie hier mit der OGS schreiben.
      </p>
    </div>
  );
}

function ParentMessagesSkeleton() {
  return (
    <div className="mx-auto w-full max-w-7xl">
      <Skeleton className="h-[calc(100dvh-13rem)] min-h-[20rem] rounded-2xl border border-gray-200 bg-white shadow-sm lg:h-[calc(100dvh-8.5rem)]" />
    </div>
  );
}
