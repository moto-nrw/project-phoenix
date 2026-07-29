"use client";

import { useState, useEffect, type ReactNode } from "react";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { createSuggestion, updateSuggestion } from "~/lib/suggestions-api";
import type { Suggestion } from "~/lib/suggestions-helpers";
import { createLogger } from "~/lib/logger";
import { trackEvent } from "~/lib/analytics";

const logger = createLogger({ component: "SuggestionForm" });

/** Write half of a board — the parent feedback board supplies its own. */
export interface SuggestionFormApi {
  create: (title: string, description: string) => Promise<unknown>;
  update: (id: string, title: string, description: string) => Promise<unknown>;
}

const staffFormApi: SuggestionFormApi = {
  create: (title, description) => createSuggestion({ title, description }),
  update: (id, title, description) =>
    updateSuggestion(id, { title, description }),
};

interface SuggestionFormProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly onSuccess: () => void;
  readonly editSuggestion?: Suggestion | null;
  /** Which board to write to. Defaults to the school's staff board. */
  readonly api?: SuggestionFormApi;
  /** Optional note above the fields, e.g. who will read this. */
  readonly hint?: ReactNode;
  /** Example wording. The staff default names a staff feature. */
  readonly titlePlaceholder?: string;
  readonly descriptionPlaceholder?: string;
}

export function SuggestionForm({
  isOpen,
  onClose,
  onSuccess,
  editSuggestion,
  api = staffFormApi,
  hint,
  titlePlaceholder = "z.B. 'PDF-Export für Vertretungsplan'",
  descriptionPlaceholder = "Beschreibe dein Feedback...",
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
        await api.update(editSuggestion.id, trimmedTitle, trimmedDescription);
        toastSuccess("Beitrag wurde aktualisiert.");
      } else {
        await api.create(trimmedTitle, trimmedDescription);
        trackEvent("suggestion_created");
        toastSuccess("Beitrag wurde eingereicht.");
      }
      onSuccess();
      onClose();
    } catch (err) {
      logger.error("suggestion_submit_failed", {
        error: err instanceof Error ? err.message : String(err),
        is_edit: isEdit,
      });
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
        {isSubmitting && "Wird gespeichert..."}
        {!isSubmitting && isEdit && "Speichern"}
        {!isSubmitting && !isEdit && "Einreichen"}
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
          {hint}
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
              placeholder={titlePlaceholder}
              className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:border-gray-400 focus-visible:ring-1 focus-visible:ring-gray-400"
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
              placeholder={descriptionPlaceholder}
              className="w-full resize-none rounded-lg border border-gray-300 px-3 py-2 text-sm transition-colors focus:outline-none focus-visible:border-gray-400 focus-visible:ring-1 focus-visible:ring-gray-400"
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
