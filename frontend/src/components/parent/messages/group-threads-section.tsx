"use client";

import { useLocale, useTranslations } from "next-intl";
import { CaretRightIcon } from "@phosphor-icons/react/ssr";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import NavigationLink from "~/components/ui/navigation-link";
import { UnreadBadge } from "~/components/messaging/unread-badge";
import { ParentSection } from "~/components/parent/shell/parent-section";
import type { GroupThreadSummary } from "~/lib/parent-api";
import { formatChatClockTime, formatChatDateTime } from "~/lib/date-helpers";
import { parentPath } from "~/lib/parent-url";

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
      concept="groups"
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
    <NavigationLink
      href={parentPath(`/parents/messages/group/${thread.id}`)}
      className="flex w-full items-center gap-3 rounded-lg py-3 transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none active:bg-gray-100"
    >
      <span className="min-w-0 flex-1 space-y-0.5">
        <span className="flex min-w-0 items-center justify-between gap-2">
          <span className="min-w-0 truncate text-xs font-medium text-gray-500">
            {thread.target_label} · {counterpart}
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
            {thread.title}
          </span>
          <span className="flex shrink-0 items-center gap-2">
            {thread.closed && (
              <span className="text-xs font-medium text-gray-500">
                {t("closed")}
              </span>
            )}
            <UnreadBadge count={thread.unread} />
          </span>
        </span>
        <span
          className={`block min-w-0 truncate text-sm ${unread ? "text-gray-700" : "text-gray-500"}`}
        >
          {thread.last_message_body}
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
