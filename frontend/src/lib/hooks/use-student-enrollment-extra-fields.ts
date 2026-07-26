import { useEffect, useState } from "react";
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

  useEffect(() => {
    let cancelled = false;
    if (!hasFullAccess) {
      setGroups([]);
      setLoading(false);
      setHasError(false);
      return () => {
        cancelled = true;
      };
    }

    setGroups([]);
    setLoading(true);
    setHasError(false);
    fetchStudentEnrollmentExtraFields(studentId)
      .then((nextGroups) => {
        if (cancelled) return;
        setGroups(nextGroups);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setGroups([]);
        setHasError(true);
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
  }, [hasFullAccess, studentId]);

  return { groups, loading, hasError };
}
