"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createSuggestion, updateSuggestion } from "~/lib/suggestions-api";
import type { Suggestion } from "~/lib/suggestions-helpers";

interface SuggestionFormProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess: () => void;
  readonly editSuggestion?: Suggestion | null;
}

export function SuggestionForm({
  isOpen,
  onClose,
  onSuccess,
  editSuggestion,
}: SuggestionFormProps) {
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { success: toastSuccess, error: toastError } = useToast();

  const isEdit = !!editSuggestion;

  useEffect(() => {
    if (isOpen && editSuggestion) {
      setTitle(editSuggestion.title);
      setDescription(editSuggestion.description);
    } else if (isOpen) {
      setTitle("");
      setDescription("");
    }
    setError("");
  }, [isOpen, editSuggestion]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError("");

    const trimmedTitle = title.trim();
    const trimmedDescription = description.trim();

    if (!trimmedTitle) {
      setError("Titel ist erforderlich.");
      return;
    }
    if (trimmedTitle.length > 200) {
      setError("Titel darf maximal 200 Zeichen lang sein.");
      return;
    }
    if (!trimmedDescription) {
      setError("Beschreibung ist erforderlich.");
      return;
    }
    if (trimmedDescription.length > 5000) {
      setError("Beschreibung darf maximal 5.000 Zeichen lang sein.");
      return;
    }

    setIsSubmitting(true);
    try {
      if (isEdit && editSuggestion) {
        await updateSuggestion(editSuggestion.id, {
          title: trimmedTitle,
          description: trimmedDescription,
        });
        toastSuccess("Beitrag wurde aktualisiert.");
      } else {
        await createSuggestion({
          title: trimmedTitle,
          description: trimmedDescription,
        });
        toastSuccess("Beitrag wurde eingereicht.");
      }
      onSuccess();
      onClose();
    } catch {
      const msg = isEdit
        ? "Fehler beim Aktualisieren des Beitrags."
        : "Fehler beim Einreichen des Beitrags.";
      setError(msg);
      toastError(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={isSubmitting}
        className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="submit"
        form="suggestion-form"
        disabled={isSubmitting}
        className="flex-1 rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {isSubmitting
          ? "Wird gespeichert..."
          : isEdit
            ? "Speichern"
            : "Einreichen"}
      </button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={isEdit ? "Beitrag bearbeiten" : "Neuer Beitrag"}
      footer={footer}
    >
      <form id="suggestion-form" onSubmit={(e) => void handleSubmit(e)}>
        <div className="space-y-4">
          <div>
            <label
              htmlFor="suggestion-title"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Titel <span className="text-red-500">*</span>
            </label>
            <input
              id="suggestion-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              maxLength={200}
              placeholder="z.B. 'PDF-Export für Vertretungsplan'"
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
              autoFocus
            />
          </div>
          <div>
            <label
              htmlFor="suggestion-description"
              className="mb-1 block text-sm font-medium text-gray-700"
            >
              Beschreibung <span className="text-red-500">*</span>
            </label>
            <textarea
              id="suggestion-description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              maxLength={5000}
              rows={5}
              placeholder="Beschreibe dein Feedback..."
              className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:border-gray-900 focus:ring-1 focus:ring-gray-900 focus:outline-none"
            />
            <div className="mt-1 text-right text-xs text-gray-400">
              {description.length} / 5.000
            </div>
          </div>
          {error && <p className="text-sm text-red-600">{error}</p>}
        </div>
      </form>
    </Modal>
  );
}
