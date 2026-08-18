/**
 * Shared chat primitives for the parent-OGS messaging feature, used by both the
 * staff thread view and the parent (Elternapp) conversation so the two chats
 * stay visually identical. The calm style (compact time, muted meta line,
 * pre-wrapped body) is the parents-portal look; the staff side adopts it.
 *
 * `own` is from the viewer's perspective — staff messages are "own" in the
 * staff view, guardian messages are "own" in the parent view — so the caller
 * decides which side a bubble sits on, not the component.
 */

import { ArrowRight } from "lucide-react";
import { ChecksIcon } from "@phosphor-icons/react/ssr";

import { Button } from "~/components/ui/button";
import { formatChatTime } from "~/lib/date-helpers";

/** One chat message bubble. */
export function ChatBubble({
  body,
  own,
  senderName,
  createdAt,
  readReceiptLabel,
  deliveryStatus,
  deliveryStatusLabel,
  showSenderName = true,
  showOwnSenderName = true,
  tone = "staff",
  locale = "de-DE",
}: Readonly<{
  body: string;
  own: boolean;
  senderName: string;
  createdAt: string;
  /**
   * "parent" ist die kompakte Sprechblasenform der Eltern-App: die OGS links
   * weiss mit Rand, eigene Nachrichten rechts in blasser Blaufuellung.
   */
  tone?: "staff" | "parent";
  locale?: string;
  // Shown after the timestamp on an own message the other side has read, e.g.
  // "Gelesen". Omit to show no read receipt.
  readReceiptLabel?: string;
  /** Parent-chat receipt. A saved message is sent, a staff read cursor makes it read. */
  deliveryStatus?: "sent" | "read";
  deliveryStatusLabel?: string;
  /** Whether to show the sender label above this bubble. */
  showSenderName?: boolean;
  // Whether to print the sender's name on the viewer's OWN bubbles. The parent
  // view sets this false: a thread has a single guardian account, so every own
  // (guardian) bubble is the logged-in parent — the name is pure noise there.
  // The staff view keeps it true: several colleagues share a thread, so the
  // name disambiguates which one replied. Only affects own bubbles; the other
  // side's name always shows.
  showOwnSenderName?: boolean;
}>) {
  const hideName = !showSenderName || (own && !showOwnSenderName);
  const parent = tone === "parent";
  const surface = parent
    ? own
      ? "bg-moto-blue-soft text-gray-900"
      : "border border-gray-200 bg-white text-gray-900"
    : own
      ? "bg-gray-900 text-white"
      : "bg-gray-100 text-gray-900";
  return (
    <div className={`flex flex-col ${own ? "items-end" : "items-start"}`}>
      {parent && !hideName ? (
        <p className="mb-0.5 px-1 text-xs font-medium text-gray-600">
          {senderName}
        </p>
      ) : null}
      <div
        className={`relative max-w-[84%] rounded-xl px-3 py-2 break-words whitespace-pre-wrap sm:max-w-[72%] lg:max-w-[38rem] ${
          parent
            ? "text-[15px] leading-5 sm:text-base sm:leading-6"
            : "text-sm leading-5"
        } ${parent ? (own ? "rounded-br-md" : "rounded-bl-md") : ""} ${surface}`}
      >
        {parent ? (
          <span
            aria-hidden="true"
            className={`absolute bottom-1 h-3 w-3 rotate-45 ${
              own
                ? "bg-moto-blue-soft -right-1"
                : "-left-1 border-b border-l border-gray-200 bg-white"
            }`}
          />
        ) : null}
        <span className="relative">{body}</span>
      </div>
      {parent ? (
        <div className="mt-0.5 flex items-center gap-1 px-1 text-xs text-gray-500">
          <time dateTime={createdAt}>{formatChatTime(createdAt, locale)}</time>
          {own && deliveryStatus && deliveryStatusLabel ? (
            <span
              className={
                deliveryStatus === "read" ? "text-moto-blue" : "text-gray-500"
              }
              role="img"
              aria-label={deliveryStatusLabel}
              title={deliveryStatusLabel}
            >
              <ChecksIcon
                className="h-3.5 w-3.5"
                weight="bold"
                aria-hidden="true"
              />
            </span>
          ) : null}
        </div>
      ) : (
        <p className="mt-1 px-1 text-xs text-gray-400">
          {hideName ? "" : `${senderName} · `}
          {formatChatTime(createdAt, locale)}
          {readReceiptLabel ? ` · ${readReceiptLabel}` : ""}
        </p>
      )}
    </div>
  );
}

/**
 * A centered system-event line (a `kind === "event"` message). An optional
 * `action` renders a call-to-action button below the text — used by the staff
 * thread to deep-link a "request created" pill to the Änderungsanfragen queue
 * (admin-only; the parent view never passes it).
 */
export function ChatEventCard({
  body,
  createdAt,
  action,
  locale = "de-DE",
}: Readonly<{
  body: string;
  createdAt: string;
  locale?: string;
  action?: { label: string; onClick: () => void };
}>) {
  return (
    <div className="mx-auto flex max-w-[90%] flex-wrap items-center gap-x-2 gap-y-1.5 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">
      <span>{body}</span>
      <span className="text-xs text-gray-400">
        {formatChatTime(createdAt, locale)}
      </span>
      {action && (
        <Button
          type="button"
          variant="primary"
          size="compact"
          onClick={action.onClick}
          className="ml-auto gap-1"
        >
          {action.label}
          <ArrowRight className="h-3.5 w-3.5" aria-hidden="true" />
        </Button>
      )}
    </div>
  );
}
