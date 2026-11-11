"use client";

// Hausaufgaben in der Klassenansicht (#4796): Eingabe, Ändern und Löschen
// jedes Eintrags der Klasse, egal wer ihn angelegt hat. Der Zustand der
// beiden Dialoge liegt hier, die Anzeige baut die Klassenseite aus den
// Kit-Bausteinen.

import { useState } from "react";
import { useFormModalPending } from "~/components/ui/use-form-modal-pending";
import { useApiFormError } from "~/contexts/ToastContext";
import type { ClassDayHomework } from "~/lib/class-day-api";
import type { RefreshAfterWrite } from "~/lib/hooks/use-refresh-after-write";
import { createLogger } from "~/lib/logger";
import type { HomeworkInput } from "~/lib/school-class-day-api";

const logger = createLogger({ component: "ClassDayHomework" });

/** Die Schreibwege der Hausaufgaben; das Schul-Portal reicht sie herein. */
export interface ClassHomeworkApi {
  readonly add: (
    schoolClass: string,
    date: string,
    input: HomeworkInput,
  ) => Promise<void>;
  readonly change: (
    schoolClass: string,
    homeworkId: string,
    input: HomeworkInput,
  ) => Promise<void>;
  readonly remove: (schoolClass: string, homeworkId: string) => Promise<void>;
}

/** Der Entwurf im Dialog; `editing` ist der geänderte Eintrag. */
interface HomeworkDraft {
  readonly editing: ClassDayHomework | null;
  readonly subject: string;
  readonly description: string;
}

export function useClassHomework({
  api,
  schoolClass,
  dateISO,
  onChanged,
}: {
  readonly api?: ClassHomeworkApi;
  readonly schoolClass: string;
  readonly dateISO: string;
  /** Reloads the class; never rejects (#6652). */
  readonly onChanged: RefreshAfterWrite;
}) {
  const [draft, setDraft] = useState<HomeworkDraft | null>(null);
  const submission = useFormModalPending(draft !== null);
  const saving = submission.pending;
  const [deleting, setDeleting] = useState<ClassDayHomework | null>(null);
  const [removing, setRemoving] = useState(false);
  const formErrors = useApiFormError();
  const deleteErrors = useApiFormError();

  const openNew = () => {
    formErrors.clear();
    setDraft({ editing: null, subject: "", description: "" });
  };
  const openEdit = (entry: ClassDayHomework) => {
    formErrors.clear();
    setDraft({
      editing: entry,
      subject: entry.subject ?? "",
      description: entry.description,
    });
  };
  const close = () => {
    formErrors.clear();
    setDraft(null);
  };
  const update = (field: "subject" | "description", value: string) =>
    setDraft((current) => (current ? { ...current, [field]: value } : current));

  const save = async () => {
    if (!api || !draft) return;
    const description = draft.description.trim();
    if (description === "") {
      formErrors.invalid("Bitte schreiben Sie, was zu tun ist.", {
        description: "Bitte ausfüllen.",
      });
      return;
    }
    const input: HomeworkInput = {
      subject: draft.subject.trim() === "" ? null : draft.subject.trim(),
      description,
    };
    formErrors.clear();
    await submission.run(async (op) => {
      try {
        if (draft.editing) {
          await api.change(schoolClass, draft.editing.id, input);
        } else {
          await api.add(schoolClass, dateISO, input);
        }
      } catch (cause) {
        logger.error("class_day_homework_save_failed", {
          error: cause instanceof Error ? cause.message : String(cause),
        });
        if (!op.isCurrent()) return;
        void formErrors.show(cause, {
          object: "die Hausaufgabe",
          retry: () => void save(),
        });
        return;
      }
      await onChanged();
      // Eine späte Antwort schließt keinen inzwischen neuen Entwurf (#6533).
      if (op.isCurrent()) setDraft(null);
    });
  };

  const askDelete = (entry: ClassDayHomework) => {
    deleteErrors.clear();
    setDeleting(entry);
  };
  const closeDelete = () => {
    deleteErrors.clear();
    setDeleting(null);
  };
  const confirmDelete = async () => {
    if (!api || !deleting) return;
    setRemoving(true);
    try {
      await api.remove(schoolClass, deleting.id);
    } catch (cause) {
      logger.error("class_day_homework_delete_failed", {
        error: cause instanceof Error ? cause.message : String(cause),
      });
      void deleteErrors.show(cause, {
        object: "die Hausaufgabe",
        retry: () => void confirmDelete(),
      });
      setRemoving(false);
      return;
    }
    setDeleting((current) => (current === deleting ? null : current));
    setRemoving(false);
    await onChanged();
  };

  return {
    draft,
    saving,
    /** Der Speicher-Vertrag für den Dialog (`FormModal pending`). */
    pending: submission,
    formError: formErrors.error,
    fieldError: formErrors.fieldError,
    openNew,
    openEdit,
    close,
    update,
    save,
    deleting,
    removing,
    deleteError: deleteErrors.error,
    askDelete,
    closeDelete,
    confirmDelete,
  };
}
