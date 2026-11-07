"use client";

import { PaperPlaneRightIcon } from "@phosphor-icons/react/ssr";
import { Button } from "~/components/ui/button";
import { Textarea } from "~/components/ui/textarea";
import { SpinnerIcon } from "~/components/ui/icons";

// Hard client cap mirroring the backend's 2000-rune message/note limit
// (maxMessageLen in modules/communication/internal/parentmessages, maxParentNoteLen on the parent side).
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
  tone = "staff",
  sendLabel = "Senden",
  sendingLabel = "Senden...",
  fieldLabel = "Nachricht",
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onSend: () => void;
  readonly sending: boolean;
  readonly placeholder?: string;
  readonly disabled?: boolean;
  /**
   * "parent" ist die kompakte Variante der Eltern-App. Das Feld bleibt mit
   * 16 px Schrift mobil zoomfest, die Schaltflaeche steht daneben.
   */
  readonly tone?: "staff" | "parent";
  readonly sendLabel?: string;
  readonly sendingLabel?: string;
  readonly fieldLabel?: string;
}) {
  const disabled = externallyDisabled || sending || value.trim().length === 0;

  const parent = tone === "parent";

  return (
    <div
      className={
        parent
          ? "flex flex-row items-end gap-2"
          : "flex flex-col gap-2 sm:flex-row sm:items-end"
      }
    >
      <div className="flex w-full min-w-0 flex-col gap-1">
        <label htmlFor="message-composer" className="sr-only">
          {fieldLabel}
        </label>
        <Textarea
          id="message-composer"
          name="message-composer"
          autoGrow
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
          className={
            parent
              ? "min-h-8 px-3 py-1.5 text-base leading-5 pointer-coarse:min-h-11 pointer-coarse:py-3"
              : "min-h-10 px-4 py-2.5 leading-5 pointer-coarse:min-h-11 pointer-coarse:py-3"
          }
        />
        {value.length >= COUNTER_VISIBLE_FROM ? (
          <span
            className={`self-end ${parent ? "text-[15px]" : "text-xs"} ${value.length >= MAX_MESSAGE_LEN ? "text-moto-red" : "text-gray-400"}`}
          >
            {value.length}/{MAX_MESSAGE_LEN}
          </span>
        ) : null}
      </div>
      <Button
        type="button"
        variant="primary"
        size={parent ? "icon" : "field"}
        onClick={onSend}
        disabled={disabled}
        aria-label={parent ? (sending ? sendingLabel : sendLabel) : undefined}
        aria-busy={sending}
        title={parent ? sendLabel : undefined}
        className="shrink-0"
      >
        {parent ? (
          sending ? (
            <SpinnerIcon className="h-5 w-5" />
          ) : (
            <PaperPlaneRightIcon
              size={19}
              weight="fill"
              className="translate-x-px"
              aria-hidden="true"
            />
          )
        ) : (
          sendLabel
        )}
      </Button>
    </div>
  );
}
