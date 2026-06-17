"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Modal } from "~/components/ui/modal";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { type Child, listMyChildren, startThread } from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "ParentNewThreadModal" });

/**
 * New-conversation flow for the parents portal. Step 1 picks the child (auto
 * if there is only one), step 2 collects subject + first message. On success
 * it navigates to the created thread's chat view.
 */
export function NewThreadModal({
  isOpen,
  onClose,
}: {
  readonly isOpen: boolean;
  readonly onClose: () => void;
}) {
  const router = useRouter();
  const [children, setChildren] = useState<Child[]>([]);
  const [childrenLoading, setChildrenLoading] = useState(true);
  const [childrenError, setChildrenError] = useState<string | null>(null);

  const [studentId, setStudentId] = useState("");
  const [subject, setSubject] = useState("");
  const [body, setBody] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Load the parent's children once the modal opens.
  useEffect(() => {
    if (!isOpen) return;
    let active = true;
    setChildrenLoading(true);
    setChildrenError(null);
    void (async () => {
      try {
        const list = await listMyChildren();
        if (!active) return;
        setChildren(list);
        // Auto-select when there is exactly one child.
        if (list.length === 1) setStudentId(list[0]!.student_id);
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        logger.warn("parent_new_thread_children_load_failed", {
          error: message,
        });
        if (active) setChildrenError(message);
      } finally {
        if (active) setChildrenLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [isOpen]);

  // Reset the form whenever the modal closes.
  useEffect(() => {
    if (isOpen) return;
    setStudentId("");
    setSubject("");
    setBody("");
    setSubmitError(null);
    setSubmitting(false);
  }, [isOpen]);

  const childOptions = useMemo(
    () =>
      children.map((child) => ({
        value: child.student_id,
        label:
          `${child.first_name} ${child.last_name}` +
          (child.school_name ? ` · ${child.school_name}` : ""),
      })),
    [children],
  );

  const canSubmit =
    studentId.length > 0 &&
    subject.trim().length > 0 &&
    body.trim().length > 0 &&
    !submitting;

  const handleSubmit = useCallback(async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const thread = await startThread({
        studentId,
        subject: subject.trim(),
        body: body.trim(),
      });
      router.push(`/parents/messages/${thread.thread_id}`);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      logger.warn("parent_new_thread_submit_failed", { error: message });
      setSubmitError("Die Nachricht konnte nicht gesendet werden.");
      setSubmitting(false);
    }
  }, [canSubmit, studentId, subject, body, router]);

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="Neue Nachricht"
      footer={
        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={submitting}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void handleSubmit()}
            disabled={!canSubmit}
          >
            {submitting ? "Senden…" : "Senden"}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {submitError ? <Alert type="error" message={submitError} /> : null}

        {childrenLoading ? (
          <div className="h-10 animate-pulse rounded-lg bg-gray-100" />
        ) : childrenError ? (
          <Alert
            type="error"
            message="Die Kinder konnten nicht geladen werden."
          />
        ) : children.length === 0 ? (
          <Alert
            type="info"
            message="Für Ihr Konto ist noch kein Kind hinterlegt."
          />
        ) : (
          <>
            <div>
              <span
                id="new-thread-child-label"
                className="mb-1.5 block text-sm font-medium text-gray-700"
              >
                Kind
              </span>
              <CustomSelect
                value={studentId}
                options={childOptions}
                onChange={setStudentId}
                labelId="new-thread-child-label"
                placeholder="Kind auswählen"
                disabled={submitting || children.length === 1}
              />
            </div>

            <Input
              label="Betreff"
              value={subject}
              onChange={(event) => setSubject(event.target.value)}
              placeholder="Worum geht es?"
              maxLength={200}
              disabled={submitting}
            />

            <div>
              <label
                htmlFor="new-thread-body"
                className="mb-1.5 block text-sm font-medium text-gray-700"
              >
                Nachricht
              </label>
              <textarea
                id="new-thread-body"
                rows={4}
                value={body}
                onChange={(event) => setBody(event.target.value)}
                disabled={submitting}
                placeholder="Ihre Nachricht an die OGS…"
                className="block w-full resize-none rounded-lg border-0 bg-white px-4 py-3 text-base text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all duration-200 ring-inset placeholder:text-gray-400 focus:outline-none focus:ring-inset focus-visible:ring-2 focus-visible:ring-gray-400 disabled:bg-gray-50 disabled:text-gray-500 disabled:ring-gray-200"
              />
            </div>
          </>
        )}
      </div>
    </Modal>
  );
}
