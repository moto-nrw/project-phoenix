"use client";

import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Input } from "~/components/ui/input";
import { getApiErrorMessage } from "~/components/ui/modal-utils";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import {
  type Guardian,
  type ThreadDetail,
  fetchGuardians,
  relationshipLabel,
  startThread,
} from "~/lib/parent-messages-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "NewMessageModal" });

interface PickedStudent {
  id: string;
  name: string;
  school_class?: string;
  group_name?: string;
}

/**
 * Lets staff start a parent-OGS thread: (1) search for a child, (2) pick the
 * recipient guardian, (3) write a subject + first message, send. The backend
 * creates the thread and returns the full ThreadDetail; onSent receives it so
 * the caller can navigate straight to the new chat window.
 *
 * When `presetStudentId`/`presetStudentName` are provided (e.g. opened from a
 * child's detail page) step 1 is skipped.
 */
export function NewMessageModal({
  onClose,
  onSent,
  presetStudentId,
  presetStudentName,
}: {
  readonly onClose: () => void;
  readonly onSent: (thread: ThreadDetail) => void;
  readonly presetStudentId?: string;
  readonly presetStudentName?: string;
}) {
  const hasPreset = Boolean(presetStudentId);

  const [query, setQuery] = useState("");
  const [results, setResults] = useState<Student[]>([]);
  const [isSearching, setIsSearching] = useState(false);

  const [student, setStudent] = useState<PickedStudent | null>(
    presetStudentId
      ? { id: presetStudentId, name: presetStudentName ?? "" }
      : null,
  );

  const [guardians, setGuardians] = useState<Guardian[]>([]);
  const [guardiansLoading, setGuardiansLoading] = useState(false);
  const [guardian, setGuardian] = useState<Guardian | null>(null);

  const [subject, setSubject] = useState("");
  const [draft, setDraft] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Debounced child search (only while no child is picked).
  useEffect(() => {
    if (student) return;
    const term = query.trim();
    if (term.length < 2) {
      setResults([]);
      return;
    }
    let active = true;
    setIsSearching(true);
    const handle = setTimeout(() => {
      fetchStudents({ search: term, page_size: 20 })
        .then((res) => {
          if (active) setResults(res.students);
        })
        .catch((err) => {
          logger.warn("new_message_student_search_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          if (active) setResults([]);
        })
        .finally(() => {
          if (active) setIsSearching(false);
        });
    }, 250);
    return () => {
      active = false;
      clearTimeout(handle);
    };
  }, [query, student]);

  // Load the guardians of the picked child for the recipient step.
  useEffect(() => {
    if (!student) {
      setGuardians([]);
      return;
    }
    let active = true;
    setGuardiansLoading(true);
    fetchGuardians(student.id)
      .then((list) => {
        if (active) setGuardians(list);
      })
      .catch((err) => {
        logger.warn("new_message_guardians_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: student.id,
        });
        if (active) setGuardians([]);
      })
      .finally(() => {
        if (active) setGuardiansLoading(false);
      });
    return () => {
      active = false;
    };
  }, [student]);

  const resetToStudentStep = () => {
    setStudent(null);
    setGuardian(null);
    setSubject("");
    setDraft("");
    setError(null);
  };

  const handleSend = async () => {
    const trimmedSubject = subject.trim();
    const body = draft.trim();
    if (!student || !guardian || !trimmedSubject || !body || isSending) return;
    setIsSending(true);
    setError(null);
    try {
      const thread = await startThread({
        studentId: student.id,
        guardianAccountId: guardian.account_id,
        subject: trimmedSubject,
        body,
      });
      onSent(thread);
      onClose();
    } catch (err) {
      logger.error("new_message_send_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: student.id,
      });
      setError(
        getApiErrorMessage(
          err,
          "senden",
          "Nachricht",
          "Nachricht konnte nicht gesendet werden.",
        ),
      );
    } finally {
      setIsSending(false);
    }
  };

  const onComposeStep = Boolean(student && guardian);

  const footer = onComposeStep ? (
    <div className="flex justify-end gap-2">
      <Button type="button" variant="outline" size="md" onClick={onClose}>
        Abbrechen
      </Button>
      <Button
        type="button"
        variant="primary"
        size="md"
        onClick={() => void handleSend()}
        isLoading={isSending}
        loadingText="Senden..."
        disabled={
          isSending || subject.trim().length === 0 || draft.trim().length === 0
        }
      >
        Senden
      </Button>
    </div>
  ) : undefined;

  return (
    <Modal isOpen onClose={onClose} title="Neue Nachricht" footer={footer}>
      {/* STEP 1 — pick a child (skipped when a preset child is given) */}
      {!student && (
        <div className="space-y-3">
          <Input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Kind suchen..."
            autoFocus
          />
          <div className="max-h-72 space-y-1 overflow-y-auto">
            {isSearching && (
              <p className="py-3 text-center text-sm text-gray-500">Suche…</p>
            )}
            {!isSearching &&
              query.trim().length >= 2 &&
              results.length === 0 && (
                <p className="py-3 text-center text-sm text-gray-500">
                  Kein Kind gefunden.
                </p>
              )}
            {results.map((s) => (
              <button
                key={s.id}
                type="button"
                onClick={() => {
                  setStudent({
                    id: s.id,
                    name: s.name,
                    school_class: s.school_class,
                    group_name: s.group_name,
                  });
                  setResults([]);
                  setQuery("");
                }}
                className="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-left hover:bg-gray-50"
              >
                <span className="truncate text-sm font-medium text-gray-900">
                  {s.name}
                </span>
                <span className="ml-2 flex-shrink-0 text-xs text-gray-500">
                  {[s.school_class, s.group_name].filter(Boolean).join(" • ")}
                </span>
              </button>
            ))}
            {query.trim().length < 2 && (
              <p className="py-3 text-center text-sm text-gray-400">
                Mindestens 2 Zeichen eingeben.
              </p>
            )}
          </div>
        </div>
      )}

      {/* STEP 2 — pick the recipient guardian */}
      {student && !guardian && (
        <div className="space-y-3">
          {!hasPreset && (
            <button
              type="button"
              onClick={resetToStudentStep}
              className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900"
            >
              <ArrowLeft className="h-4 w-4" /> Anderes Kind wählen
            </button>
          )}
          <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
            <p className="text-sm font-semibold text-gray-900">
              {student.name}
            </p>
            {[student.school_class, student.group_name].filter(Boolean).length >
              0 && (
              <p className="text-xs text-gray-500">
                {[student.school_class, student.group_name]
                  .filter(Boolean)
                  .join(" • ")}
              </p>
            )}
          </div>
          <p className="text-sm font-medium text-gray-700">
            Bezugsperson wählen
          </p>
          {guardiansLoading ? (
            <p className="py-3 text-center text-sm text-gray-500">Lädt…</p>
          ) : guardians.length === 0 ? (
            <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-3 text-sm text-gray-600">
              Dieses Kind hat keine Bezugsperson mit Eltern-Zugang.
            </p>
          ) : (
            <div className="space-y-1">
              {guardians.map((g) => (
                <button
                  key={g.account_id}
                  type="button"
                  onClick={() => setGuardian(g)}
                  className="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-left hover:bg-gray-50"
                >
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium text-gray-900">
                      {g.name}
                    </span>
                    <span className="block text-xs text-gray-500">
                      {relationshipLabel(g.relationship_type)}
                    </span>
                  </span>
                  {g.is_primary && (
                    <span className="ml-2 flex-shrink-0 rounded-full bg-[#83CD2D]/10 px-2 py-0.5 text-xs font-medium text-[#669f21]">
                      primär
                    </span>
                  )}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {/* STEP 3 — subject + first message */}
      {onComposeStep && student && guardian && (
        <div className="space-y-3">
          <button
            type="button"
            onClick={() => {
              setGuardian(null);
              setError(null);
            }}
            className="flex items-center gap-1 text-sm text-gray-500 hover:text-gray-900"
          >
            <ArrowLeft className="h-4 w-4" /> Andere Bezugsperson wählen
          </button>
          <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
            <p className="text-sm font-semibold text-gray-900">
              {guardian.name}
            </p>
            <p className="text-xs text-gray-500">
              {relationshipLabel(guardian.relationship_type)} von {student.name}
            </p>
          </div>
          {error && <Alert type="error" message={error} />}
          <Input
            value={subject}
            onChange={(e) => setSubject(e.target.value)}
            placeholder="Betreff"
            autoFocus
          />
          <textarea
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            placeholder="Nachricht an die Eltern schreiben..."
            rows={4}
            disabled={isSending}
            className="moto-content-surface w-full resize-none rounded-lg border border-gray-300 px-4 py-3 text-sm text-gray-900 focus:border-[#83CD2D] focus:ring-1 focus:ring-[#83CD2D] focus:outline-none disabled:opacity-60"
          />
        </div>
      )}
    </Modal>
  );
}
