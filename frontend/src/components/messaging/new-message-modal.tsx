"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { ArrowLeft } from "lucide-react";
import {
  StudentChoiceList,
  type StudentChoicesLoad,
  matchStudentChoices,
  useStudentChoices,
} from "~/components/students/student-choice-list";
import { Button } from "~/components/ui/button";
import {
  SlideOver,
  SlideOverBody,
  SlideOverCloseButton,
  SlideOverContent,
  SlideOverDescription,
  SlideOverFooter,
  SlideOverHeader,
  SlideOverTitle,
} from "~/components/ui/slide-over";
import { formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  type Guardian,
  type ThreadDetail,
  fetchGuardians,
  fetchReachableStudentIds,
  openThread,
  relationshipLabel,
} from "~/lib/parent-messages-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "NewMessageModal" });

const NO_PARENT_ACCESS = "Kein Eltern-Zugang";
const NO_STUDENT_IDS: ReadonlySet<string> = new Set();

/**
 * The children with a guardian who can receive a message (#4362): undefined
 * while loading, null when the answer is unknown.
 */
function useReachableStudentIds(
  enabled: boolean,
): ReadonlySet<string> | null | undefined {
  const [ids, setIds] = useState<ReadonlySet<string> | null | undefined>(
    undefined,
  );
  useEffect(() => {
    if (!enabled) return;
    let active = true;
    fetchReachableStudentIds()
      .then((result) => {
        if (active) setIds(result);
      })
      .catch((err) => {
        // The mark is an extra: without it the list works as before and the
        // parents step still says when a child has no parent access.
        logger.warn("new_message_reachable_students_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (active) setIds(null);
      });
    return () => {
      active = false;
    };
  }, [enabled]);
  return ids;
}

interface PickedStudent {
  id: string;
  name: string;
  /** Klasse · Gruppe, empty for a preset child. */
  detail?: string;
}

/**
 * Lets staff start a conversation: (1) pick a child from the alphabetical
 * list of all children, filtered by the search field (#4362); a child without
 * a guardian who can receive a message is marked and cannot be picked, (2) pick the
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

  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const { load: choicesLoad, error: loadError } = useStudentChoices(!hasPreset);
  const reachableIds = useReachableStudentIds(!hasPreset);
  // Both answers arrive together, so no mark appears after the list stands.
  const load: StudentChoicesLoad =
    reachableIds === undefined && choicesLoad.kind === "loaded"
      ? { kind: "loading" }
      : choicesLoad;
  const unreachableIds = useMemo(
    () =>
      reachableIds && choicesLoad.kind === "loaded"
        ? new Set(
            choicesLoad.choices
              .filter((child) => !reachableIds.has(child.id))
              .map((child) => child.id),
          )
        : NO_STUDENT_IDS,
    [reachableIds, choicesLoad],
  );
  const { target, visible } = matchStudentChoices(
    load,
    search,
    selectedId,
    unreachableIds,
  );
  const showsUnreachable = visible.some((child) =>
    unreachableIds.has(child.id),
  );

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
  const [guardiansFailed, setGuardiansFailed] = useState(false);
  const [guardiansReloadKey, setGuardiansReloadKey] = useState(0);
  const guardiansError = useApiLoadError();
  const showGuardiansError = guardiansError.show;
  const clearGuardiansError = guardiansError.clear;
  const [openingId, setOpeningId] = useState<string | null>(null);
  // Opening the chat fails inside the open panel; the error stays there.
  const openError = useApiFormError();
  const latestPickRef = useRef<(guardian: Guardian) => void>(() => undefined);

  // Load the guardians of the picked child for the recipient step.
  useEffect(() => {
    if (!student) {
      setGuardians([]);
      setGuardiansFailed(false);
      clearGuardiansError();
      return;
    }
    let active = true;
    setGuardiansLoading(true);
    setGuardiansFailed(false);
    clearGuardiansError();
    fetchGuardians(student.id)
      .then((list) => {
        if (active) setGuardians(list);
      })
      .catch((err) => {
        logger.warn("new_message_guardians_load_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: student.id,
        });
        if (active) {
          setGuardians([]);
          setGuardiansFailed(true);
          void showGuardiansError(err, {
            object: "die Liste der Eltern",
            retry: () => setGuardiansReloadKey((key) => key + 1),
          });
        }
      })
      .finally(() => {
        if (active) setGuardiansLoading(false);
      });
    return () => {
      active = false;
    };
  }, [student, guardiansReloadKey, showGuardiansError, clearGuardiansError]);

  const handlePickStudent = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (target) setStudent(target);
  };

  const resetToStudentStep = () => {
    setStudent(null);
    openError.clear();
  };

  const handlePickGuardian = async (guardian: Guardian) => {
    if (!student || openingId) return;
    setOpeningId(guardian.account_id);
    openError.clear();
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
      void openError.show(err, {
        object: "die Unterhaltung",
        retry: () => latestPickRef.current(guardian),
      });
    } finally {
      setOpeningId(null);
    }
  };
  // "Wiederholen" opens with the then current child.
  useLayoutEffect(() => {
    latestPickRef.current = (guardian) => void handlePickGuardian(guardian);
  });

  // The catalog text loads on demand; until it is there the failed list
  // keeps its loading line, never "no guardians".
  const guardiansErrorPending =
    guardiansFailed && formErrorMessage(guardiansError.error) === null;

  return (
    <SlideOver
      open
      onOpenChange={(nextOpen) => {
        if (!nextOpen) onClose();
      }}
    >
      <SlideOverContent widthClass="sm:w-[560px]">
        <SlideOverHeader className="flex-row items-start justify-between gap-3">
          <div className="min-w-0">
            <SlideOverTitle>Neue Nachricht</SlideOverTitle>
            <SlideOverDescription>
              {student
                ? "Wählen Sie die Eltern, die die Nachricht erhalten."
                : "Wählen Sie zuerst das Kind, um das es geht."}
            </SlideOverDescription>
          </div>
          <SlideOverCloseButton />
        </SlideOverHeader>
        {/* STEP 1 — pick a child (skipped when a preset child is given) */}
        {!student && (
          <form
            onSubmit={handlePickStudent}
            className="flex min-h-0 flex-1 flex-col"
          >
            <SlideOverBody className="flex min-h-0 flex-col gap-3">
              <StudentChoiceList
                load={load}
                loadError={loadError}
                search={search}
                onSearchChange={setSearch}
                searchLabel="Kind suchen"
                radioName="new-message-student"
                selectedId={selectedId}
                onSelectedIdChange={setSelectedId}
                unavailableIds={unreachableIds}
                unavailableLabel={NO_PARENT_ACCESS}
                autoFocus
                fillHeight
              />
              {showsUnreachable ? (
                <p className="text-sm text-gray-500">
                  Bei „{NO_PARENT_ACCESS}“ kann niemand die Nachricht in der
                  Eltern-App lesen. Den Zugang der Eltern legen Sie beim Kind
                  fest.
                </p>
              ) : null}
            </SlideOverBody>
            <SlideOverFooter className="flex-row justify-end">
              <Button
                type="submit"
                variant="success"
                size="md"
                disabled={!target}
              >
                Weiter
              </Button>
            </SlideOverFooter>
          </form>
        )}

        {/* STEP 2 — pick the recipient guardian → opens the chat */}
        {student && (
          <div className="flex-1 overflow-y-auto px-5 py-4">
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
                {student.detail ? (
                  <p className="text-xs text-gray-500">{student.detail}</p>
                ) : null}
              </div>
              <p className="text-sm font-medium text-gray-700">Eltern wählen</p>
              <FormErrorAlert message={openError.error} />
              {guardiansLoading || guardiansErrorPending ? (
                <p className="py-3 text-center text-sm text-gray-500">
                  Lädt...
                </p>
              ) : guardiansFailed ? (
                <LoadErrorAlert error={guardiansError.error} />
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
                          <span className="bg-moto-green/10 text-moto-green-vivid rounded-full px-2 py-0.5 text-xs font-medium">
                            primär
                          </span>
                        )}
                        {openingId === g.account_id && (
                          <span className="text-xs text-gray-400">
                            Öffnen...
                          </span>
                        )}
                      </span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          </div>
        )}
      </SlideOverContent>
    </SlideOver>
  );
}
