"use client";

import { useState, useEffect, useCallback } from "react";
import { Loader2, Plus, Pencil, Trash2, X, StickyNote } from "lucide-react";
import { FormModal } from "~/components/ui/form-modal";
import type { PickupNote, DayData } from "@/lib/pickup-schedule-helpers";
import {
  formatPickupTime,
  getWeekdayLabel,
  formatShortDate,
} from "@/lib/pickup-schedule-helpers";

interface PickupDayEditModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly day: DayData | null;
  readonly studentId: string;
  readonly onSaveException: (data: {
    pickupTime?: string;
    reason?: string;
  }) => Promise<void>;
  readonly onDeleteException: () => Promise<void>;
  readonly onCreateNote: (content: string) => Promise<void>;
  readonly onUpdateNote: (noteId: string, content: string) => Promise<void>;
  readonly onDeleteNote: (noteId: string) => Promise<void>;
}

export function PickupDayEditModal({
  isOpen,
  onClose,
  day,
  onSaveException,
  onDeleteException,
  onCreateNote,
  onUpdateNote,
  onDeleteNote,
}: PickupDayEditModalProps) {
  // Exception section state
  const [pickupTime, setPickupTime] = useState("");
  const [hasTimeOverride, setHasTimeOverride] = useState(false);

  // Notes section state
  const [newNoteContent, setNewNoteContent] = useState("");
  const [isAddingNote, setIsAddingNote] = useState(false);
  const [editingNoteId, setEditingNoteId] = useState<string | null>(null);
  const [editingNoteContent, setEditingNoteContent] = useState("");

  // Loading states
  const [isSavingException, setIsSavingException] = useState(false);
  const [isDeletingException, setIsDeletingException] = useState(false);
  const [isSavingNote, setIsSavingNote] = useState(false);
  const [deletingNoteId, setDeletingNoteId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Reset form when modal opens / day changes
  useEffect(() => {
    if (isOpen && day) {
      const exception = day.exception;
      if (exception) {
        setHasTimeOverride(true);
        setPickupTime(
          exception.pickupTime ? formatPickupTime(exception.pickupTime) : "",
        );
      } else {
        setHasTimeOverride(false);
        setPickupTime("");
      }
      setNewNoteContent("");
      setIsAddingNote(false);
      setEditingNoteId(null);
      setEditingNoteContent("");
      setError(null);
    }
  }, [isOpen, day]);

  // Time override handlers
  const handleSaveException = useCallback(async () => {
    if (!hasTimeOverride) return;

    // Validate time
    const timeRegex = /^([01]?\d|2[0-3]):[0-5]\d$/;
    if (pickupTime && !timeRegex.test(pickupTime)) {
      setError("Ungültiges Zeitformat. Bitte verwenden Sie HH:MM.");
      return;
    }

    setIsSavingException(true);
    setError(null);
    try {
      await onSaveException({
        pickupTime: pickupTime || undefined,
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : "Fehler beim Speichern");
    } finally {
      setIsSavingException(false);
    }
  }, [hasTimeOverride, pickupTime, onSaveException]);

  const handleRemoveException = useCallback(async () => {
    setIsDeletingException(true);
    setError(null);
    try {
      await onDeleteException();
      setHasTimeOverride(false);
      setPickupTime("");
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Fehler beim Löschen der Ausnahme",
      );
    } finally {
      setIsDeletingException(false);
    }
  }, [onDeleteException]);

  // Note handlers
  const handleAddNote = useCallback(async () => {
    if (!newNoteContent.trim()) return;

    setIsSavingNote(true);
    setError(null);
    try {
      await onCreateNote(newNoteContent.trim());
      setNewNoteContent("");
      setIsAddingNote(false);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Fehler beim Erstellen der Notiz",
      );
    } finally {
      setIsSavingNote(false);
    }
  }, [newNoteContent, onCreateNote]);

  const handleUpdateNote = useCallback(async () => {
    if (!editingNoteId || !editingNoteContent.trim()) return;

    setIsSavingNote(true);
    setError(null);
    try {
      await onUpdateNote(editingNoteId, editingNoteContent.trim());
      setEditingNoteId(null);
      setEditingNoteContent("");
    } catch (err) {
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
    async (noteId: string) => {
      setDeletingNoteId(noteId);
      setError(null);
      try {
        await onDeleteNote(noteId);
      } catch (err) {
        setError(
          err instanceof Error ? err.message : "Fehler beim Löschen der Notiz",
        );
      } finally {
        setDeletingNoteId(null);
      }
    },
    [onDeleteNote],
  );

  const startEditNote = (note: PickupNote) => {
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
  const baseTime = day.baseSchedule?.pickupTime
    ? formatPickupTime(day.baseSchedule.pickupTime)
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
        {error && (
          <div className="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {/* Section 1: Pickup Time Override */}
        <div>
          <h3 className="mb-2 text-sm font-semibold text-gray-900">
            Abholzeit
          </h3>

          {/* Show base schedule time */}
          {baseTime && (
            <p className="mb-2 text-xs text-gray-500">
              Reguläre Zeit: {baseTime} Uhr
            </p>
          )}

          {!hasTimeOverride ? (
            <button
              type="button"
              onClick={() => setHasTimeOverride(true)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-dashed border-gray-300 px-3 py-2 text-sm text-gray-600 transition-colors hover:border-gray-400 hover:bg-gray-50"
            >
              <Pencil className="h-3.5 w-3.5" />
              Abweichende Zeit eintragen
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <input
                type="time"
                value={pickupTime}
                onChange={(e) => setPickupTime(e.target.value)}
                className="min-w-0 flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
                autoFocus
              />
              <button
                type="button"
                onClick={handleSaveException}
                disabled={isSavingException}
                className="inline-flex items-center gap-1 rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
              >
                {isSavingException && (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                )}
                {day.exception ? "Ändern" : "Speichern"}
              </button>
              {day.exception ? (
                <button
                  type="button"
                  onClick={handleRemoveException}
                  disabled={isDeletingException}
                  className="rounded-lg p-2 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 disabled:opacity-50"
                  title="Zeitausnahme entfernen"
                >
                  {isDeletingException ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    <X className="h-4 w-4" />
                  )}
                </button>
              ) : (
                <button
                  type="button"
                  onClick={() => {
                    setHasTimeOverride(false);
                    setPickupTime("");
                  }}
                  className="rounded-lg px-3 py-2 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100"
                  title="Abbrechen"
                >
                  Abbrechen
                </button>
              )}
            </div>
          )}
        </div>

        {/* Divider */}
        <div className="border-t border-gray-100" />

        {/* Section 2: Notes */}
        <div>
          <div className="mb-2 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-gray-900">Notizen</h3>
            {!isAddingNote && !editingNoteId && (
              <button
                type="button"
                onClick={() => setIsAddingNote(true)}
                className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100"
              >
                <Plus className="h-3.5 w-3.5" />
                Notiz hinzufügen
              </button>
            )}
          </div>

          {/* Existing notes */}
          {day.notes.length > 0 ? (
            <div className="space-y-2">
              {day.notes.map((note) => (
                <div
                  key={note.id}
                  className="group flex items-start gap-2 rounded-lg border border-gray-100 bg-gray-50 px-3 py-2"
                >
                  {editingNoteId === note.id ? (
                    /* Edit inline */
                    <div className="flex-1 space-y-2">
                      <textarea
                        value={editingNoteContent}
                        onChange={(e) => setEditingNoteContent(e.target.value)}
                        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
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
                            {isSavingNote && (
                              <Loader2 className="h-4 w-4 animate-spin" />
                            )}
                            Speichern
                          </button>
                        </div>
                      </div>
                    </div>
                  ) : (
                    /* Display */
                    <>
                      <StickyNote className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-gray-400" />
                      <p className="min-w-0 flex-1 text-sm text-gray-700">
                        {note.content}
                      </p>
                      <div className="flex flex-shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
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

          {/* Add note inline */}
          {isAddingNote && (
            <div className="mt-2 space-y-2">
              <textarea
                value={newNoteContent}
                onChange={(e) => setNewNoteContent(e.target.value)}
                placeholder="Notiz eingeben..."
                className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
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
                    {isSavingNote && (
                      <Loader2 className="h-4 w-4 animate-spin" />
                    )}
                    Hinzufügen
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </FormModal>
  );
}
