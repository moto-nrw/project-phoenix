"use client";

import { useLocale, useTranslations } from "next-intl";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ListRowLink } from "~/components/ui/list-row-link";
import { ChatTimestamp } from "~/components/parent/messages/chat-timestamp";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { ParentSection } from "~/components/parent/shell/parent-section";
import type { GroupThreadSummary } from "~/lib/parent-api";
import { formatChatClockTime, formatChatDateTime } from "~/lib/date-helpers";
import { parentPath } from "~/lib/parent-url";
import { groupsConcept } from "~/lib/concepts/groups";

/**
 * Gruppennachrichten der OGS (#4784): what the OGS wrote to the group, class
 * or AG of the guardian's children. Each row opens the thread; an answer goes
 * only to the OGS. Renders nothing while there is neither a thread nor a
 * load error, so a family without one keeps the page it knows.
 *
 * `compact` caps the list height for the single-child page, where the chat
 * below fills the rest of the screen.
 */
export function GroupThreadsSection({
  threads,
  compact = false,
  error,
}: Readonly<{
  threads: readonly GroupThreadSummary[];
  compact?: boolean;
  error?: FormErrorInput;
}>) {
  const t = useTranslations("parentGroupThreads");
  if (threads.length === 0 && !error) return null;
  return (
    <ParentSection
      title={t("sectionTitle")}
      description={t("sectionDescription")}
      concept={groupsConcept}
    >
      {error && <LoadErrorAlert error={error} />}
      {threads.length > 0 && (
        <ul
          className={`divide-y divide-gray-100 ${compact ? "max-h-44 overflow-y-auto" : ""}`}
          data-testid="parent-group-threads"
        >
          {threads.map((thread) => (
            <li key={thread.id}>
              <GroupThreadRow thread={thread} />
            </li>
          ))}
        </ul>
      )}
    </ParentSection>
  );
}

function GroupThreadRow({ thread }: Readonly<{ thread: GroupThreadSummary }>) {
  const t = useTranslations("parentGroupThreads");
  const tm = useTranslations("parentMessages");
  const locale = useLocale();
  const timestamp = formatChatDateTime(thread.last_message_at, locale);
  const compactTimestamp = thread.last_message_at
    ? formatChatClockTime(thread.last_message_at, locale)
    : "";
  const unread = thread.unread > 0;
  const counterpart = thread.school_name
    ? tm("ogsTeam", { school: thread.school_name })
    : thread.counterpart_name;

  return (
    <ListRowLink
      href={parentPath(`/parents/messages/group/${thread.id}`)}
      className="rounded-lg px-0 py-3"
      overline={`${thread.target_label} · ${counterpart}`}
      metaLabel={timestamp || undefined}
      meta={
        timestamp ? (
          <ChatTimestamp full={timestamp} compact={compactTimestamp} />
        ) : undefined
      }
      title={thread.title}
      badge={
        <>
          {thread.closed && (
            <span className="text-xs font-medium text-gray-500">
              {t("closed")}
            </span>
          )}
          <UnreadBadge count={thread.unread} />
        </>
      }
      unread={unread}
      subtitle={thread.last_message_body}
    />
  );
}
