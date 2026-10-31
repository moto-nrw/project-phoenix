"use client";

import { useSession } from "next-auth/react";

import { CourseWaitlistSummary } from "~/components/students/course-waitlist";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { hasPermission } from "~/lib/auth-utils";
import { getEnrolledStudents } from "~/lib/activity-api";
import { fetchCourseWaitlists } from "~/lib/course-waitlist-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";

/**
 * Plätze und Warteliste einer AG (#4767), im Fenster der Aktivität. Nur zur
 * Information: bestätigt wird die Anfrage des nächsten Kindes in „Anfragen“.
 * Sichtbar für alle, die Anfragen zu Angeboten entscheiden dürfen.
 */
export function ActivityWaitlistSection({
  activityId,
}: Readonly<{ activityId: string }>) {
  const { data: session } = useSession();
  const allowed = hasPermission(session, "users:update");
  const swr = useSWRAuth(allowed ? `course-waitlist-${activityId}` : null, () =>
    fetchCourseWaitlists(activityId),
  );
  // Die Namen der Teilnehmenden kommen aus der Kinderliste der Aktivität.
  const participants = useSWRAuth(
    allowed ? `activity-participants-${activityId}` : null,
    () => getEnrolledStudents(activityId),
  );
  // Belegung und Namen gehören zusammen: fehlen die Namen, wäre die
  // dargestellte Warteliste unvollständig und damit irreführend.
  const loadError = useSwrLoadError(
    swr.error ?? participants.error,
    swr.error ? "die Warteliste der AG" : "die Teilnehmenden der AG",
    () => (swr.error ? swr.mutate() : participants.mutate()),
  );
  const waitlist = swr.data?.[0];
  const participantNames = participants.data
    ?.map((student) => student.name ?? "")
    .filter((name) => name !== "");
  if (!allowed || (!waitlist && !loadError)) return null;
  return (
    <section
      aria-labelledby={`activity-waitlist-${activityId}`}
      className="rounded-xl border border-gray-200/50 bg-gray-50 p-3 md:p-4"
    >
      <h3
        id={`activity-waitlist-${activityId}`}
        className="text-xs font-semibold text-gray-700"
      >
        Plätze und Warteliste
      </h3>
      <p className="mt-0.5 mb-3 text-xs text-gray-500">
        Heute dabei und freie Plätze für die nächste Anfrage.
      </p>
      {loadError ? (
        <LoadErrorAlert error={loadError} actionLayout="responsive" />
      ) : (
        waitlist && (
          <CourseWaitlistSummary
            waitlist={waitlist}
            participantNames={participantNames}
          />
        )
      )}
    </section>
  );
}
