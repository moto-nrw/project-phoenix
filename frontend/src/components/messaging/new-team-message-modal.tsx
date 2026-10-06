"use client";

import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";
import { Modal } from "~/components/ui/modal";
import { formErrorMessage } from "~/components/ui/form-error";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  type MessageableStaff,
  type StaffMessagesApi,
  staffRoleKindLabel,
} from "~/lib/staff-messages-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "NewTeamMessageModal" });

/**
 * Starts an internal conversation: pick one colleague, the chat opens directly.
 *
 * One step, unlike the parent-facing NewMessageModal (child → guardian): the
 * recipient IS the whole choice here. The backend get-or-creates the
 * conversation, so picking someone you already write with lands in the existing
 * history rather than creating a second thread.
 *
 * Portal-neutral (#2208): the API client and the reader's portal come in as
 * props, so the school portal reuses it against its own proxy routes.
 */
export function NewTeamMessageModal({
  api,
  portal,
  hint,
  onClose,
  onOpened,
}: {
  readonly api: StaffMessagesApi;
  readonly portal: "tenant" | "school";
  /** The one line under the title that says who can be reached here. */
  readonly hint: string;
  readonly onClose: () => void;
  readonly onOpened: (threadId: string) => void;
}) {
  const [query, setQuery] = useState("");
  const [people, setPeople] = useState<MessageableStaff[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  // Increments on "Wiederholen" and reloads the list.
  const [loadAttempt, setLoadAttempt] = useState(0);
  // Both errors stay in the open dialog (#2517): the list with a retry,
  // opening a chat in the alert above it.
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;
  const [openingId, setOpeningId] = useState<string | null>(null);
  const openError = useApiFormError();
  const latestPickRef = useRef<(person: MessageableStaff) => void>(
    () => undefined,
  );

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setLoadFailed(false);
    clearLoadError();
    api
      .fetchRecipients()
      .then((rows) => {
        if (!cancelled) setPeople(rows);
      })
      .catch((err: unknown) => {
        logger.error("team_recipients_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          setLoadFailed(true);
          void showLoadError(err, {
            object: "die Liste der Personen",
            retry: () => setLoadAttempt((attempt) => attempt + 1),
          });
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [api, loadAttempt, showLoadError, clearLoadError]);

  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) return people;
    return people.filter((person) => person.name.toLowerCase().includes(term));
  }, [people, query]);

  const handlePick = async (person: MessageableStaff) => {
    if (openingId) return;
    setOpeningId(person.account_id);
    openError.clear();
    try {
      const thread = await api.openThread(person.account_id);
      onOpened(thread.thread_id);
    } catch (err) {
      logger.error("team_thread_open_failed", {
        error: err instanceof Error ? err.message : String(err),
        account_id: person.account_id,
      });
      void openError.show(err, {
        object: "die Unterhaltung",
        retry: () => latestPickRef.current(person),
      });
    } finally {
      setOpeningId(null);
    }
  };
  useLayoutEffect(() => {
    latestPickRef.current = (person) => void handlePick(person);
  });

  // Until the catalog text is there, the failed list keeps its loading
  // line, never "Es gibt niemanden, dem Sie schreiben können."
  const loading =
    isLoading || (loadFailed && formErrorMessage(loadError.error) === null);

  return (
    <Modal isOpen onClose={onClose} title="Neue Nachricht">
      <div className="space-y-3">
        <p className="text-sm text-gray-600">{hint}</p>

        <Input
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder="Person suchen..."
          autoFocus
        />

        <FormErrorAlert message={openError.error} />
        <LoadErrorAlert error={loadError.error} />

        <div className="max-h-72 space-y-1 overflow-y-auto">
          {loading && (
            <p className="py-3 text-center text-sm text-gray-500">
              Wird geladen...
            </p>
          )}

          {!loading && !loadFailed && filtered.length === 0 && (
            <p className="py-3 text-center text-sm text-gray-500">
              {people.length === 0
                ? "Es gibt niemanden, dem Sie schreiben können."
                : "Niemand gefunden."}
            </p>
          )}

          {filtered.map((person) => {
            const roleLabel = staffRoleKindLabel(person.role_kind, portal);
            return (
              <button
                key={person.account_id}
                type="button"
                disabled={openingId !== null}
                onClick={() => void handlePick(person)}
                className="flex w-full items-center justify-between rounded-lg border border-gray-200 px-3 py-2 text-left hover:bg-gray-50 disabled:opacity-60"
              >
                <span className="flex min-w-0 items-baseline gap-2">
                  <span className="truncate text-sm font-medium text-gray-900">
                    {person.name}
                  </span>
                  {roleLabel && (
                    <span className="flex-shrink-0 text-xs text-gray-500">
                      {roleLabel}
                    </span>
                  )}
                </span>
                {openingId === person.account_id && (
                  <span className="ml-2 flex-shrink-0 text-xs text-gray-500">
                    Wird geöffnet...
                  </span>
                )}
              </button>
            );
          })}
        </div>
      </div>
    </Modal>
  );
}
