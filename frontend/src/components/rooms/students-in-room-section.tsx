// components/rooms/students-in-room-section.tsx
//
// Live "Kinder im Raum" view rendered on /rooms/{id} (#1323).
//
// Reuses the same `/api/students?room_id=` data path as Kindersuche so the
// cards here are visually identical to the rest of the app — same component,
// same props, same redaction model. Names are visible to all authenticated
// staff; deeper detail rows (Ankunft/Abholzeit, notes, RFID tag) are gated
// server-side per row via `has_full_access`. The total count is the response
// length — every row shown is a real, currently checked-in student.
//
// Live updates: SSE in `use-global-sse` invalidates `room-students-{roomId}`
// on student_checkin / student_checkout / activity_start / activity_end /
// dashboard_counts_changed events.

"use client";

import { usePathname, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSWRAuth } from "~/lib/swr";
import { InfoCard } from "~/components/ui/info-card";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { StudentPresenceBadge } from "~/components/ui/student-presence-badge";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  StudentCard,
  StudentInfoRow,
  SchoolClassIcon,
  GroupIcon,
  PickupTimeRow,
  ArrivalTimeRow,
} from "~/components/students/student-card";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentsInRoomSection" });

interface StudentsInRoomSectionProps {
  readonly roomId: string;
  readonly roomName: string;
}

export function StudentsInRoomSection({
  roomId,
  roomName,
}: StudentsInRoomSectionProps) {
  const router = useTenantRouter();
  const pathname = usePathname();
  const sectionSearchParams = useSearchParams();
  // Drilling into a child must return to the same UI state the user
  // came from. The section renders in two contexts:
  //   - Legacy /rooms/{id} subpage → return there.
  //   - New modal at /rooms?room={id} → return to the grid with the
  //     full query string intact (room=… AND any filter params like
  //     ?search=…&building=…&status=…) so the user lands back on the
  //     same narrowed view, with the modal reopened.
  const fromReferrer = (() => {
    if (pathname && pathname.endsWith(`/rooms/${roomId}`)) {
      return `/rooms/${roomId}`;
    }
    const qs = sectionSearchParams?.toString() ?? "";
    return qs ? `/rooms?${qs}` : `/rooms?room=${roomId}`;
  })();
  const { userContext } = useUserContext();
  const myGroups = userContext?.educationalGroupIds ?? [];
  const myGroupRooms = userContext?.educationalGroupRoomNames ?? [];
  const mySupervisedRooms = userContext?.supervisedRoomNames ?? [];
  const now = useMinuteClock();

  // pageSize must comfortably exceed any realistic room occupancy (combined
  // groups in Sporthalle/Aula/Mensa cap well below 100 in normal usage) so
  // every present child shows up. Backend default is only 50, which would
  // silently truncate larger rooms — see PR #1374 review.
  const { data, error, isLoading } = useSWRAuth<{
    students: Student[];
    pagination?: { total_records: number };
  }>(`room-students-${roomId}`, async () =>
    studentService.getStudents({
      roomId,
      includePickupTimes: true,
      includeArrivalTimes: true,
      pageSize: 200,
    }),
  );

  if (error) {
    logger.warn("students_in_room_load_failed", {
      room_id: roomId,
      error: error instanceof Error ? error.message : String(error),
    });
  }

  const students = data?.students ?? [];
  // Prefer the server's authoritative count so the badge stays honest even
  // if pagination ever truncates the response. Fall back to length only
  // when pagination metadata is missing.
  const totalCount = data?.pagination?.total_records ?? students.length;
  // If the response is truncated (room has more occupants than pageSize),
  // surface the overflow + offer the Kindersuche escape hatch (#1374).
  // Without this notice, staff see N cards while the badge says >N and
  // have no affordance to open the missing children.
  const isTruncated = totalCount > students.length;
  const hiddenCount = Math.max(0, totalCount - students.length);

  const openInSearch = () => {
    const qs = new URLSearchParams({
      room_id: roomId,
      room_name: roomName,
    }).toString();
    router.push(`/students/search?${qs}`);
  };

  return (
    <InfoCard
      title="Kinder im Raum"
      icon={
        <svg
          className="h-5 w-5"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
          />
        </svg>
      }
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-gray-600">
          <span className="font-medium text-gray-900">{totalCount}</span>{" "}
          {totalCount === 1 ? "Kind" : "Kinder"} aktuell anwesend
        </p>
        {totalCount > 0 && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={openInSearch}
          >
            In Kindersuche öffnen
          </Button>
        )}
      </div>

      {isTruncated && (
        <div
          role="status"
          className="mt-3 rounded-lg border border-amber-200 bg-amber-50 p-3 text-sm text-amber-900"
        >
          Es werden {students.length} von {totalCount} Kindern angezeigt.{" "}
          {hiddenCount} weitere {hiddenCount === 1 ? "Kind ist" : "Kinder sind"}{" "}
          aktuell nicht in dieser Übersicht — bitte in der Kindersuche öffnen,
          um alle zu sehen.
        </div>
      )}

      <StudentsInRoomBody
        fromReferrer={fromReferrer}
        loading={isLoading}
        hasError={!!error}
        students={students}
        myGroups={myGroups}
        myGroupRooms={myGroupRooms}
        mySupervisedRooms={mySupervisedRooms}
        now={now}
        router={router}
      />
    </InfoCard>
  );
}

interface StudentsInRoomBodyProps {
  readonly fromReferrer: string;
  readonly loading: boolean;
  readonly hasError: boolean;
  readonly students: readonly Student[];
  readonly myGroups: string[];
  readonly myGroupRooms: string[];
  readonly mySupervisedRooms: string[];
  readonly now: Date;
  readonly router: ReturnType<typeof useTenantRouter>;
}

function StudentsInRoomBody({
  fromReferrer,
  loading,
  hasError,
  students,
  myGroups,
  myGroupRooms,
  mySupervisedRooms,
  now,
  router,
}: StudentsInRoomBodyProps) {
  if (hasError) {
    return (
      <Alert
        type="error"
        message="Die Liste der Kinder konnte nicht geladen werden."
      />
    );
  }

  if (loading && students.length === 0) {
    return <Loading message="Kinderliste wird geladen..." fullPage={false} />;
  }

  if (students.length === 0) {
    return (
      <div className="py-8 text-center text-gray-500">
        Aktuell keine Kinder im Raum.
      </div>
    );
  }

  return (
    // Container queries (Tailwind v4) so the grid responds to its own
    // width, not the viewport. This keeps the cards readable inside the
    // narrower /rooms detail modal while still allowing 3 columns on the
    // wider Kindersuche page where the same component is rendered.
    <div className="@container">
      <div className="grid grid-cols-1 gap-6 @3xl:grid-cols-2 @5xl:grid-cols-3">
        {students.map((student) => (
          <StudentCard
            key={student.id}
            studentId={student.id}
            firstName={student.first_name}
            lastName={student.second_name}
            onClick={() =>
              router.push(
                `/students/${student.id}?from=${encodeURIComponent(fromReferrer)}`,
              )
            }
            locationBadge={
              <StudentPresenceBadge
                student={{
                  ...student,
                  not_arrival_today:
                    (student.arrival_is_exception ?? false) &&
                    !student.arrival_time,
                  not_arrival_reason: student.arrival_notes ?? null,
                }}
                displayMode="contextAware"
                userGroups={myGroups}
                groupRooms={myGroupRooms}
                supervisedRooms={mySupervisedRooms}
                variant="modern"
                size="md"
              />
            }
            extraContent={
              <>
                <StudentInfoRow icon={<SchoolClassIcon />}>
                  {student.school_class}
                </StudentInfoRow>
                {student.group_name && (
                  <StudentInfoRow icon={<GroupIcon />}>
                    Gruppe: {student.group_name}
                  </StudentInfoRow>
                )}
                {/* Backend skips enriching arrival/pickup fields for
                    students the viewer has no full access to, so the rows
                    would render as "Ankunftszeit: —" placeholders. Hide
                    them entirely to match the Kindersuche redacted card
                    shape (students/search/page.tsx). */}
                {student.has_full_access !== false && (
                  <>
                    <ArrivalTimeRow
                      arrivalTime={student.arrival_time}
                      actualTime={student.actual_arrival_time}
                      isException={student.arrival_is_exception ?? false}
                      isAbsent={
                        (student.arrival_is_exception ?? false) &&
                        !student.arrival_time
                      }
                      notes={student.arrival_notes}
                      now={now}
                    />
                    <PickupTimeRow
                      pickupTime={student.pickup_time ?? undefined}
                      actualTime={student.actual_pickup_time}
                      isException={student.pickup_is_exception ?? false}
                      notes={student.pickup_notes}
                      now={now}
                    />
                  </>
                )}
              </>
            }
          />
        ))}
      </div>
    </div>
  );
}
