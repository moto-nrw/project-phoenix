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

import { Button } from "~/components/ui/button";
import { formatChatTime } from "~/lib/date-helpers";

/** One chat message bubble. */
export function ChatBubble({
  body,
  own,
  senderName,
  createdAt,
  readReceiptLabel,
  showOwnSenderName = true,
}: Readonly<{
  body: string;
  own: boolean;
  senderName: string;
  createdAt: string;
  // Shown after the timestamp on an own message the other side has read, e.g.
  // "Gelesen". Omit to show no read receipt.
  readReceiptLabel?: string;
  // Whether to print the sender's name on the viewer's OWN bubbles. The parent
  // view sets this false: a thread has a single guardian account, so every own
  // (guardian) bubble is the logged-in parent — the name is pure noise there.
  // The staff view keeps it true: several colleagues share a thread, so the
  // name disambiguates which one replied. Only affects own bubbles; the other
  // side's name always shows.
  showOwnSenderName?: boolean;
}>) {
  const hideName = own && !showOwnSenderName;
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
        {hideName ? "" : `${senderName} · `}
        {formatChatTime(createdAt)}
        {readReceiptLabel ? ` · ${readReceiptLabel}` : ""}
      </p>
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
}: Readonly<{
  body: string;
  createdAt: string;
  action?: { label: string; onClick: () => void };
}>) {
  return (
    <div className="mx-auto flex max-w-[90%] flex-wrap items-center gap-x-2 gap-y-1.5 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 text-sm text-gray-700">
      <span>{body}</span>
      <span className="text-xs text-gray-400">{formatChatTime(createdAt)}</span>
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
