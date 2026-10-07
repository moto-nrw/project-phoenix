import { useCallback, useEffect, useState } from "react";
import {
  fetchStudentEnrollmentExtraFields,
  type StudentEnrollmentExtraFieldGroup,
} from "~/lib/student-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "useStudentEnrollmentExtraFields" });

export function useStudentEnrollmentExtraFields(
  studentId: string,
  hasFullAccess: boolean,
) {
  const [groups, setGroups] = useState<StudentEnrollmentExtraFieldGroup[]>([]);
  const [loading, setLoading] = useState(false);
  const [hasError, setHasError] = useState(false);
  // The raw failure for the shared load error path (#2517): the card shows
  // the catalog text with "Wiederholen" instead of silently dropping the
  // Anmeldung answers.
  const [error, setError] = useState<unknown>(null);
  const [attempt, setAttempt] = useState(0);
  const reload = useCallback(() => setAttempt((count) => count + 1), []);

  useEffect(() => {
    let cancelled = false;
    if (!hasFullAccess) {
      setGroups([]);
      setLoading(false);
      setHasError(false);
      setError(null);
      return () => {
        cancelled = true;
      };
    }

    setGroups([]);
    setLoading(true);
    setHasError(false);
    setError(null);
    fetchStudentEnrollmentExtraFields(studentId)
      .then((nextGroups) => {
        if (cancelled) return;
        setGroups(nextGroups);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setGroups([]);
        setHasError(true);
        setError(err);
        logger.warn("student_enrollment_extra_fields_load_failed", {
          student_id: studentId,
          error: err instanceof Error ? err.message : String(err),
        });
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [hasFullAccess, studentId, attempt]);

  return { groups, loading, hasError, error, reload };
}
