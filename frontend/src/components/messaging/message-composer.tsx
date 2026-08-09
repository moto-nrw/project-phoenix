"use client";

import { useCallback, useEffect, useRef } from "react";
import { Button } from "~/components/ui/button";

// Hard client cap mirroring the backend's 2000-rune message/note limit
// (maxMessageLen in services/messaging, maxParentNoteLen on the parent side).
// Without it, a >2000 paste sends and bounces back as a generic
// "konnte nicht gesendet werden" 400; maxLength stops the over-long input at the
// source and the counter near the limit tells the user why. Note: maxLength
// counts UTF-16 code units while the backend counts runes, so an astral-plane
// glyph (rare in German school chat) is a slight over-count — acceptable as a
// client guard; the backend stays the authority.
const MAX_MESSAGE_LEN = 2000;
// Reveal the counter only in the last stretch so the calm chat composer is not
// cluttered with "0/2000" on every empty field.
const COUNTER_VISIBLE_FROM = MAX_MESSAGE_LEN - 200;

/**
 * The shared chat composer for the parent-OGS messaging feature: an
 * auto-growing textarea plus an inline send button, Enter-to-send (Shift+Enter
 * for a newline). Used identically by the parent chat (OgsConversation) and the
 * staff thread page so both stay visually in sync with the calm portal look —
 * neutral gray focus, kit Button, no brand-green accent.
 */
export function MessageComposer({
  value,
  onChange,
  onSend,
  sending,
  placeholder = "Nachricht schreiben...",
  disabled: externallyDisabled = false,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onSend: () => void;
  readonly sending: boolean;
  readonly placeholder?: string;
  readonly disabled?: boolean;
}) {
  const ref = useRef<HTMLTextAreaElement | null>(null);

  // Grow with the content up to a max, then scroll inside the field.
  const autoSize = useCallback(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
    el.style.overflowY = el.scrollHeight > 160 ? "auto" : "hidden";
  }, []);
  useEffect(autoSize, [value, autoSize]);

  const disabled = externallyDisabled || sending || value.trim().length === 0;

  return (
    <div className="flex flex-col gap-2 sm:flex-row sm:items-end">
      <div className="flex w-full min-w-0 flex-col gap-1">
        <label htmlFor="message-composer" className="sr-only">
          Nachricht
        </label>
        <textarea
          id="message-composer"
          name="message-composer"
          ref={ref}
          rows={1}
          value={value}
          maxLength={MAX_MESSAGE_LEN}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            // Ignore the Enter that confirms an IME composition candidate
            // (accented letters, mobile autocomplete): nativeEvent.isComposing is
            // true mid-composition, and sending there would fire off a
            // half-composed message instead of accepting the candidate.
            if (
              event.key === "Enter" &&
              !event.shiftKey &&
              !event.nativeEvent.isComposing
            ) {
              event.preventDefault();
              if (!disabled) onSend();
            }
          }}
          disabled={externallyDisabled || sending}
          placeholder={placeholder}
          className="moto-content-surface max-h-40 min-h-[46px] w-full resize-none overflow-hidden rounded-lg border border-gray-300 px-4 py-3 text-sm text-gray-900 transition-shadow focus:border-gray-400 focus:ring-1 focus:ring-gray-300 focus:outline-none disabled:opacity-60"
        />
        {value.length >= COUNTER_VISIBLE_FROM ? (
          <span
            className={`self-end text-xs ${value.length >= MAX_MESSAGE_LEN ? "text-moto-red" : "text-gray-400"}`}
          >
            {value.length}/{MAX_MESSAGE_LEN}
          </span>
        ) : null}
      </div>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={onSend}
        isLoading={sending}
        loadingText="Senden..."
        disabled={disabled}
        className="h-[46px] sm:flex-shrink-0"
      >
        Senden
      </Button>
    </div>
  );
}
