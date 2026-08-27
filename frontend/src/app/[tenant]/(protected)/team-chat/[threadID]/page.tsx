"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { useSession } from "next-auth/react";
import useSWR from "swr";
import { MessagesSquare } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { BackButton } from "~/components/ui/back-button";
import { MessageComposer } from "~/components/messaging/message-composer";
import { ChatBubble } from "~/components/messaging/chat-bubble";
import { useChatViewportLock } from "~/lib/hooks/use-chat-viewport-lock";
import { useMessagesActivity } from "~/lib/hooks/use-messages-activity";
import { useTenant, useTenantSlugSafe } from "~/lib/tenant-context";
import {
  type StaffMessage,
  fetchStaffThread,
  isCounterpartUnavailable,
  isStaffMessagingDisabled,
  postStaffMessage,
} from "~/lib/staff-messages-api";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { createLogger } from "~/lib/logger";
import { TeamThreadSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "TeamChatThreadPage" });

function TeamThreadContent() {
  const params = useParams();
  const threadID = params.threadID as string;
  const { data: session } = useSession();
  const { tenant } = useTenant();
  const tenantSlug = useTenantSlugSafe();
  const flagSaysEnabled = tenant?.staffMessagingEnabled === true;

  // Which bubbles are "mine". The backend stamps every message with its sender
  // account, and the session carries the viewer's — so the side a bubble sits
  // on is decided here, not by the API.
  const myAccountId = session?.user?.id ?? "";

  const {
    data: thread,
    error: loadError,
    isLoading,
    isValidating,
    mutate,
  } = useSWR(
    // Same gate as the inbox: with the feature off there is nothing to fetch,
    // and firing anyway leaves the page in a permanent skeleton while SWR
    // retries a 403 (isLoading is !data && isValidating). The backend code is
    // still consulted below for the case the cached flag is stale.
    flagSaysEnabled ? [`${tenantSlug ?? ""}:team-chat-thread`, threadID] : null,
    () => fetchStaffThread(threadID),
    {
      revalidateOnFocus: false,
      // "Die Schule hat den Chat ausgeschaltet" ist kein transienter Fehler:
      // SWR würde ihn sonst mit Backoff endlos wiederholen, und weil
      // isLoading als (!data && isValidating) definiert ist, bliebe die Seite
      // dauerhaft im Skelett stehen statt den Aus-Zustand zu zeigen.
      shouldRetryOnError: (err: unknown) => !isStaffMessagingDisabled(err),
      onError: (err: unknown) =>
        logger.error("team_chat_thread_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          thread_id: threadID,
        }),
    },
  );

  // Die Schule kann den Chat abschalten, WÄHREND diese Seite offen steht. Dann
  // lädt der Verlauf längst, nur das Senden fällt in den 403. Ohne diesen
  // Merker bliebe der Composer aktiv und jeder weitere Versuch liefe in
  // denselben Fehler.
  const [disabledWhileOpen, setDisabledWhileOpen] = useState(false);
  // Das Gegenüber hat die Schule verlassen, während diese Seite offen stand.
  // Der Verlauf bleibt lesbar, geschrieben wird hier nichts mehr.
  const [counterpartGone, setCounterpartGone] = useState(false);

  const messages: StaffMessage[] = thread?.messages ?? [];

  // Reads are gated too: the service calls requireEnabled before loading a
  // thread, so a switched-off school gets a 403 here, not just on send. The
  // cached tenant flag lags that by up to the tenant-metadata cache window, and
  // a deep-linked or bookmarked thread bypasses the inbox entirely — so the
  // backend's stable code decides, exactly as on the inbox page. Without this
  // the user sees "Der Verlauf konnte nicht geladen werden." for a school that
  // simply switched the feature off.
  const disabledByBackend = isStaffMessagingDisabled(loadError);
  const chatEnabled =
    flagSaysEnabled && !disabledByBackend && !disabledWhileOpen;
  // Off is off, whichever side said so: the cached tenant flag (fast, may lag
  // by the metadata cache window) or the backend's stable code (authoritative,
  // covers a stale flag and a deep-linked thread).
  const chatDisabled =
    !flagSaysEnabled || disabledByBackend || disabledWhileOpen;
  const threadLoadFailed = Boolean(loadError) && !disabledByBackend;
  // Getrennt vom Aus-Zustand: der Chat läuft, nur DIESE Unterhaltung ist zu.
  const readOnlyThread = !chatDisabled && counterpartGone;

  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [sendError, setSendError] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement | null>(null);

  // A colleague wrote in THIS conversation → reload it. Unlike the parent
  // messaging fan-out, the internal event keeps its thread_id (it only reaches
  // the participants), so unrelated conversations are skipped rather than
  // waking every open chat.
  //
  // marksRead: opening/refetching advances the read cursor server-side, so a
  // refetch while the tab is hidden would silently mark an unseen message read.
  // refetchOnFocus heals the case where the connection dropped while the tab
  // slept and no event fired at all.
  const refreshThread = useCallback(() => void mutate(), [mutate]);
  useMessagesActivity({
    onMatch: refreshThread,
    eventName: "team-messages-activity",
    threadId: threadID,
    debounceMs: 500,
    marksRead: true,
    refetchOnFocus: true,
    // Diese Seite schiebt den Lesecursor vor. Ein eigener Send aus einem
    // anderen Tab darf sie deshalb nicht wecken, sonst markiert sie als
    // gelesen, was das Gegenüber in der Zwischenzeit geschrieben hat.
    ignoreOwnSource: myAccountId || null,
  });

  // Loading the conversation marks it read server-side, so nudge the sidebar
  // badge. Gate on !isValidating so this fires only once the real read landed.
  useEffect(() => {
    if (thread && !isValidating && typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("team-messages-unread-refresh"));
    }
  }, [thread, isValidating]);

  // Keep the newest message in view.
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [thread]);

  const handleSend = async () => {
    const body = draft.trim();
    if (!body || isSending) return;
    setIsSending(true);
    setSendError(null);
    try {
      const sent = await postStaffMessage(threadID, body);
      // Show the sent message immediately, then revalidate as a freshness
      // backstop.
      await mutate(
        (prev) =>
          prev ? { ...prev, messages: [...prev.messages, sent] } : prev,
        { revalidate: true },
      );
      setDraft("");
    } catch (err) {
      logger.error("team_chat_message_send_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: threadID,
      });
      if (isCounterpartUnavailable(err)) {
        // Auch das ist ein Zustand, kein Fehlschlag: ohne diesen Zweig bliebe
        // der Composer bedienbar und liefe bei jedem Versuch in denselben 409.
        setCounterpartGone(true);
        setSendError(null);
      } else if (isStaffMessagingDisabled(err)) {
        // Kein roter Fehler: das ist kein Fehlschlag, sondern ein Zustand.
        setDisabledWhileOpen(true);
        setSendError(null);
      } else {
        setSendError(
          getApiErrorMessage(
            err,
            "senden",
            "Nachricht",
            "Die Nachricht konnte nicht gesendet werden.",
          ),
        );
      }
    } finally {
      setIsSending(false);
    }
  };

  const containerRef = useChatViewportLock<HTMLDivElement>(
    Boolean(thread) && !isLoading,
  );

  // Ein Fehler beendet das Skelett. Ohne das `!loadError` hält jede laufende
  // SWR-Wiederholung isLoading wahr und die Seite zeigt ewig Platzhalter statt
  // zu sagen, was los ist.
  const showSkeleton = !thread && isLoading && !loadError && !chatDisabled;

  if (!showSkeleton && !thread) {
    return (
      <div className="w-full space-y-6">
        <BackButton referrer="/team-chat" />
        {chatDisabled ? (
          <EmptyState
            icon={<MessagesSquare size={48} className="text-gray-400" />}
            title="Der Team-Chat ist ausgeschaltet"
            description="Ihre Schule hat den Team-Chat nicht eingeschaltet. Wenden Sie sich an Ihre Leitung, wenn Sie ihn nutzen möchten."
          />
        ) : (
          <Alert
            type="error"
            message={
              loadError
                ? "Der Verlauf konnte nicht geladen werden."
                : "Diese Unterhaltung gibt es nicht."
            }
          />
        )}
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className="flex min-h-[20rem] w-full flex-col overflow-hidden"
    >
      <BackButton referrer="/team-chat" />

      <div className="moto-content-surface flex min-h-0 flex-1 flex-col rounded-2xl border p-4 backdrop-blur-sm sm:p-6">
        {showSkeleton ? (
          <TeamThreadSkeleton />
        ) : (
          <>
            {/* Kopf der Unterhaltung in PageIntro-Optik (Kicker, Titel,
                Unterzeile). Eine zweite Kopfkarte darüber ist hier nicht
                möglich: die Chat-Karte ist an das Sichtfenster gekoppelt und
                trägt den Kopf selbst. */}
            <div className="mb-4 min-w-0">
              <p className="text-moto-blue text-xs font-semibold tracking-wide uppercase">
                Team-Chat
              </p>
              <h1 className="mt-1 truncate text-xl leading-tight font-semibold tracking-tight text-gray-900 sm:text-2xl">
                {thread?.counterpart_name}
              </h1>
              {/* Statuszeile unter dem Titel: die Zahl der geladenen
                  Nachrichten dieser Unterhaltung. */}
              <p className="mt-1 text-sm leading-6 text-gray-600">
                {`${messages.length} ${messages.length === 1 ? "Nachricht" : "Nachrichten"} · nur Sie beide sehen sie`}
              </p>
            </div>

            <div
              ref={scrollRef}
              className="min-h-0 flex-1 space-y-3 overflow-y-auto pr-1"
            >
              {threadLoadFailed && messages.length > 0 && (
                <Alert
                  type="error"
                  message="Der Verlauf konnte nicht aktualisiert werden."
                />
              )}

              {messages.length > 0 ? (
                messages.map((message) => (
                  <ChatBubble
                    key={message.id}
                    body={message.body}
                    own={message.sender_account_id === myAccountId}
                    senderName={message.sender_name}
                    createdAt={message.created_at}
                    // One counterpart per conversation, so the viewer's own
                    // name on their own bubbles is pure noise.
                    showOwnSenderName={false}
                  />
                ))
              ) : threadLoadFailed ? (
                <Alert
                  type="error"
                  message="Der Verlauf konnte nicht geladen werden."
                />
              ) : (
                <EmptyState
                  title="Noch keine Nachrichten"
                  description="Schreiben Sie die erste Nachricht in dieser Unterhaltung."
                />
              )}
            </div>
          </>
        )}

        {sendError && (
          <div className="mt-3">
            <Alert type="error" message={sendError} />
          </div>
        )}

        <div className="mt-4">
          {readOnlyThread ? (
            <Alert
              type="info"
              message="Diese Person gehört nicht mehr zu Ihrer Schule. Sie können den Verlauf lesen, aber nicht mehr schreiben."
            />
          ) : chatEnabled ? (
            <MessageComposer
              value={draft}
              onChange={setDraft}
              onSend={() => void handleSend()}
              sending={isSending}
              disabled={showSkeleton}
              placeholder="Nachricht schreiben…"
            />
          ) : (
            <Alert
              type="info"
              message="Der Team-Chat ist ausgeschaltet. Sie können hier nicht schreiben."
            />
          )}
        </div>
      </div>
    </div>
  );
}

export default function TeamThreadPage() {
  return <TeamThreadContent />;
}
