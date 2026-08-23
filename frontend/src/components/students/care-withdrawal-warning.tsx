"use client";

import { useEffect, useState } from "react";

import { CareExitModal } from "~/components/students/care-exit-modal";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  fetchStudentCareWithdrawal,
  type CareWithdrawalCompletion,
} from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "CareWithdrawalWarning" });

function useCareWithdrawalTask(enabled: boolean, studentId: string) {
  const [task, setTask] = useState<CareWithdrawalCompletion | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  useEffect(() => {
    if (!enabled || !studentId) {
      setTask(null);
      setLoadFailed(false);
      return;
    }
    let cancelled = false;
    const load = () => {
      void fetchStudentCareWithdrawal(studentId)
        .then((result) => {
          if (!cancelled) setTask(result);
          if (!cancelled) setLoadFailed(false);
        })
        .catch((error: unknown) => {
          if (cancelled) return;
          logger.warn("care_withdrawal_warning_load_failed", {
            student_id: studentId,
            error: error instanceof Error ? error.message : String(error),
          });
          setTask(null);
          setLoadFailed(true);
        });
    };
    load();
    window.addEventListener("change-requests-refresh", load);
    return () => {
      cancelled = true;
      window.removeEventListener("change-requests-refresh", load);
    };
  }, [enabled, studentId]);
  return { task, setTask, loadFailed };
}

export function CareWithdrawalWarning({
  enabled,
  studentId,
}: Readonly<{
  enabled: boolean;
  studentId: string;
}>) {
  const { task, setTask, loadFailed } = useCareWithdrawalTask(
    enabled,
    studentId,
  );

  if (!enabled) return null;
  if (loadFailed) {
    return (
      <div className="mt-4">
        <Alert
          type="error"
          message="Die offene Abmeldung konnte nicht geladen werden."
        />
      </div>
    );
  }
  if (!task) return null;

  return (
    <CareWithdrawalTaskNotice
      task={task}
      studentId={studentId}
      onFinished={() => setTask(null)}
    />
  );
}

function CareWithdrawalTaskNotice({
  task,
  studentId,
  onFinished,
}: Readonly<{
  task: CareWithdrawalCompletion;
  studentId: string;
  onFinished: () => void;
}>) {
  const [modalOpen, setModalOpen] = useState(false);
  const finish = () => {
    setModalOpen(false);
    onFinished();
    window.dispatchEvent(new Event("change-requests-refresh"));
  };

  return (
    <div className="mt-4">
      <Alert
        type="warning"
        message={`Abmeldung noch abschließen. Ab ${formatDate(task.firstBookinglessDay)} ist kein Betreuungstag mehr gebucht.`}
        action={
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setModalOpen(true)}
          >
            Betreuung beenden
          </Button>
        }
      />
      <CareExitModal
        isOpen={modalOpen}
        studentIds={[studentId]}
        completionId={task.id}
        firstBookinglessDay={task.firstBookinglessDay}
        onClose={() => setModalOpen(false)}
        onFinished={finish}
      />
    </div>
  );
}
