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
  const { userContext } = useUserContext();
  const myGroups = userContext?.educationalGroupIds ?? [];
  const myGroupRooms = userContext?.educationalGroupRoomNames ?? [];
  const mySupervisedRooms = userContext?.supervisedRoomNames ?? [];
  const now = useMinuteClock();

  const { data, error, isLoading } = useSWRAuth<{ students: Student[] }>(
    `room-students-${roomId}`,
    async () =>
      studentService.getStudents({
        roomId,
        includePickupTimes: true,
        includeArrivalTimes: true,
      }),
  );

  if (error) {
    logger.warn("students_in_room_load_failed", {
      room_id: roomId,
      error: error instanceof Error ? error.message : String(error),
    });
  }

  const students = data?.students ?? [];
  const totalCount = students.length;

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

      <StudentsInRoomBody
        roomId={roomId}
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
  readonly roomId: string;
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
  roomId,
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
    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
      {students.map((student) => (
        <StudentCard
          key={student.id}
          studentId={student.id}
          firstName={student.first_name}
          lastName={student.second_name}
          onClick={() =>
            router.push(`/students/${student.id}?from=/rooms/${roomId}`)
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
          }
        />
      ))}
    </div>
  );
}
