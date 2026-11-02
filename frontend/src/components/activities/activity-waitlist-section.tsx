"use client";

import { useSession } from "next-auth/react";

import { CourseWaitlistSummary } from "~/components/students/course-waitlist";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SectionCard } from "~/components/ui/section-card";
import { hasPermission } from "~/lib/auth-utils";
import { getEnrolledStudents } from "~/lib/activity-api";
import { fetchCourseWaitlists } from "~/lib/course-waitlist-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import {
  activityWaitlistKeys,
  courseWaitlistKeys,
} from "~/lib/swr/course-waitlist-keys";
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
  const mayReadParticipants = hasPermission(session, "activities:read");
  const swr = useSWRAuth(
    allowed ? courseWaitlistKeys.activity(activityId) : null,
    () => fetchCourseWaitlists(activityId),
  );
  // Die Namen der Teilnehmenden kommen aus der Kinderliste der Aktivität.
  const participants = useSWRAuth(
    allowed && mayReadParticipants
      ? activityWaitlistKeys.participants(activityId)
      : null,
    () => getEnrolledStudents(activityId),
  );
  // Wenn die Teilnehmerliste gelesen wird, darf sie nicht unbemerkt fehlen:
  // dann wäre die dargestellte Warteliste unvollständig und irreführend.
  const loadError = useSwrLoadError(
    swr.error ?? participants.error,
    swr.error ? "die Warteliste der AG" : "die Teilnehmenden der AG",
    () => {
      if (swr.error) void swr.mutate();
      if (participants.error) void participants.mutate();
    },
  );
  const waitlist = swr.data?.[0];
  const participantNames = participants.data
    ?.map((student) => student.name ?? "")
    .filter((name) => name !== "");
  if (!allowed || (!waitlist && !loadError)) return null;
  return (
    <SectionCard
      id={`activity-waitlist-${activityId}`}
      title="Plätze und Warteliste"
      description="Heute dabei und freie Plätze für die nächste Anfrage."
      headingLevel={3}
      namedRegion
      bodyClassName="mt-3"
    >
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
    </SectionCard>
  );
}
