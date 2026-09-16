"use client";

import { useState, type ComponentProps } from "react";
import { UserPlus } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import {
  SectionCard,
  SectionCollapseToggle,
} from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { ActiveSupervisionLoadingView } from "~/components/active-supervisions/states";
import { CompleteInstanceModal } from "~/components/active-supervisions/complete-instance-modal";
import { SupervisionStudentGrid } from "~/components/active-supervisions/student-grid";
import { TimetableRosterContent } from "~/components/active-supervisions/timetable-roster";
import { useTimetableActions } from "~/components/active-supervisions/use-timetable-actions";
import { useTimetableRoster } from "~/components/active-supervisions/use-timetable-roster";
import type {
  ActiveSupervisionStudent,
  OpenRoomBlockSection,
  OpenRoomOccupancySection,
  OpenRoomSection,
} from "~/components/active-supervisions/view-model";

type TimetableActionsOptions = Parameters<typeof useTimetableActions>[0];

/**
 * What every block roster on the page shares: the page's mutation plumbing
 * plus the rights that do not differ per block.
 */
export type OpenRoomBlockContext = Omit<
  TimetableActionsOptions,
  "activeTimetableInstanceId" | "currentTimetableRoster" | "mutateRoster"
> & {
  readonly attendanceWebEnabled: boolean;
  readonly showTimetableCounts: boolean;
  /** Offers „Rest des Tages“ when the caller may record partial absences. */
  readonly canExcuseRestOfDay: boolean;
  /** The school-wide overview lets the caller read every running roster. */
  readonly overviewEnabled: boolean;
  readonly onAddSupervisor: (activeGroupId: string) => void;
};

type StudentGridProps = Omit<
  ComponentProps<typeof SupervisionStudentGrid>,
  "students" | "filteredStudents"
>;

interface OpenRoomSectionsProps {
  readonly sections: readonly OpenRoomSection[];
  /** Every child in the room, before and after the page's search/filters. */
  readonly students: readonly ActiveSupervisionStudent[];
  readonly filteredStudents: readonly ActiveSupervisionStudent[];
  readonly grid: StudentGridProps;
  readonly blocks: OpenRoomBlockContext;
}

/**
 * A released room with running blocks (ADR 0019, point 3): one section per
 * block with its roster, the caller's own blocks first and open, then the
 * children outside every block. Each section is a head card with the content
 * cards below it, like the roster of an own session.
 */
export function OpenRoomSections({
  sections,
  students,
  filteredStudents,
  grid,
  blocks,
}: OpenRoomSectionsProps) {
  return (
    <div className="space-y-6">
      {sections.map((section) => {
        const ids = sectionSessionIds(section);
        const children = {
          students: students.filter((s) => ids.includes(s.activeGroupId)),
          filteredStudents: filteredStudents.filter((s) =>
            ids.includes(s.activeGroupId),
          ),
          grid,
        };
        return section.kind === "block" ? (
          <OpenRoomBlock
            key={section.key}
            section={section}
            context={blocks}
            {...children}
          />
        ) : (
          <OpenRoomOccupancy
            key={section.key}
            section={section}
            onAddSupervisor={blocks.onAddSupervisor}
            {...children}
          />
        );
      })}
    </div>
  );
}

function sectionSessionIds(section: OpenRoomSection): readonly string[] {
  return section.kind === "block"
    ? [section.session.activeGroupId]
    : section.activeGroupIds;
}

function childCountLabel(count: number): string {
  return `${count} ${count === 1 ? "Kind" : "Kinder"}`;
}

interface SectionChildren {
  readonly students: readonly ActiveSupervisionStudent[];
  readonly filteredStudents: readonly ActiveSupervisionStudent[];
  readonly grid: StudentGridProps;
}

/** The section's children as cards; nothing when none are present. */
function SectionStudents({
  students,
  filteredStudents,
  grid,
}: SectionChildren) {
  if (students.length === 0) return null;
  return (
    <SupervisionStudentGrid
      students={students}
      filteredStudents={filteredStudents}
      {...grid}
    />
  );
}

function AddSupervisorButton({
  activeGroupId,
  onAddSupervisor,
}: Readonly<{
  activeGroupId: string | null;
  onAddSupervisor: (activeGroupId: string) => void;
}>) {
  if (!activeGroupId) return null;
  return (
    <Button
      type="button"
      variant="outline"
      size="md"
      className="bg-white"
      onClick={() => onAddSupervisor(activeGroupId)}
    >
      <UserPlus className="h-4 w-4" aria-hidden="true" />
      Betreuer hinzufügen
    </Button>
  );
}

function OpenRoomBlock({
  section,
  context,
  students,
  filteredStudents,
  grid,
}: Readonly<
  SectionChildren & {
    section: OpenRoomBlockSection;
    context: OpenRoomBlockContext;
  }
>) {
  const { session, block } = section;
  const {
    attendanceWebEnabled,
    showTimetableCounts,
    canExcuseRestOfDay,
    overviewEnabled,
    onAddSupervisor,
    ...actionOptions
  } = context;
  const [collapsed, setCollapsed] = useState(!section.isOwn);
  // Reading a foreign block's roster needs the school-wide overview (#2380).
  // Without it the section shows the block's children from the room.
  const canViewRoster = block.canOperate || overviewEnabled;
  const roster = useTimetableRoster({
    selectedTimetableInstanceId:
      !collapsed && canViewRoster ? block.instanceId : null,
    currentRoomId: undefined,
  });
  const actions = useTimetableActions({
    ...actionOptions,
    activeTimetableInstanceId: roster.activeTimetableInstanceId,
    currentTimetableRoster: roster.currentTimetableRoster,
    mutateRoster: roster.mutateRoster,
  });

  const title = session.title;
  const addSupervisor = (
    <AddSupervisorButton
      activeGroupId={section.assignableSessionId}
      onAddSupervisor={onAddSupervisor}
    />
  );
  const header = (
    <SectionCard
      collapsible
      collapsed={collapsed}
      onCollapsedChange={setCollapsed}
      title={title}
      titleBadge={
        <>
          {section.isOwn ? (
            <StatusBadge
              label={
                session.isUserSupervising ? "Eigene Aufsicht" : "Eingeplant"
              }
              tone="green"
            />
          ) : null}
          <StatusBadge
            label={childCountLabel(session.studentCount)}
            tone="gray"
          />
        </>
      }
      description={
        section.isOwn
          ? `${block.startTime}–${block.endTime} Uhr`
          : `${block.startTime}–${block.endTime} Uhr · Sie sind hier nicht eingeplant.`
      }
      actions={section.assignableSessionId ? addSupervisor : undefined}
    />
  );

  if (collapsed) return header;

  const currentRoster = roster.currentTimetableRoster;
  if (currentRoster) {
    return (
      <div role="group" aria-label={title} className="space-y-4">
        {actions.moveNotice ? (
          <Alert type="info" message={actions.moveNotice} />
        ) : null}
        <TimetableRosterContent
          addStudentResults={actions.addStudentResults}
          addStudentSearch={actions.addStudentSearch}
          addStudentError={actions.addStudentError}
          attendanceWebEnabled={attendanceWebEnabled}
          isAddingStudent={actions.isAddingStudent}
          isCompletingInstance={actions.isCompletingInstance}
          isConfirmingExpected={actions.isConfirmingExpected}
          roster={currentRoster}
          showTimetableCounts={showTimetableCounts}
          headerActions={section.assignableSessionId ? addSupervisor : null}
          headerToggle={
            <SectionCollapseToggle
              title={title}
              collapsed={false}
              onToggle={() => setCollapsed(true)}
            />
          }
          onAddStudent={actions.handleAddUnplannedStudent}
          onComplete={actions.handleCompleteTimetableInstance}
          onConfirmExpected={actions.handleConfirmExpectedStudents}
          onRosterAction={actions.handleRosterAction}
          onExcuseRestOfDay={
            canExcuseRestOfDay ? actions.handleExcuseRestOfDay : undefined
          }
          onSearchChange={actions.handleAddStudentSearchChange}
        />
        <CompleteInstanceModal
          isOpen={actions.showCompleteConfirmation}
          roster={currentRoster}
          isCompleting={actions.isCompletingInstance}
          onClose={() => actions.setShowCompleteConfirmation(false)}
          onConfirm={() => void actions.confirmCompleteTimetableInstance()}
        />
      </div>
    );
  }

  const isLoading = canViewRoster && !roster.hasRosterError;
  return (
    <div role="group" aria-label={title} className="space-y-4">
      {header}
      {isLoading ? (
        <ActiveSupervisionLoadingView withHeader={false} />
      ) : (
        <SectionStudents
          students={students}
          filteredStudents={filteredStudents}
          grid={grid}
        />
      )}
    </div>
  );
}

function OpenRoomOccupancy({
  section,
  onAddSupervisor,
  students,
  filteredStudents,
  grid,
}: Readonly<
  SectionChildren & {
    section: OpenRoomOccupancySection;
    onAddSupervisor: (activeGroupId: string) => void;
  }
>) {
  // Children outside every block are the room's shared occupancy: they stay
  // open. A foreign kiosk session starts collapsed like a foreign block.
  const [collapsed, setCollapsed] = useState(
    !section.independent && !section.isOwn,
  );
  return (
    <div className="space-y-4">
      <SectionCard
        collapsible
        collapsed={collapsed}
        onCollapsedChange={setCollapsed}
        title={section.independent ? "Ohne Angebot" : section.title}
        titleBadge={
          <StatusBadge
            label={childCountLabel(section.studentCount)}
            tone="gray"
          />
        }
        description={
          section.independent
            ? "Diese Kinder nutzen nur den Raum."
            : "Diese Aktivität läuft ohne Block aus dem Betreuungsplan."
        }
        actions={
          section.assignableSessionId ? (
            <AddSupervisorButton
              activeGroupId={section.assignableSessionId}
              onAddSupervisor={onAddSupervisor}
            />
          ) : undefined
        }
      />
      {collapsed ? null : (
        <SectionStudents
          students={students}
          filteredStudents={filteredStudents}
          grid={grid}
        />
      )}
    </div>
  );
}
