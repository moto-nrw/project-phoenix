"use client";

import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import { EmptyState } from "~/components/ui/empty-state";
import { SectionCard } from "~/components/ui/section-card";
import {
  StudentCard,
  StudentInfoRow,
  SchoolClassIcon,
  GroupIcon,
  ActivityIcon,
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
import {
  StudentTable,
  arrivalColumn,
  classColumn,
  compactColumns,
  groupColumn,
  nameColumn,
  pickupColumn,
  statusColumn,
  trackingColumn,
  type StudentTableColumn,
  type StudentTableDay,
} from "~/components/students/student-table";

/**
 * Arrival, pickup or absence of one visitor for the table view (#3834). Same
 * branches as the card below.
 */
function supervisionStudentDay(
  student: ActiveSupervisionStudent,
  pickupTimesData: ReadonlyMap<string, BulkPickupTime> | undefined,
  arrivalTimesData: ReadonlyMap<string, BulkArrivalTime> | undefined,
  now: Date,
): StudentTableDay {
  const studentPickup = pickupTimesData?.get(student.id.toString());
  const studentArrival = arrivalTimesData?.get(student.id.toString());
  const presentStudent = withActiveSupervisionPresence(student);
  const arrivalExceptionAbsent =
    (studentArrival?.isException ?? false) && !studentArrival?.expectedArrival;
  const pickupNote = studentPickup
    ? combineTimeNotes(studentPickup.notes, studentPickup.dayNotes)
    : undefined;
  const result = {
    arrival: {
      arrivalTime: studentArrival?.expectedArrival,
      actualTime: student.actual_arrival_time,
      isException:
        !arrivalExceptionAbsent && (studentArrival?.isException ?? false),
      isAbsent: false,
      notes:
        studentArrival && !arrivalExceptionAbsent
          ? combineTimeNotes(studentArrival.notes, studentArrival.dayNotes)
          : undefined,
      now,
    },
    pickup: {
      pickupTime: studentPickup?.pickupTime,
      actualTime: student.actual_pickup_time,
      isException: studentPickup?.isException ?? false,
      notes: pickupNote,
      now,
    },
  };
  if (presentStudent.actual_pickup_time) return result;
  const absence = getStudentAbsence({
    sick: presentStudent.sick,
    classTrip: presentStudent.class_trip,
    excused: presentStudent.excused,
  });
  if (absence) return { ...result, absence: { label: absence.label } };
  const notComing = getDayPlanningNotComingLabel(presentStudent);
  return notComing
    ? { ...result, absence: { label: notComing, note: pickupNote } }
    : result;
}

/** The table columns of the visitor list (#3834): what the card shows. */
export function buildSupervisionTableColumns({
  pickupTimesData,
  arrivalTimesData,
  trackingData,
  myGroupIds,
  myGroupRooms,
  now,
  photosEnabled,
  hrefFor,
}: Readonly<{
  pickupTimesData: ReadonlyMap<string, BulkPickupTime> | undefined;
  arrivalTimesData: ReadonlyMap<string, BulkArrivalTime> | undefined;
  trackingData: TrackingIndicatorsResponse | undefined;
  myGroupIds: readonly string[];
  myGroupRooms: readonly string[];
  now: Date;
  photosEnabled: boolean;
  hrefFor: (student: ActiveSupervisionStudent) => string;
}>): StudentTableColumn<ActiveSupervisionStudent>[] {
  const day = (student: ActiveSupervisionStudent) =>
    supervisionStudentDay(student, pickupTimesData, arrivalTimesData, now);
  return compactColumns<ActiveSupervisionStudent>([
    nameColumn(hrefFor, photosEnabled),
    statusColumn("Aufenthalt", (student) => (
      <StudentPresenceBadge
        student={withActiveSupervisionPresence(student)}
        displayMode="contextAware"
        userGroups={[...myGroupIds]}
        groupRooms={[...myGroupRooms]}
        variant="modern"
        size="md"
      />
    )),
    classColumn(),
    groupColumn(),
    {
      key: "activity",
      header: "Angebot",
      render: (student) =>
        student.activity_name ??
        (student.independent ? (
          "Ohne Angebot"
        ) : (
          <span className="text-gray-400">–</span>
        )),
      sortValue: (student) => student.activity_name ?? "",
    },
    arrivalColumn(day),
    pickupColumn(day),
    trackingColumn(trackingData),
  ]);
}

/** Table mode of the visitor list; omit for the tiles. */
interface SupervisionStudentTable {
  readonly columns: StudentTableColumn<ActiveSupervisionStudent>[];
  readonly hiddenColumns: ReadonlySet<string>;
  readonly phoneDetail: string | null;
  readonly selection: {
    readonly selectedIds: ReadonlySet<string>;
    readonly onChange: (ids: readonly string[], selected: boolean) => void;
    readonly disabled?: boolean;
  };
}

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
  readonly table?: SupervisionStudentTable | null;
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
  table,
}: SupervisionStudentGridProps) {
  // Leer- und Filterzustand sitzen auf derselben Fläche wie die Karten, die
  // sie ersetzen: freier Text auf dem gemusterten Grund liest sich als Lücke,
  // nicht als Zustand.
  if (students.length === 0) {
    return (
      <SectionCard>
        <EmptyState
          icon={<MotoConceptIcon concept="children" size={40} />}
          title="Keine Kinder in diesem Raum"
          description="Es wurden noch keine Kinder eingecheckt."
        />
      </SectionCard>
    );
  }

  if (filteredStudents.length === 0) {
    return (
      <SectionCard>
        <EmptyStudentResults
          totalCount={students.length}
          filteredCount={filteredStudents.length}
        />
      </SectionCard>
    );
  }

  if (table) {
    return (
      <StudentTable
        rows={filteredStudents}
        columns={table.columns}
        hiddenColumns={table.hiddenColumns}
        phoneDetail={table.phoneDetail}
        onOpen={(student) => onOpenStudent(student.id.toString())}
        selection={table.selection}
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
                  {student.activity_name && (
                    <StudentInfoRow icon={<ActivityIcon />}>
                      Angebot: {student.activity_name}
                    </StudentInfoRow>
                  )}
                  {student.independent && !student.activity_name && (
                    <StudentInfoRow icon={<ActivityIcon />}>
                      Ohne Angebot
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
                        <StudentAbsenceRow
                          label={dayPlanningNotComingLabel}
                          note={combineTimeNotes(
                            studentPickup?.notes,
                            studentPickup?.dayNotes,
                          )}
                        />
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
