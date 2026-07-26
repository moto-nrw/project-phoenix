"use client";

import { useCallback, useEffect, useState } from "react";
import { Loader2, Pencil, Plus, StickyNote, Trash2, X } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { createLogger } from "~/lib/logger";
import type { ArrivalNote } from "~/lib/student-arrival-api";
import {
  type ArrivalDayData,
  formatShortDate,
  getWeekdayLabel,
} from "~/lib/arrival-schedule-helpers";

const logger = createLogger({ component: "ArrivalDayEdit" });

interface ArrivalDayEditModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly day: ArrivalDayData | null;
  readonly onSaveException: (params: {
    expectedArrival: string | null;
    reason?: string | null;
  }) => Promise<void>;
  readonly onDeleteException: () => Promise<void>;
  readonly onCreateNote: (content: string) => Promise<void>;
  readonly onUpdateNote: (noteId: number, content: string) => Promise<void>;
  readonly onDeleteNote: (noteId: number) => Promise<void>;
}

export function ArrivalDayEditModal({
  isOpen,
  onClose,
  day,
  onSaveException,
  onDeleteException,
  onCreateNote,
  onUpdateNote,
  onDeleteNote,
}: ArrivalDayEditModalProps) {
  const [arrivalTime, setArrivalTime] = useState("");
  const [hasTimeOverride, setHasTimeOverride] = useState(false);
  const [markAbsent, setMarkAbsent] = useState(false);
  const [reason, setReason] = useState("");

  const [newNoteContent, setNewNoteContent] = useState("");
  const [isAddingNote, setIsAddingNote] = useState(false);
  const [editingNoteId, setEditingNoteId] = useState<number | null>(null);
  const [editingNoteContent, setEditingNoteContent] = useState("");

  const [isSavingException, setIsSavingException] = useState(false);
  const [isDeletingException, setIsDeletingException] = useState(false);
  const [isSavingNote, setIsSavingNote] = useState(false);
  const [deletingNoteId, setDeletingNoteId] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen && day) {
      const exception = day.exception;
      if (exception) {
        setHasTimeOverride(true);
        if (exception.expected_arrival) {
          setArrivalTime(exception.expected_arrival.slice(0, 5));
          setMarkAbsent(false);
        } else {
          setArrivalTime("");
          setMarkAbsent(true);
        }
        setReason(exception.reason ?? "");
      } else {
        setHasTimeOverride(false);
        setArrivalTime("");
        setMarkAbsent(false);
        setReason("");
      }
      setNewNoteContent("");
      setIsAddingNote(false);
      setEditingNoteId(null);
      setEditingNoteContent("");
      setError(null);
    }
  }, [isOpen, day]);

  const handleSaveException = useCallback(async () => {
    if (!hasTimeOverride) return;
    const timeRegex = /^([01]?\d|2[0-3]):[0-5]\d$/;
    if (!markAbsent) {
      if (!arrivalTime) {
        setError("Bitte eine Uhrzeit eingeben oder 'kommt nicht' markieren.");
        return;
      }
      if (!timeRegex.test(arrivalTime)) {
        setError("Ungültiges Zeitformat. Bitte HH:MM.");
        return;
      }
    }

    setIsSavingException(true);
    setError(null);
    try {
      await onSaveException({
        expectedArrival: markAbsent ? null : arrivalTime,
        reason: reason.trim() ? reason.trim() : null,
      });
    } catch (err) {
      logger.error("arrival_exception_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(err instanceof Error ? err.message : "Fehler beim Speichern");
    } finally {
      setIsSavingException(false);
    }
  }, [hasTimeOverride, markAbsent, arrivalTime, reason, onSaveException]);

  const handleRemoveException = useCallback(async () => {
    setIsDeletingException(true);
    setError(null);
    try {
      await onDeleteException();
      setHasTimeOverride(false);
      setArrivalTime("");
      setMarkAbsent(false);
      setReason("");
    } catch (err) {
      logger.error("arrival_exception_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error ? err.message : "Fehler beim Löschen der Ausnahme",
      );
    } finally {
      setIsDeletingException(false);
    }
  }, [onDeleteException]);

  const handleAddNote = useCallback(async () => {
    if (!newNoteContent.trim()) return;
    setIsSavingNote(true);
    setError(null);
    try {
      await onCreateNote(newNoteContent.trim());
      setNewNoteContent("");
      setIsAddingNote(false);
    } catch (err) {
      logger.error("arrival_note_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error ? err.message : "Fehler beim Erstellen der Notiz",
      );
    } finally {
      setIsSavingNote(false);
    }
  }, [newNoteContent, onCreateNote]);

  const handleUpdateNote = useCallback(async () => {
    if (editingNoteId === null || !editingNoteContent.trim()) return;
    setIsSavingNote(true);
    setError(null);
    try {
      await onUpdateNote(editingNoteId, editingNoteContent.trim());
      setEditingNoteId(null);
      setEditingNoteContent("");
    } catch (err) {
      logger.error("arrival_note_update_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error
          ? err.message
          : "Fehler beim Aktualisieren der Notiz",
      );
    } finally {
      setIsSavingNote(false);
    }
  }, [editingNoteId, editingNoteContent, onUpdateNote]);

  const handleDeleteNote = useCallback(
    async (noteId: number) => {
      setDeletingNoteId(noteId);
      setError(null);
      try {
        await onDeleteNote(noteId);
      } catch (err) {
        logger.error("arrival_note_delete_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(
          err instanceof Error ? err.message : "Fehler beim Löschen der Notiz",
        );
      } finally {
        setDeletingNoteId(null);
      }
    },
    [onDeleteNote],
  );

  const startEditNote = (note: ArrivalNote) => {
    setEditingNoteId(note.id);
    setEditingNoteContent(note.content);
    setIsAddingNote(false);
  };

  const cancelEditNote = () => {
    setEditingNoteId(null);
    setEditingNoteContent("");
  };

  if (!day) return null;

  const weekdayLabel = getWeekdayLabel(day.weekday);
  const dateLabel = formatShortDate(day.date);
  const baseTime = day.baseSchedule?.expected_arrival
    ? day.baseSchedule.expected_arrival.slice(0, 5)
    : null;

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`${weekdayLabel}, ${dateLabel}`}
      size="sm"
      mobilePosition="center"
    >
      <div className="space-y-5">
        {error ? (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        ) : null}

        {/* Section 1: Ankunftszeit-Ausnahme */}
        <div>
          <h3 className="mb-2 text-sm font-semibold text-gray-900">Ankunft</h3>
          {baseTime ? (
            <p className="mb-2 text-xs text-gray-500">
              Reguläre Zeit: {baseTime} Uhr
            </p>
          ) : (
            <p className="mb-2 text-xs text-gray-500">
              Kein regulärer Ankunftstag hinterlegt.
            </p>
          )}

          {hasTimeOverride ? (
            <div className="space-y-3">
              <label className="inline-flex items-center gap-2 text-sm text-gray-700">
                <input
                  type="checkbox"
                  checked={markAbsent}
                  onChange={(event) => {
                    setMarkAbsent(event.target.checked);
                    if (event.target.checked) setArrivalTime("");
                  }}
                  className="h-4 w-4 rounded border-gray-300 text-[#83CD2D] focus:ring-[#83CD2D]/30"
                />
                Kind kommt nicht
              </label>
              {!markAbsent ? (
                <input
                  aria-label="Abweichende Ankunftszeit"
                  type="time"
                  value={arrivalTime}
                  onChange={(e) => setArrivalTime(e.target.value)}
                  className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none"
                />
              ) : null}
              <input
                type="text"
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="Grund (optional)"
                maxLength={255}
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none"
              />
              <div className="flex items-center justify-between gap-2">
                <div>
                  {day.exception ? (
                    <button
                      type="button"
                      onClick={handleRemoveException}
                      disabled={isDeletingException}
                      className="inline-flex items-center gap-1 rounded-lg px-3 py-2 text-sm font-medium text-red-600 transition-colors hover:bg-red-50 disabled:opacity-50"
                    >
                      {isDeletingException ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <X className="h-3.5 w-3.5" />
                      )}
                      Ausnahme entfernen
                    </button>
                  ) : null}
                </div>
                <div className="flex items-center gap-2">
                  {!day.exception ? (
                    <button
                      type="button"
                      onClick={() => {
                        setHasTimeOverride(false);
                        setArrivalTime("");
                        setMarkAbsent(false);
                        setReason("");
                      }}
                      className="rounded-lg px-3 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100"
                    >
                      Abbrechen
                    </button>
                  ) : null}
                  <button
                    type="button"
                    onClick={handleSaveException}
                    disabled={isSavingException}
                    className="inline-flex items-center gap-1 rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
                  >
                    {isSavingException ? (
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    ) : null}
                    {day.exception ? "Ändern" : "Speichern"}
                  </button>
                </div>
              </div>
            </div>
          ) : (
            <button
              type="button"
              onClick={() => setHasTimeOverride(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-dashed border-gray-300 px-3 py-2 text-sm text-gray-600 transition-colors hover:border-gray-400 hover:bg-gray-50"
            >
              <Pencil className="h-3.5 w-3.5" />
              Abweichende Ankunft eintragen
            </button>
          )}
        </div>

        <div className="border-t border-gray-100" />

        {/* Section 2: Notizen */}
        <div>
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">Notizen</h3>
            {!isAddingNote && editingNoteId === null ? (
              <button
                type="button"
                onClick={() => setIsAddingNote(true)}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100"
              >
                <Plus className="h-3.5 w-3.5" />
                Notiz hinzufügen
              </button>
            ) : null}
          </div>

          <div className="mb-3">
            <Alert
              type="info"
              message="Notizen werden auch auf den NFC-Tablets angezeigt."
            />
          </div>

          {day.notes.length > 0 ? (
            <div className="space-y-2">
              {day.notes.map((note) => (
                <div
                  key={note.id}
                  className="group flex items-start gap-2 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2"
                >
                  {editingNoteId === note.id ? (
                    <div className="flex-1 space-y-2">
                      <textarea
                        aria-label="Ankunftsnotiz bearbeiten"
                        value={editingNoteContent}
                        onChange={(e) => setEditingNoteContent(e.target.value)}
                        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none"
                        rows={2}
                        maxLength={500}
                        autoFocus
                      />
                      <div className="flex items-center justify-between">
                        <span className="text-xs text-gray-400">
                          {editingNoteContent.length}/500
                        </span>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={cancelEditNote}
                            className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
                          >
                            Abbrechen
                          </button>
                          <button
                            type="button"
                            onClick={handleUpdateNote}
                            disabled={
                              isSavingNote || !editingNoteContent.trim()
                            }
                            className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
                          >
                            {isSavingNote ? (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            ) : null}
                            Speichern
                          </button>
                        </div>
                      </div>
                    </div>
                  ) : (
                    <>
                      <StickyNote className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-gray-400" />
                      <p className="min-w-0 flex-1 text-sm text-gray-700">
                        {note.content}
                      </p>
                      <div className="flex flex-shrink-0 items-center gap-1">
                        <button
                          type="button"
                          onClick={() => startEditNote(note)}
                          className="rounded p-1 text-gray-400 hover:bg-gray-200 hover:text-gray-600"
                          title="Bearbeiten"
                        >
                          <Pencil className="h-3.5 w-3.5" />
                        </button>
                        <button
                          type="button"
                          onClick={() => handleDeleteNote(note.id)}
                          disabled={deletingNoteId === note.id}
                          className="rounded p-1 text-gray-400 hover:bg-red-50 hover:text-red-600 disabled:opacity-50"
                          title="Löschen"
                        >
                          {deletingNoteId === note.id ? (
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Trash2 className="h-3.5 w-3.5" />
                          )}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              ))}
            </div>
          ) : (
            !isAddingNote && (
              <p className="text-sm text-gray-400">
                Keine Notizen für diesen Tag.
              </p>
            )
          )}

          {isAddingNote ? (
            <div className="mt-2 space-y-2">
              <textarea
                value={newNoteContent}
                onChange={(e) => setNewNoteContent(e.target.value)}
                placeholder="Notiz eingeben..."
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none"
                rows={2}
                maxLength={500}
                autoFocus
              />
              <div className="flex items-center justify-between">
                <span className="text-xs text-gray-400">
                  {newNoteContent.length}/500
                </span>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={() => {
                      setIsAddingNote(false);
                      setNewNoteContent("");
                    }}
                    className="rounded-lg px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-100"
                  >
                    Abbrechen
                  </button>
                  <button
                    type="button"
                    onClick={handleAddNote}
                    disabled={isSavingNote || !newNoteContent.trim()}
                    className="inline-flex items-center gap-2 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-700 disabled:opacity-50"
                  >
                    {isSavingNote ? (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    ) : null}
                    Hinzufügen
                  </button>
                </div>
              </div>
            </div>
          ) : null}
        </div>
      </div>
    </FormModal>
  );
}
