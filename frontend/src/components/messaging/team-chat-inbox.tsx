"use client";

import { messageKeys } from "~/lib/swr/portal-keys";

import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useSWRAuth } from "~/lib/swr";
import { MessagesSquare, UsersRound } from "lucide-react";
import { Button } from "~/components/ui/button";
import { formErrorMessage } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { TileCard } from "~/components/ui/tile-card";
import { renderInboxFrame } from "~/components/messaging/team-chat-frames";
import type {
  TeamChatEmptyState,
  TeamChatInboxParts,
} from "~/components/messaging/team-chat-frame-parts";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { NewTeamMessageModal } from "~/components/messaging/new-team-message-modal";
import { NewTeamGroupModal } from "~/components/messaging/team-group-modals";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  type StaffInboxThread,
  isGroupThread,
  isStaffMessagingDisabled,
  staffRoleKindLabel,
} from "~/lib/staff-messages-api";
import type { TeamChatPortal } from "~/lib/team-chat-portal";
import { createLogger } from "~/lib/logger";
import { formatChatDateTime } from "~/lib/date-helpers";
import { foldForSearch } from "~/lib/search-fold";

const logger = createLogger({ component: "TeamChatInbox" });

const DISABLED_TITLE = "Der Team-Chat ist ausgeschaltet";
const DISABLED_DESCRIPTION =
  "Ihre Schule hat den Team-Chat ausgeschaltet. Wenden Sie sich an die OGS-Leitung, wenn Sie ihn nutzen möchten.";

export type { TeamChatInboxParts } from "~/components/messaging/team-chat-frame-parts";

export function TeamChatInbox({
  portal,
  frame = renderInboxFrame,
}: {
  readonly portal: TeamChatPortal;
  /** Das Gerüst um die Teile; nur Tests reichen ein eigenes herein. */
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
  // Gruppen legt nur das OGS-Portal an (#4776); Lehrkräfte sind Mitglied.
  const canStartGroups = portal.kind === "tenant";
  const [groupOpen, setGroupOpen] = useState(false);

  const {
    data: threads,
    error,
    isLoading,
    mutate,
  } = useSWRAuth(
    flagSaysEnabled ? messageKeys.teamChatInbox(onlyUnread) : null,
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
    const term = foldForSearch(searchTerm);
    return list.filter((thread) =>
      foldForSearch(threadTitle(thread)).includes(term),
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
  // Katalogtext, Wiederholen und Vorgangskennung über den gemeinsamen
  // Anzeigeweg (#2517).
  const loadError = useSwrLoadError(
    loadFailed ? error : undefined,
    "die Liste der Unterhaltungen",
    mutate,
  );
  // Der Katalogtext kommt asynchron; bis dahin bleibt das Skelett stehen.
  const errorTextPending = loadFailed && formErrorMessage(loadError) === null;

  useEffect(() => {
    if (!chatEnabled && composeOpen) {
      setComposeOpen(false);
    }
    if (!chatEnabled && groupOpen) {
      setGroupOpen(false);
    }
  }, [chatEnabled, composeOpen, groupOpen]);

  // Ein Ladefehler beendet das Skelett. Ohne das `!loadFailed` hält jede
  // laufende SWR-Wiederholung isLoading wahr (isLoading = !data &&
  // isValidating) und die Seite zeigt ewig Platzhalter, statt zu sagen, was
  // los ist - der Fehlerzustand darunter wäre unerreichbar. Gleiche Regel wie
  // auf der Thread-Seite.
  const showSkeleton =
    (isLoading && !threads && !loadFailed) || errorTextPending;
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
  // Ohne geladene Liste steht keine "0 Unterhaltungen" neben dem Ladefehler
  // (#2517).
  const stats = !chatEnabled
    ? "Ausgeschaltet"
    : threads === undefined
      ? null
      : `${threadList.length} ${threadList.length === 1 ? "Unterhaltung" : "Unterhaltungen"} · ${unreadThreads} ungelesen`;

  const composeButton = chatEnabled ? (
    <>
      {canStartGroups && (
        <Button
          type="button"
          variant="outline"
          size="md"
          onClick={() => setGroupOpen(true)}
        >
          Neuer Gruppenchat
        </Button>
      )}
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => setComposeOpen(true)}
      >
        Neue Nachricht
      </Button>
    </>
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
    // Fehler OHNE Daten: nur den Fehler zeigen (`error`). Eine leere Liste
    // danebenzustellen behauptet "Sie haben keine Nachrichten", obwohl in
    // Wahrheit niemand nachsehen konnte.
    if (loadFailed && nothingToShow) return null;
    if (filteredThreads.length === 0 && !loadFailed) {
      // Kein eigener Knopf: „Neue Nachricht“ steht schon im Kopf (#5034).
      // Bei Suche oder Filter ist die Liste nicht leer, sondern gefiltert.
      return searchTerm || onlyUnread
        ? {
            icon: emptyIcon,
            title: "Keine passenden Unterhaltungen",
            description: "Ändern Sie die Suche oder den Filter.",
          }
        : {
            icon: emptyIcon,
            title: "Noch keine Nachrichten",
            description: portal.emptyDescription,
          };
    }
    return null;
  })();

  const failedWithoutData =
    chatEnabled && !showSkeleton && loadFailed && nothingToShow;
  const staleWarning =
    loadFailed && !nothingToShow ? (
      // Fehler NEBEN vorhandenen (möglicherweise veralteten) Daten: die
      // Liste bleibt stehen, der Hinweis sagt, dass sie nicht aktuell
      // sein muss.
      <LoadErrorAlert error={loadError} />
    ) : null;

  const list = (
    <ul className="space-y-3">
      {filteredThreads.map((thread) => {
        const group = isGroupThread(thread);
        const roleLabel = group
          ? null
          : staffRoleKindLabel(thread.counterpart_role_kind, portal.kind);
        return (
          <li key={thread.thread_id}>
            <TileCard onClick={() => navigate(threadHref(thread.thread_id))}>
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0 flex-1">
                  <div className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
                    {group && (
                      <UsersRound
                        size={16}
                        aria-hidden
                        className="flex-shrink-0 self-center text-gray-500"
                      />
                    )}
                    <h3 className="truncate text-base font-semibold text-gray-900">
                      {threadTitle(thread)}
                    </h3>
                    {group && (
                      <span className="text-xs text-gray-500">
                        Gruppenchat ·{" "}
                        {memberCountLabel(thread.member_count ?? 0)}
                      </span>
                    )}
                    {roleLabel && (
                      <span className="text-xs text-gray-500">{roleLabel}</span>
                    )}
                    <UnreadBadge count={thread.unread_count} tone="staff" />
                  </div>
                  {thread.last_message_body ? (
                    <p className="mt-1 truncate text-sm text-gray-600">
                      <span className="text-gray-500">
                        {previewSender(thread)}
                      </span>
                      {thread.last_message_body}
                    </p>
                  ) : (
                    group && (
                      <p className="mt-1 truncate text-sm text-gray-500">
                        Noch keine Nachricht
                      </p>
                    )
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

  const overlays = (
    <>
      {composeOpen && chatEnabled && (
        <NewTeamMessageModal
          api={api}
          portal={portal.kind}
          hint={portal.recipientHint}
          onClose={() => setComposeOpen(false)}
          onOpened={(threadId) => navigate(threadHref(threadId))}
        />
      )}
      {groupOpen && chatEnabled && canStartGroups && (
        <NewTeamGroupModal
          api={api}
          onClose={() => setGroupOpen(false)}
          onCreated={(threadId) => navigate(threadHref(threadId))}
        />
      )}
    </>
  );

  const parts: TeamChatInboxParts = {
    title: portal.title,
    stats,
    statsLoading: showSkeleton,
    chatEnabled,
    count: threads === undefined ? null : filteredThreads.length,
    search: {
      value: searchTerm,
      onChange: setSearchTerm,
      placeholder: "Person oder Gruppenchat suchen…",
    },
    onlyUnread,
    setOnlyUnread,
    composeButton,
    loading: showSkeleton,
    empty,
    error: failedWithoutData ? loadError : null,
    staleWarning,
    list,
    overlays,
  };

  return frame(parts);
}

/** Name in der Liste: das Gegenüber, bei einer Gruppe ihr Name (#4776). */
function threadTitle(thread: StaffInboxThread): string {
  return isGroupThread(thread)
    ? (thread.group_name ?? "Gruppenchat")
    : thread.counterpart_name;
}

function memberCountLabel(count: number): string {
  return `${count} ${count === 1 ? "Mitglied" : "Mitglieder"}`;
}

/** „Sie: “ vor der eigenen Nachricht, in einer Gruppe der Vorname sonst. */
function previewSender(thread: StaffInboxThread): string {
  if (thread.last_message_mine) return "Sie: ";
  if (isGroupThread(thread) && thread.last_sender_name) {
    return `${thread.last_sender_name.split(" ")[0]}: `;
  }
  return "";
}
