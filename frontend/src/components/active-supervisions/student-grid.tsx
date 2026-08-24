"use client";

import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import {
  StudentCard,
  StudentInfoRow,
  SchoolClassIcon,
  GroupIcon,
  PickupTimeRow,
  ArrivalTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import type { BulkArrivalTime } from "~/lib/student-arrival-api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import { combineTimeNotes, getStudentAbsence } from "~/lib/student-time-status";
import { getDayPlanningNotComingLabel } from "~/lib/day-planning-helper";
import type { ActiveSupervisionStudent } from "~/components/active-supervisions/view-model";
import { withActiveSupervisionPresence } from "~/components/active-supervisions/view-model";

interface SupervisionStudentGridProps {
  readonly students: readonly ActiveSupervisionStudent[];
  readonly filteredStudents: readonly ActiveSupervisionStudent[];
  readonly pickupTimesData: ReadonlyMap<string, BulkPickupTime> | undefined;
  readonly arrivalTimesData: ReadonlyMap<string, BulkArrivalTime> | undefined;
  readonly trackingData: TrackingIndicatorsResponse | undefined;
  readonly myGroupIds: readonly string[];
  readonly myGroupRooms: readonly string[];
  readonly now: Date;
  readonly onOpenStudent: (studentId: string) => void;
}

/**
 * The visitor list of the selected session: one StudentCard per checked-in
 * child, with presence badge, arrival/pickup rows, and tracking indicators.
 * Renders the no-children and no-filter-match empty states itself.
 */
export function SupervisionStudentGrid({
  students,
  filteredStudents,
  pickupTimesData,
  arrivalTimesData,
  trackingData,
  myGroupIds,
  myGroupRooms,
  now,
  onOpenStudent,
}: SupervisionStudentGridProps) {
  if (students.length === 0) {
    return (
      <div className="py-8 text-center">
        <div className="flex flex-col items-center gap-3">
          <MotoConceptIcon concept="children" size={40} />
          <div>
            <h3 className="text-sm font-medium text-gray-600">
              Keine Kinder in diesem Raum
            </h3>
            <p className="mt-1 text-xs text-gray-500">
              Es wurden noch keine Kinder eingecheckt
            </p>
          </div>
        </div>
      </div>
    );
  }

  if (filteredStudents.length === 0) {
    return (
      <EmptyStudentResults
        totalCount={students.length}
        filteredCount={filteredStudents.length}
      />
    );
  }

  return (
    <div>
      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
        {filteredStudents.map((student) => {
          const studentPickup = pickupTimesData?.get(student.id.toString());
          const studentArrival = arrivalTimesData?.get(student.id.toString());
          const presentStudent = withActiveSupervisionPresence(student);
          const arrivalExceptionAbsent =
            (studentArrival?.isException ?? false) &&
            !studentArrival?.expectedArrival;

          return (
            <StudentCard
              key={student.id}
              studentId={student.id}
              firstName={student.first_name}
              lastName={student.second_name}
              photoUrl={student.photo_url ?? null}
              onClick={() => onOpenStudent(student.id.toString())}
              locationBadge={
                <StudentPresenceBadge
                  student={presentStudent}
                  displayMode="contextAware"
                  userGroups={[...myGroupIds]}
                  groupRooms={[...myGroupRooms]}
                  variant="modern"
                  size="md"
                />
              }
              extraContent={
                <>
                  {student.school_class && (
                    <StudentInfoRow icon={<SchoolClassIcon />}>
                      {student.school_class}
                    </StudentInfoRow>
                  )}
                  {student.group_name && (
                    <StudentInfoRow icon={<GroupIcon />}>
                      Gruppe: {student.group_name}
                    </StudentInfoRow>
                  )}
                  {student.pending_excused_note !== undefined && (
                    <StudentPendingExcusedRow
                      note={student.pending_excused_note}
                    />
                  )}
                  {(() => {
                    const absence = getStudentAbsence({
                      sick: presentStudent.sick,
                      classTrip: presentStudent.class_trip,
                      excused: presentStudent.excused,
                    });
                    if (absence && !presentStudent.actual_pickup_time) {
                      return <StudentAbsenceRow label={absence.label} />;
                    }
                    const dayPlanningNotComingLabel =
                      getDayPlanningNotComingLabel(presentStudent);
                    if (
                      dayPlanningNotComingLabel &&
                      !presentStudent.actual_pickup_time
                    ) {
                      return (
                        <StudentAbsenceRow label={dayPlanningNotComingLabel} />
                      );
                    }
                    return (
                      <>
                        <ArrivalTimeRow
                          arrivalTime={studentArrival?.expectedArrival}
                          actualTime={student.actual_arrival_time}
                          isException={
                            !arrivalExceptionAbsent &&
                            (studentArrival?.isException ?? false)
                          }
                          isAbsent={false}
                          notes={
                            studentArrival && !arrivalExceptionAbsent
                              ? combineTimeNotes(
                                  studentArrival.notes,
                                  studentArrival.dayNotes,
                                )
                              : undefined
                          }
                          now={now}
                        />
                        <PickupTimeRow
                          pickupTime={studentPickup?.pickupTime}
                          actualTime={student.actual_pickup_time}
                          isException={studentPickup?.isException ?? false}
                          notes={
                            studentPickup
                              ? combineTimeNotes(
                                  studentPickup.notes,
                                  studentPickup.dayNotes,
                                )
                              : undefined
                          }
                          now={now}
                        />
                      </>
                    );
                  })()}
                </>
              }
              trackingIndicators={
                trackingData?.labels.length ? (
                  <TrackingIndicators
                    labels={trackingData.labels}
                    results={trackingData.results[student.id] ?? []}
                  />
                ) : undefined
              }
            />
          );
        })}
      </div>
    </div>
  );
}
