"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import useSWR from "swr";
import { MessagesSquare } from "lucide-react";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { EmptyState } from "~/components/ui/empty-state";
import { TileCard } from "~/components/ui/tile-card";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { NewTeamMessageModal } from "~/components/messaging/new-team-message-modal";
import { TeamChatSkeleton } from "~/components/messaging/team-chat-skeletons";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import {
  type StaffInboxThread,
  isStaffMessagingDisabled,
  staffRoleKindLabel,
} from "~/lib/staff-messages-api";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";

const logger = createLogger({ component: "TeamChatInbox" });

const DISABLED_TITLE = "Der Team-Chat ist ausgeschaltet";
const DISABLED_DESCRIPTION =
  "Ihre Schule hat den Team-Chat nicht eingeschaltet. Wenden Sie sich an die OGS-Leitung, wenn Sie ihn nutzen möchten.";

/** Leerzustand des Posteingangs — Form wie `TenantPage.empty`. */
interface TeamChatEmptyState {
  readonly icon?: ReactNode;
  readonly title: string;
  readonly description?: string;
  readonly action?: ReactNode;
}

/**
 * Was die Hülle eines Portals vom Posteingang bekommt. Die Logik (Laden,
 * Filtern, Aus-Zustand, Fehler) ist eine; nur die Hülle unterscheidet sich:
 * das OGS-Portal rendert `TenantPage` als Wurzel, das Schul-Portal seine
 * eigene Kopfzeile. Beide bauen aus denselben Teilen.
 */
export interface TeamChatInboxParts {
  readonly title: string;
  /** Statuszeile aus echten Zahlen: Unterhaltungen und ungelesene. */
  readonly stats: string;
  readonly statsLoading: boolean;
  readonly chatEnabled: boolean;
  /** Zahl der sichtbaren (gefilterten) Unterhaltungen — für die Zähler-Plakette. */
  readonly count: number;
  readonly search: {
    readonly value: string;
    readonly onChange: (value: string) => void;
    readonly placeholder: string;
  };
  readonly onlyUnread: boolean;
  readonly setOnlyUnread: (next: boolean) => void;
  /** Hauptaktion; `null`, solange der Chat ausgeschaltet ist. */
  readonly composeButton: ReactNode | null;
  /** Skelett statt Liste. */
  readonly loading: boolean;
  /** Ersetzt die Liste, sobald nichts zu zeigen ist. */
  readonly empty: TeamChatEmptyState | null;
  /** Fehler NEBEN vorhandenen (möglicherweise veralteten) Daten. */
  readonly staleWarning: ReactNode | null;
  /** Die Liste der Unterhaltungen. */
  readonly list: ReactNode;
  /** Dialoge; bleiben in jedem Zustand gemountet. */
  readonly overlays: ReactNode;
}

export function TeamChatInbox({
  portal,
  frame,
}: {
  readonly portal: TeamChatPortal;
  /**
   * Hülle des Portals. Ohne Angabe die Kopfzeile des Schul-Portals; das
   * OGS-Portal reicht hier `TenantPage` herein.
   */
  readonly frame?: (parts: TeamChatInboxParts) => ReactNode;
}) {
  const { api, navigate, threadHref } = portal;
  // The school can switch the internal chat off. The sidebar hides the entry
  // then, but a bookmarked URL still lands here — so the page has to say why it
  // is empty instead of showing a compose button that dead-ends in a 403.
  const flagSaysEnabled = portal.flagSaysEnabled !== false;

  const [searchTerm, setSearchTerm] = useState("");
  const [onlyUnread, setOnlyUnread] = useState(false);
  const [composeOpen, setComposeOpen] = useState(false);

  const {
    data: threads,
    error,
    isLoading,
    mutate,
  } = useSWR(
    flagSaysEnabled
      ? [`${portal.cacheScope}:team-chat-inbox`, onlyUnread]
      : null,
    () => api.fetchInbox({ onlyUnread }),
    {
      revalidateOnFocus: false,
      // A different cache scope means a different authenticated school
      // session. Never render its predecessor's conversations while loading.
      keepPreviousData: false,
      // "Ausgeschaltet" ist kein transienter Fehler; SWR soll ihn nicht mit
      // Backoff wiederholen.
      shouldRetryOnError: (err: unknown) => !isStaffMessagingDisabled(err),
      onError: (err: unknown) =>
        logger.error("team_chat_inbox_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        }),
    },
  );

  // A colleague wrote → revalidate the list. Refetch-only (never advances a
  // read cursor), so it fires even in a background tab; the debounce collapses
  // a burst into one refetch.
  const refreshInbox = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshInbox,
    eventName: "team-messages-activity",
    debounceMs: 500,
    marksRead: false,
  });

  const filteredThreads = useMemo(() => {
    const list: StaffInboxThread[] = threads ?? [];
    if (!searchTerm) return list;
    const term = searchTerm.toLowerCase();
    return list.filter((thread) =>
      thread.counterpart_name.toLowerCase().includes(term),
    );
  }, [threads, searchTerm]);

  // The cached tenant metadata is not the last word: it is resolved once and
  // cached, so a school switching the chat off mid-session would leave this
  // page showing a red "loading failed" plus a compose button that dead-ends in
  // the very 403 that produced the error. The backend's stable code is the
  // authority — if it says the feature is off, render the off-state whatever
  // the cached flag believes.
  const disabledByBackend = isStaffMessagingDisabled(error);
  const chatEnabled = flagSaysEnabled && !disabledByBackend;
  // Only a REAL failure belongs in a red alert. "Switched off" is not one.
  const loadFailed = Boolean(error) && !disabledByBackend;

  useEffect(() => {
    if (!chatEnabled && composeOpen) {
      setComposeOpen(false);
    }
  }, [chatEnabled, composeOpen]);

  // Ein Ladefehler beendet das Skelett. Ohne das `!loadFailed` hält jede
  // laufende SWR-Wiederholung isLoading wahr (isLoading = !data &&
  // isValidating) und die Seite zeigt ewig Platzhalter, statt zu sagen, was
  // los ist - der Fehlerzustand darunter wäre unerreichbar. Gleiche Regel wie
  // auf der Thread-Seite.
  const showSkeleton = isLoading && !threads && !loadFailed;
  // Arrays sind truthy: ein zwischengespeichertes LEERES Ergebnis aus einem
  // früheren erfolgreichen Abruf lässt `threads` wahr werden, obwohl der
  // aktuelle Abruf gescheitert ist. Ohne diese Zusammenfassung präsentiert die
  // Seite den Fehlschlag als belastbares "keine Nachrichten".
  const nothingToShow = !threads || threads.length === 0;

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Liste.
  const threadList = threads ?? [];
  const unreadThreads = threadList.filter(
    (thread) => thread.unread_count > 0,
  ).length;
  const stats = chatEnabled
    ? `${threadList.length} ${threadList.length === 1 ? "Unterhaltung" : "Unterhaltungen"} · ${unreadThreads} ungelesen`
    : "Team-Chat ist nicht eingeschaltet";

  const composeButton = chatEnabled ? (
    <Button
      type="button"
      variant="primary"
      size="md"
      onClick={() => setComposeOpen(true)}
    >
      Neue Nachricht
    </Button>
  ) : null;

  const emptyIcon = <MessagesSquare size={48} className="text-gray-400" />;
  const empty: TeamChatEmptyState | null = (() => {
    if (!chatEnabled) {
      return {
        icon: emptyIcon,
        title: DISABLED_TITLE,
        description: DISABLED_DESCRIPTION,
      };
    }
    if (showSkeleton) return null;
    if (loadFailed && nothingToShow) {
      // Fehler OHNE Daten: nur den Fehler zeigen. Eine leere Liste
      // danebenzustellen behauptet "Sie haben keine Nachrichten",
      // obwohl in Wahrheit niemand nachsehen konnte - und das ist die
      // auffälligere der beiden Aussagen.
      return {
        icon: emptyIcon,
        title: "Das hat leider nicht geklappt",
        description:
          "Die Unterhaltungen konnten nicht geladen werden. Bitte versuchen Sie es noch einmal.",
        action: (
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void mutate()}
          >
            Erneut versuchen
          </Button>
        ),
      };
    }
    if (filteredThreads.length === 0 && !loadFailed) {
      return {
        icon: emptyIcon,
        title: "Noch keine Nachrichten",
        description: portal.emptyDescription,
        action: composeButton,
      };
    }
    return null;
  })();

  const staleWarning =
    loadFailed && !nothingToShow ? (
      // Fehler NEBEN vorhandenen (möglicherweise veralteten) Daten: die
      // Liste bleibt stehen, der Hinweis sagt, dass sie nicht aktuell
      // sein muss.
      <Alert
        type="error"
        message="Die Unterhaltungen konnten nicht geladen werden."
      />
    ) : null;

  const list = (
    <ul className="space-y-3">
      {filteredThreads.map((thread) => {
        const roleLabel = staffRoleKindLabel(
          thread.counterpart_role_kind,
          portal.kind,
        );
        return (
          <li key={thread.thread_id}>
            <TileCard onClick={() => navigate(threadHref(thread.thread_id))}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    <h3 className="truncate text-base font-semibold text-gray-900">
                      {thread.counterpart_name}
                    </h3>
                    {roleLabel && (
                      <span className="text-xs text-gray-500">{roleLabel}</span>
                    )}
                    <UnreadBadge count={thread.unread_count} tone="staff" />
                  </div>
                  {thread.last_message_body && (
                    <p className="mt-1 truncate text-sm text-gray-600">
                      {thread.last_message_mine && (
                        <span className="text-gray-500">Sie: </span>
                      )}
                      {thread.last_message_body}
                    </p>
                  )}
                </div>
                {thread.last_message_at && (
                  <span className="flex-shrink-0 text-xs whitespace-nowrap text-gray-400">
                    {formatChatDateTime(thread.last_message_at)}
                  </span>
                )}
              </div>
            </TileCard>
          </li>
        );
      })}
    </ul>
  );

  const overlays =
    composeOpen && chatEnabled ? (
      <NewTeamMessageModal
        api={api}
        portal={portal.kind}
        hint={portal.recipientHint}
        onClose={() => setComposeOpen(false)}
        onOpened={(threadId) => navigate(threadHref(threadId))}
      />
    ) : null;

  const parts: TeamChatInboxParts = {
    title: portal.title,
    stats,
    statsLoading: showSkeleton,
    chatEnabled,
    count: filteredThreads.length,
    search: {
      value: searchTerm,
      onChange: setSearchTerm,
      placeholder: "Person suchen…",
    },
    onlyUnread,
    setOnlyUnread,
    composeButton,
    loading: showSkeleton,
    empty,
    staleWarning,
    list,
    overlays,
  };

  return frame ? frame(parts) : <DefaultInboxFrame parts={parts} />;
}

/** Die Hülle des Schul-Portals: Kopfzeile mit Suche, Filter, Liste. */
function DefaultInboxFrame({ parts }: { readonly parts: TeamChatInboxParts }) {
  const {
    title,
    chatEnabled,
    search,
    onlyUnread,
    setOnlyUnread,
    composeButton,
    loading,
    empty,
    staleWarning,
    list,
    overlays,
  } = parts;
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch
        title={title}
        badge={
          loading
            ? undefined
            : { icon: <MessagesSquare size={20} />, count: parts.count }
        }
        search={{
          value: search.value,
          onChange: search.onChange,
          placeholder: search.placeholder,
        }}
      />

      {chatEnabled && (
        <div className="mb-4 flex flex-wrap items-center justify-end gap-2">
          <CustomSelect
            ariaLabel="Unterhaltungen filtern"
            value={onlyUnread ? "unread" : "all"}
            onChange={(next) => setOnlyUnread(next === "unread")}
            triggerClassName="moto-content-surface h-10 w-48 hover:border-gray-300"
            options={[
              { value: "all", label: "Alle Unterhaltungen" },
              { value: "unread", label: "Nur ungelesen" },
            ]}
          />
          {composeButton}
        </div>
      )}

      {loading ? (
        <TeamChatSkeleton />
      ) : empty ? (
        <EmptyState
          icon={empty.icon}
          title={empty.title}
          description={empty.description}
          action={empty.action}
        />
      ) : (
        <>
          {staleWarning && <div className="mb-4">{staleWarning}</div>}
          {list}
        </>
      )}

      {overlays}
    </div>
  );
}
