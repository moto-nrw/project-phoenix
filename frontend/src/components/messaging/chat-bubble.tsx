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

import { formatChatTime } from "~/lib/date-helpers";

/** One chat message bubble. */
export function ChatBubble({
  body,
  own,
  senderName,
  createdAt,
  readReceiptLabel,
}: Readonly<{
  body: string;
  own: boolean;
  senderName: string;
  createdAt: string;
  // Shown after the timestamp on an own message the other side has read, e.g.
  // "Gelesen". Omit to show no read receipt.
  readReceiptLabel?: string;
}>) {
  return (
    <div className={`flex flex-col ${own ? "items-end" : "items-start"}`}>
      <div
        className={`max-w-[85%] rounded-2xl px-4 py-2 text-sm leading-5 break-words whitespace-pre-wrap ${
          own ? "bg-gray-900 text-white" : "bg-gray-100 text-gray-900"
        }`}
      >
        {body}
      </div>
      <p className="mt-1 px-1 text-xs text-gray-400">
        {senderName} · {formatChatTime(createdAt)}
        {readReceiptLabel ? ` · ${readReceiptLabel}` : ""}
      </p>
    </div>
  );
}

/** A centered system-event line (a `kind === "event"` message). */
export function ChatEventCard({
  body,
  createdAt,
}: Readonly<{ body: string; createdAt: string }>) {
  return (
    <div className="mx-auto max-w-[90%] rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">
      {body}
      <span className="ml-2 text-xs text-gray-400">
        {formatChatTime(createdAt)}
      </span>
    </div>
  );
}
