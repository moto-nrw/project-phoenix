"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { PaperPlaneRightIcon } from "@phosphor-icons/react/ssr";
import { Paperclip } from "lucide-react";
import { Button } from "~/components/ui/button";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { SpinnerIcon } from "~/components/ui/icons";
import { RemovableChip } from "~/components/ui/removable-chip";
import { Textarea } from "~/components/ui/textarea";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import { formatBytes } from "~/lib/files-api";
import {
  checkAttachmentFiles,
  MAX_MESSAGE_ATTACHMENT_MB,
  MAX_MESSAGE_ATTACHMENTS,
  MESSAGE_ATTACHMENT_ACCEPT,
  uploadMessageAttachment,
  type AttachmentRefusal,
  type MessageAttachment,
} from "~/lib/message-attachments";

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

/** Texte der Anhang-Leiste; die Eltern-App übersetzt sie. */
export interface ComposerAttachmentLabels {
  /** Name des Knopfes mit der Büroklammer. */
  readonly attach: string;
  /** Was angehängt werden darf, steht am Knopf, bevor jemand wählt. */
  readonly attachHint: string;
  readonly remove: (filename: string) => string;
  readonly uploading: (filename: string) => string;
  readonly tooMany: string;
  readonly tooLarge: (filename: string) => string;
  readonly wrongType: (filename: string) => string;
  /** Die Datei als Satzgegenstand einer Fehlermeldung: „die Datei“. */
  readonly fileObject: string;
}

const GERMAN_ATTACHMENT_LABELS: ComposerAttachmentLabels = {
  attach: "Foto oder Datei anhängen",
  attachHint: `Bis zu ${MAX_MESSAGE_ATTACHMENTS} Dateien, je höchstens ${MAX_MESSAGE_ATTACHMENT_MB} MB: Fotos, PDF, Word, Excel, PowerPoint`,
  remove: (filename) => `Anhang entfernen: ${filename}`,
  uploading: (filename) => `${filename} wird hochgeladen`,
  tooMany: `Eine Nachricht kann höchstens ${MAX_MESSAGE_ATTACHMENTS} Dateien haben.`,
  tooLarge: (filename) =>
    `„${filename}“ ist zu groß. Erlaubt sind bis zu ${MAX_MESSAGE_ATTACHMENT_MB} MB je Datei.`,
  wrongType: (filename) =>
    `„${filename}“ kann nicht angehängt werden. Erlaubt sind Fotos, PDF, Word, Excel und PowerPoint.`,
  fileObject: "die Datei",
};

/** Eine gewählte Datei auf dem Weg in die Nachricht. */
interface PendingAttachment {
  readonly key: string;
  readonly file: File;
  /** Vorschau eines Bildes aus dem Gerät, solange es nicht gesendet ist. */
  readonly previewUrl?: string;
  /** Gesetzt, sobald der Upload angekommen ist. */
  readonly attachment?: MessageAttachment;
}

function refusalText(
  refusal: AttachmentRefusal,
  labels: ComposerAttachmentLabels,
): string {
  if (refusal.kind === "too_many") return labels.tooMany;
  if (refusal.kind === "too_large") return labels.tooLarge(refusal.filename);
  return labels.wrongType(refusal.filename);
}

let pendingSequence = 0;

/**
 * The shared chat composer for the parent-OGS messaging feature: an
 * auto-growing textarea plus an inline send button, Enter-to-send (Shift+Enter
 * for a newline). Used identically by the parent chat (OgsConversation) and the
 * staff thread page so both stay visually in sync with the calm portal look —
 * neutral gray focus, kit Button, no brand-green accent.
 *
 * With `attachments` it also carries photos and files (#6685): the paperclip
 * or a pasted picture uploads at once, the tray above the field shows what
 * the message will carry, and `onSend` receives the uploaded files.
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
  attachments,
}: {
  readonly value: string;
  readonly onChange: (value: string) => void;
  readonly onSend: (attachments: readonly MessageAttachment[]) => void;
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
  /**
   * Schaltet Anhänge ein: wohin hochgeladen wird, die Texte und ein Hinweis,
   * der erscheint, sobald etwas angehängt ist (wer die Datei sieht).
   */
  readonly attachments?: {
    readonly uploadUrl: string;
    readonly labels?: ComposerAttachmentLabels;
    readonly notice?: string;
  };
}) {
  const labels = attachments?.labels ?? GERMAN_ATTACHMENT_LABELS;
  const [pending, setPending] = useState<readonly PendingAttachment[]>([]);
  const pendingRef = useRef(pending);
  pendingRef.current = pending;
  const fileInput = useRef<HTMLInputElement>(null);
  // Eine abgelehnte Datei meldet der Composer selbst, über dem Feld.
  const attachError = useApiFormError();

  // Die Vorschauen gehören dem Composer: beim Verlassen gibt er sie frei.
  useEffect(
    () => () => {
      for (const item of pendingRef.current) {
        if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
      }
    },
    [],
  );

  const removePending = useCallback((key: string) => {
    setPending((current) => {
      const gone = current.find((item) => item.key === key);
      if (gone?.previewUrl) URL.revokeObjectURL(gone.previewUrl);
      return current.filter((item) => item.key !== key);
    });
  }, []);

  const addFiles = useCallback(
    (files: readonly File[]) => {
      if (!attachments || files.length === 0) return;
      const { accepted, refusals } = checkAttachmentFiles(
        pendingRef.current.length,
        files,
      );
      const refusal = refusals[0];
      if (refusal) attachError.invalid(refusalText(refusal, labels));
      else attachError.clear();
      const added = accepted.map((file): PendingAttachment => {
        pendingSequence += 1;
        return {
          key: `attachment-${pendingSequence}`,
          file,
          previewUrl: file.type.startsWith("image/")
            ? URL.createObjectURL(file)
            : undefined,
        };
      });
      if (added.length === 0) return;
      setPending((current) => [...current, ...added]);
      for (const item of added) {
        uploadMessageAttachment(attachments.uploadUrl, item.file)
          .then((attachment) => {
            setPending((current) =>
              current.map((entry) =>
                entry.key === item.key ? { ...entry, attachment } : entry,
              ),
            );
          })
          .catch((error: unknown) => {
            removePending(item.key);
            void attachError.show(error, { object: labels.fileObject });
          });
      }
    },
    [attachments, attachError, labels, removePending],
  );

  const uploading = pending.some((item) => !item.attachment);
  const ready = pending.flatMap((item) =>
    item.attachment ? [item.attachment] : [],
  );
  const disabled =
    externallyDisabled ||
    sending ||
    uploading ||
    (value.trim().length === 0 && ready.length === 0);

  const send = () => {
    attachError.clear();
    onSend(ready);
    // Die Nachricht trägt die Dateien jetzt; die Vorschauen sind erledigt.
    for (const item of pending) {
      if (item.previewUrl) URL.revokeObjectURL(item.previewUrl);
    }
    setPending([]);
  };

  const parent = tone === "parent";
  const inputLocked = externallyDisabled || sending;
  const atLimit = pending.length >= MAX_MESSAGE_ATTACHMENTS;

  return (
    <div className="flex flex-col gap-2">
      <FormErrorAlert message={attachError.error} />
      {pending.length > 0 ? (
        <div className="flex flex-col gap-1.5">
          <ul className="flex flex-wrap gap-2">
            {pending.map((item) => (
              <li key={item.key} className="max-w-full">
                <RemovableChip
                  label={item.file.name}
                  detail={
                    item.attachment
                      ? formatBytes(item.attachment.size_bytes)
                      : undefined
                  }
                  icon={
                    item.attachment ? (
                      item.previewUrl ? (
                        // eslint-disable-next-line @next/next/no-img-element -- Vorschau aus dem Gerät (blob:), kein Server-Bild
                        <img
                          src={item.previewUrl}
                          alt=""
                          className="-ml-1.5 h-6 w-6 shrink-0 rounded-full object-cover"
                        />
                      ) : (
                        <FileTypeIcon
                          contentType={item.attachment.content_type}
                        />
                      )
                    ) : (
                      <span
                        role="status"
                        aria-label={labels.uploading(item.file.name)}
                        className="flex shrink-0"
                      >
                        <SpinnerIcon className="h-4 w-4" />
                      </span>
                    )
                  }
                  onRemove={() => removePending(item.key)}
                  removeLabel={labels.remove(item.file.name)}
                />
              </li>
            ))}
          </ul>
          {attachments?.notice ? (
            <p className="text-xs text-gray-600">{attachments.notice}</p>
          ) : null}
        </div>
      ) : null}
      {/* Personal am Handy: das Feld über die ganze Breite, Büroklammer und
          Senden darunter. Die Eltern-App bleibt mit ihren Symbolknöpfen
          einzeilig. */}
      <div
        className={
          parent
            ? "flex flex-row items-end gap-2"
            : "flex flex-wrap items-end gap-2 sm:flex-nowrap"
        }
      >
        {attachments ? (
          <>
            <input
              ref={fileInput}
              type="file"
              multiple
              accept={MESSAGE_ATTACHMENT_ACCEPT}
              className="sr-only"
              tabIndex={-1}
              aria-label={labels.attach}
              onChange={(event) => {
                addFiles(Array.from(event.target.files ?? []));
                // Dieselbe Datei lässt sich danach erneut wählen.
                event.target.value = "";
              }}
            />
            <Button
              type="button"
              variant="ghost"
              size={parent ? "icon" : "field"}
              onClick={() => fileInput.current?.click()}
              disabled={inputLocked || atLimit}
              aria-label={labels.attach}
              title={atLimit ? labels.tooMany : labels.attachHint}
              className="shrink-0"
            >
              <Paperclip className="h-5 w-5" aria-hidden="true" />
            </Button>
          </>
        ) : null}
        <div
          className={
            parent
              ? "flex w-full min-w-0 flex-col gap-1"
              : "order-first flex min-w-0 basis-full flex-col gap-1 sm:order-none sm:flex-1 sm:basis-0"
          }
        >
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
            onPaste={(event) => {
              // Ein eingefügtes Bild (Bildschirmfoto) wird zum Anhang.
              const files = Array.from(event.clipboardData.files);
              if (attachments && files.length > 0) {
                event.preventDefault();
                addFiles(files);
              }
            }}
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
                if (!disabled) send();
              }
            }}
            disabled={inputLocked}
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
          onClick={send}
          disabled={disabled}
          aria-label={parent ? (sending ? sendingLabel : sendLabel) : undefined}
          aria-busy={sending}
          title={parent ? sendLabel : undefined}
          className={parent ? "shrink-0" : "ml-auto shrink-0"}
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
    </div>
  );
}
