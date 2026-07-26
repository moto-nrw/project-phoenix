"use client";

import { useEffect, useState } from "react";
import { ArrowLeft } from "lucide-react";
import { Modal } from "~/components/ui/modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { getApiErrorMessage } from "~/lib/api-error-message";
import { fetchStudents } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import {
  type Guardian,
  type ThreadDetail,
  fetchGuardians,
  openThread,
  relationshipLabel,
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
 * Lets staff start a conversation: (1) search for a child, (2) pick the
 * recipient guardian — then the chat opens directly (WhatsApp-style), no
 * compose step in the modal. The backend get-or-creates the conversation
 * (history shown if one already exists) and `onOpened` receives the
 * ThreadDetail so the caller can navigate straight to the chat window.
 *
 * When `presetStudentId`/`presetStudentName` are provided (e.g. opened from a
 * child's detail page) step 1 is skipped.
 */
export function NewMessageModal({
  onClose,
  onOpened,
  presetStudentId,
  presetStudentName,
}: {
  readonly onClose: () => void;
  readonly onOpened: (thread: ThreadDetail) => void;
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
  // A failed guardian lookup (403/500/network) is an operational error, not the
  // domain fact "this child has no portal guardian" — keep it separate so the UI
  // can offer a retry instead of misreporting the failure as an empty list.
  const [guardiansError, setGuardiansError] = useState<string | null>(null);
  const [guardiansReloadKey, setGuardiansReloadKey] = useState(0);
  const [openingId, setOpeningId] = useState<string | null>(null);
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
      setGuardiansError(null);
      return;
    }
    let active = true;
    setGuardiansLoading(true);
    setGuardiansError(null);
    fetchGuardians(student.id)
      .then((list) => {
        if (active) {
          setGuardians(list);
          setGuardiansError(null);
        }
      })
      .catch((err) => {
        logger.warn("new_message_guardians_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: student.id,
        });
        if (active) {
          setGuardians([]);
          setGuardiansError(
            getApiErrorMessage(
              err,
              "laden",
              "Eltern",
              "Eltern konnten nicht geladen werden.",
            ),
          );
        }
      })
      .finally(() => {
        if (active) setGuardiansLoading(false);
      });
    return () => {
      active = false;
    };
  }, [student, guardiansReloadKey]);

  const resetToStudentStep = () => {
    setStudent(null);
    setError(null);
  };

  const handlePickGuardian = async (guardian: Guardian) => {
    if (!student || openingId) return;
    setOpeningId(guardian.account_id);
    setError(null);
    try {
      const thread = await openThread({
        studentId: student.id,
        guardianAccountId: guardian.account_id,
      });
      onOpened(thread);
      onClose();
    } catch (err) {
      logger.error("new_message_open_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: student.id,
      });
      setError(
        getApiErrorMessage(
          err,
          "öffnen",
          "Unterhaltung",
          "Unterhaltung konnte nicht geöffnet werden.",
        ),
      );
    } finally {
      setOpeningId(null);
    }
  };

  return (
    <Modal isOpen onClose={onClose} title="Neue Nachricht">
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
              <p className="py-3 text-center text-sm text-gray-500">Suche...</p>
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

      {/* STEP 2 — pick the recipient guardian → opens the chat */}
      {student && (
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
          <p className="text-sm font-medium text-gray-700">Eltern wählen</p>
          {error && <Alert type="error" message={error} />}
          {guardiansLoading ? (
            <p className="py-3 text-center text-sm text-gray-500">Lädt...</p>
          ) : guardiansError ? (
            <div className="space-y-2">
              <Alert type="error" message={guardiansError} />
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={() => setGuardiansReloadKey((k) => k + 1)}
              >
                Erneut versuchen
              </Button>
            </div>
          ) : guardians.length === 0 ? (
            <p className="rounded-lg border border-gray-200 bg-gray-50 px-3 py-3 text-sm text-gray-600">
              Für dieses Kind ist kein Eltern-Zugang hinterlegt.
            </p>
          ) : (
            <div className="space-y-1">
              {guardians.map((g) => (
                <button
                  key={g.account_id}
                  type="button"
                  onClick={() => void handlePickGuardian(g)}
                  disabled={openingId !== null}
                  className="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-left hover:bg-gray-50 disabled:opacity-60"
                >
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-medium text-gray-900">
                      {g.name}
                    </span>
                    <span className="block text-xs text-gray-500">
                      {relationshipLabel(g.relationship_type)}
                    </span>
                  </span>
                  <span className="ml-2 flex flex-shrink-0 items-center gap-2">
                    {g.is_primary && (
                      <span className="rounded-full bg-[#83CD2D]/10 px-2 py-0.5 text-xs font-medium text-[#669f21]">
                        primär
                      </span>
                    )}
                    {openingId === g.account_id && (
                      <span className="text-xs text-gray-400">Öffnen...</span>
                    )}
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
