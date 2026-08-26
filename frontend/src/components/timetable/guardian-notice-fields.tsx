"use client";

import { useEffect, useState } from "react";

import { Checkbox } from "~/components/ui/checkbox";
import { Input } from "~/components/ui/input";
import { Textarea } from "~/components/ui/textarea";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { timetableService } from "~/lib/timetable-api";
import type {
  GuardianNoticeInput,
  GuardianNoticeReach,
} from "~/lib/timetable-types";

const logger = createLogger({ component: "GuardianNoticeFields" });

/** The block a notice would be about; the prefilled text is built from it. */
export interface GuardianNoticeBlock {
  id: string;
  title: string;
  date: string; // YYYY-MM-DD
  startTime: string; // HH:MM
  endTime: string; // HH:MM
}

/** Form state the parent surface owns; `send` mirrors the checkbox. */
export interface GuardianNoticeDraft {
  send: boolean;
  title: string;
  message: string;
}

// Mirrors the announcement service limits (maxTitleLen / maxBodyLen).
const GUARDIAN_NOTICE_TITLE_MAX = 200;
const GUARDIAN_NOTICE_MESSAGE_MAX = 4000;

/** Prefilled text for the families: what falls out, and when. */
export function suggestGuardianNotice(
  block: GuardianNoticeBlock,
): Pick<GuardianNoticeDraft, "title" | "message"> {
  const day = formatDate(block.date);
  return {
    title: `${block.title} am ${day} entfällt`,
    message: `Die Betreuung „${block.title}“ am ${day} von ${block.startTime} bis ${block.endTime} Uhr fällt aus.`,
  };
}

/**
 * Turns a draft into the request payload, or nothing when the family notice is
 * switched off or not allowed for this block.
 */
export function guardianNoticePayload(
  draft: GuardianNoticeDraft | null,
  reach: GuardianNoticeReach | null,
): GuardianNoticeInput | undefined {
  if (!draft?.send || !reach?.enabled) return undefined;
  const title = draft.title.trim();
  const message = draft.message.trim();
  if (!title || !message) return undefined;
  return { title, message };
}

/** True while the draft says "send" but the text is not usable yet. */
export function guardianNoticeIncomplete(
  draft: GuardianNoticeDraft | null,
  reach: GuardianNoticeReach | null,
  previewApplies = false,
): boolean {
  // Before the preview resolves, its school default is unknown. Do not let a
  // cancellation silently turn that default into "off".
  if (previewApplies && draft === null && reach === null) return true;
  if (!draft?.send || !reach?.enabled) return false;
  return draft.title.trim() === "" || draft.message.trim() === "";
}

function familiesLabel(count: number): string {
  if (count === 1) return "Erreicht 1 Familie im Elternportal.";
  return `Erreicht ${count} Familien im Elternportal.`;
}

interface GuardianNoticeFieldsProps {
  readonly block: GuardianNoticeBlock;
  /** Today as YYYY-MM-DD; a block before today never offers the notice. */
  readonly today: string;
  readonly draft: GuardianNoticeDraft | null;
  readonly onDraftChange: (draft: GuardianNoticeDraft) => void;
  readonly onReachChange: (reach: GuardianNoticeReach | null) => void;
  readonly disabled?: boolean;
  /** Compact chrome for the slide-over; the modal uses the default. */
  readonly compact?: boolean;
}

/**
 * "Eltern informieren" on a cancellation (#2601). Loads what the notice would
 * reach, seeds the draft from the school default and the block, and renders
 * the checkbox with the editable text below it. The parent surface owns the
 * draft so it can send it with the cancellation.
 *
 * Renders nothing while the school has the notice switched off or the block
 * lies in the past, so the cancel dialog looks exactly as before in that case.
 */
export function GuardianNoticeFields({
  block,
  today,
  draft,
  onDraftChange,
  onReachChange,
  disabled = false,
  compact = false,
}: GuardianNoticeFieldsProps) {
  const [reach, setReach] = useState<GuardianNoticeReach | null>(null);
  const [failed, setFailed] = useState(false);
  const past = block.date < today;
  const checkboxId = `guardian-notice-send-${block.id}`;

  useEffect(() => {
    if (past) {
      setReach(null);
      onReachChange(null);
      return;
    }
    let cancelled = false;
    setFailed(false);
    // Wrapped in a resolved promise so a synchronous throw (a test double
    // without the method, a broken client) lands in the same catch as a
    // rejected request.
    Promise.resolve()
      .then(() => timetableService.getGuardianNoticeReach(block.id))
      .then((loaded) => {
        if (cancelled) return;
        setReach(loaded);
        onReachChange(loaded);
        if (loaded.enabled) {
          onDraftChange({
            send: loaded.defaultOn,
            ...suggestGuardianNotice(block),
          });
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setFailed(true);
        setReach(null);
        onReachChange(null);
        // A failed preview must not permanently block a cancellation. Seed an
        // explicit opt-out, distinct from the initial loading state above.
        onDraftChange({ send: false, ...suggestGuardianNotice(block) });
        logger.error("guardian_notice_reach_failed", {
          instance_id: block.id,
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
    // The block identity is what matters; the title/date inside it only change
    // together with the id.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [block.id, past]);

  if (past) return null;
  if (failed) {
    return (
      <p className="text-xs text-gray-500">
        Ob die Eltern informiert werden können, ließ sich gerade nicht laden.
        Die Absage selbst ist davon nicht betroffen.
      </p>
    );
  }
  if (!reach?.enabled || !draft) return null;

  const surface = compact
    ? "space-y-2"
    : "space-y-3 rounded-lg border border-gray-200 bg-gray-50 p-3";

  return (
    <div className={surface} data-testid="guardian-notice-fields">
      <label
        htmlFor={checkboxId}
        className="flex cursor-pointer items-start gap-3"
      >
        <Checkbox
          id={checkboxId}
          checked={draft.send}
          disabled={disabled}
          onChange={(e) => onDraftChange({ ...draft, send: e.target.checked })}
          aria-label="Eltern informieren"
        />
        <span className="min-w-0">
          <span className="block text-sm font-medium text-gray-900">
            Eltern informieren
          </span>
          <span className="block text-xs text-gray-500">
            {reach.familyCount > 0
              ? familiesLabel(reach.familyCount)
              : "Kein Kind dieses Termins hat Eltern mit Zugang zum Elternportal."}
          </span>
        </span>
      </label>
      {draft.send && (
        <div className="space-y-2">
          <Input
            id={`guardian-notice-title-${block.id}`}
            label="Betreff"
            value={draft.title}
            maxLength={GUARDIAN_NOTICE_TITLE_MAX}
            disabled={disabled}
            onChange={(e) => onDraftChange({ ...draft, title: e.target.value })}
          />
          <Textarea
            id={`guardian-notice-message-${block.id}`}
            label="Text an die Eltern"
            rows={3}
            value={draft.message}
            maxLength={GUARDIAN_NOTICE_MESSAGE_MAX}
            disabled={disabled}
            onChange={(e) =>
              onDraftChange({ ...draft, message: e.target.value })
            }
          />
          <p className="text-xs text-gray-500">
            Der interne Grund wird nicht mitgeschickt. Die Eltern sehen nur
            diesen Text.
          </p>
        </div>
      )}
    </div>
  );
}
