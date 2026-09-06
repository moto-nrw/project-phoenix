"use client";

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { ParentSectionSkeleton } from "~/components/parent/parent-page";
import { StatusBadge } from "~/components/ui/status-badge";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  getChildCourses,
  requestChildCourse,
  withdrawChildCourseRequest,
  type ChildCourses,
  type CourseItem,
} from "~/lib/parent-api";

const logger = createLogger({ component: "CoursesSection" });

const DAY_KEY_TO_WEEKDAY: Record<string, number> = {
  mon: 1,
  tue: 2,
  wed: 3,
  thu: 4,
  fri: 5,
  sat: 6,
  sun: 7,
};

/**
 * "Kurse": die AGs der Schule, die Eltern für ihr Kind anfragen können
 * (#3075, SH 4.3, ADR 0012).
 *
 * Der Vorgang heißt hier überall Anfrage, nie Anmeldung: er wird erst
 * wirksam, wenn die OGS ihn bestätigt. Genau das war das Missverständnis
 * aus #2296, und die Benennung ist die Absicherung dagegen, nicht der
 * Schulschalter.
 *
 * Ist die Funktion an der Schule aus, rendert der Abschnitt nichts. Eine
 * Liste ohne Handlungsmöglichkeit wird nicht wieder eingeführt (#2303).
 */
export function CoursesSection({
  studentId,
  careEnded,
}: Readonly<{
  studentId: string;
  /** Nach dem Ende der Betreuung gibt es nichts mehr anzufragen. */
  careEnded: boolean;
}>) {
  const t = useTranslations("parentMasterData");
  const locale = useLocale();
  const [courses, setCourses] = useState<ChildCourses | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [busyCourseId, setBusyCourseId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setCourses(await getChildCourses(studentId));
      setLoadFailed(false);
    } catch (err: unknown) {
      setCourses(null);
      setLoadFailed(true);
      logger.warn("parent_courses_load_failed", {
        error: err instanceof Error ? err.message : String(err),
        student_id: studentId,
      });
    } finally {
      setLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const dayList = useCallback(
    (days: string[]) => {
      const weekdays = days.flatMap((day) => {
        const weekday = DAY_KEY_TO_WEEKDAY[day];
        return weekday === undefined ? [] : [weekday];
      });
      if (weekdays.length === 0) return "";
      return weekdays
        .sort((a, b) => a - b)
        .map((day) => t(`weekdayShort.${day}`))
        .join(", ");
    },
    [t],
  );

  const runAction = useCallback(
    async (courseId: string, action: () => Promise<ChildCourses>) => {
      setBusyCourseId(courseId);
      setActionError(null);
      try {
        setCourses(await action());
      } catch (err: unknown) {
        // Der Grund steht im Log, nicht auf dem Bildschirm: er ist auf
        // Englisch und nennt Dinge, mit denen eine Familie nichts anfangen
        // kann. Die Liste wird neu geladen, damit ein inzwischen voller Kurs
        // sofort als voll dasteht.
        logger.warn("parent_course_action_failed", {
          error: err instanceof Error ? err.message : String(err),
          student_id: studentId,
          course_id: courseId,
        });
        setActionError(t("courses.actionError"));
        void load();
      } finally {
        setBusyCourseId(null);
      }
    },
    [load, studentId, t],
  );

  if (loading) return <ParentSectionSkeleton rows={3} />;

  if (loadFailed) {
    return (
      <ParentSection title={t("courses.title")} concept="activities">
        <Alert type="error" message={t("courses.loadError")} />
      </ParentSection>
    );
  }

  // Die Schule bietet das nicht an, oder diese Person darf nicht anfragen:
  // dann steht hier nichts. Ein leerer Abschnitt wäre eine Frage ohne
  // Antwort.
  if (
    courses === null ||
    courses.disabled_reason === "school_disabled" ||
    courses.disabled_reason === "no_permission"
  ) {
    return null;
  }

  const requestable =
    courses.enabled && !careEnded && !courses.other_request_pending;

  return (
    <ParentSection
      title={t("courses.title")}
      description={t("courses.description")}
      concept="activities"
    >
      {courses.items.length === 0 ? (
        <EmptyState
          className="border-t border-gray-100"
          title={
            courses.disabled_reason === "no_enrollment"
              ? t("courses.needsEnrollment")
              : t("courses.noCourses")
          }
        />
      ) : (
        <ul className="space-y-2">
          {courses.items.map((course) => (
            <li
              key={course.id}
              className="flex flex-wrap items-start justify-between gap-3 rounded-xl border border-gray-200 p-3"
            >
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">
                  {course.name}
                </p>
                {course.description ? (
                  <p className="mt-0.5 text-sm text-gray-700">
                    {course.description}
                  </p>
                ) : null}
                <p className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-sm text-gray-500">
                  {dayList(course.available_days) ? (
                    <span>{dayList(course.available_days)}</span>
                  ) : null}
                  <CourseSlots course={course} />
                </p>
              </div>
              <div className="flex shrink-0 flex-col items-end gap-2">
                <CourseState course={course} />
                <CourseAction
                  course={course}
                  courses={courses}
                  requestable={requestable}
                  busy={busyCourseId === course.id}
                  onRequest={() =>
                    void runAction(course.id, () =>
                      requestChildCourse(studentId, course.id),
                    )
                  }
                  onWithdraw={() =>
                    void runAction(course.id, () =>
                      withdrawChildCourseRequest(
                        studentId,
                        courses.pending_request_id ?? "",
                      ),
                    )
                  }
                />
              </div>
            </li>
          ))}
        </ul>
      )}

      {courses.other_request_pending ? (
        <p className="text-sm text-gray-500">{t("courses.otherRequest")}</p>
      ) : null}
      {courses.items.length > 0 && courses.effective_from ? (
        <p className="text-sm text-gray-500">
          {t("courses.effectiveFrom", {
            date: formatDate(courses.effective_from, false, locale),
          })}
        </p>
      ) : null}
      {actionError ? <Alert type="error" message={actionError} /> : null}
    </ParentSection>
  );
}

/** Freie Plätze im Klartext. Ohne Grenze steht hier nichts. */
function CourseSlots({ course }: Readonly<{ course: CourseItem }>) {
  const t = useTranslations("parentMasterData");
  if (course.free_slots === undefined) return null;
  if (course.free_slots <= 0) {
    // Wer auf "Anfragen" tippt, soll vorher wissen, worauf es hinausläuft.
    // Für ein Kind, das schon dabei ist oder schon angefragt hat, wäre der
    // Zusatz eine Aufforderung ins Leere.
    const full =
      course.booked || course.requested
        ? t("courses.full")
        : t("courses.fullWaitlistHint");
    return <span>{full}</span>;
  }
  if (course.free_slots === 1) return <span>{t("courses.freeSlotOne")}</span>;
  return <span>{t("courses.freeSlots", { count: course.free_slots })}</span>;
}

/** Der Stand des Kindes bei diesem Kurs, in einem Wort. */
function CourseState({ course }: Readonly<{ course: CourseItem }>) {
  const t = useTranslations("parentMasterData");
  if (course.booked) {
    return <StatusBadge label={t("courses.booked")} tone="green" />;
  }
  if (course.waitlisted) {
    return (
      <StatusBadge
        label={t("courses.waitlist", {
          position: course.waitlist_position ?? 1,
        })}
        tone="orange"
      />
    );
  }
  if (course.requested) {
    return <StatusBadge label={t("courses.requested")} tone="orange" />;
  }
  return null;
}

function CourseAction({
  course,
  courses,
  requestable,
  busy,
  onRequest,
  onWithdraw,
}: Readonly<{
  course: CourseItem;
  courses: ChildCourses;
  requestable: boolean;
  busy: boolean;
  onRequest: () => void;
  onWithdraw: () => void;
}>) {
  const t = useTranslations("parentMasterData");
  if (course.booked) return null;
  if (course.requested) {
    if (!courses.pending_submitted_by_self || !courses.pending_request_id) {
      return null;
    }
    return (
      <Button
        type="button"
        variant="outline"
        size="md"
        className="max-sm:min-h-11"
        disabled={busy}
        onClick={onWithdraw}
      >
        {t("courses.withdraw")}
      </Button>
    );
  }
  if (!requestable) return null;
  return (
    <Button
      type="button"
      variant="surface"
      size="md"
      className="max-sm:min-h-11"
      disabled={busy}
      onClick={onRequest}
    >
      {t("courses.request")}
    </Button>
  );
}
