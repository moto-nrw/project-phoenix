"use client";

import { AlertCircle, CheckSquare, MoreVertical } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { DatabaseListItem } from "~/components/database/database-list-item";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import {
  useGroupedItems,
  type GroupDecorator,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";
import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";
import { SelectionBulkPickupModal } from "./selection-bulk-pickup-modal";
import { Button } from "~/components/ui/button";
import { ArrivalScheduleManager } from "./arrival-schedule-manager";
import { StudentAbholungTab } from "./student-abholung-tab";
import { StudentGuardiansTab } from "./student-guardians-tab";
import { StudentHistorieTab } from "./student-historie-tab";
import { StudentEnrollmentsTab } from "./student-enrollments-tab";
import { StudentStammdatenTab } from "./student-stammdaten-tab";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { getInitials } from "~/lib/format-utils";
import { hasPlannedCareExit } from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";
import type { Student } from "~/lib/api";
import type { BulkArrivalFilter } from "~/lib/student-arrival-api";

export type GroupingMode = "class" | "group" | "none";

interface StudentsMasterDetailProps {
  students: Student[];
  /** Unfiltered source used for bulk previews so search never narrows writes. */
  bulkStudents?: Student[];
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  grouping: GroupingMode;
  studentsWithArrival: Set<string>;
  arrivalSummaryById: Map<string, string>;
  onArrivalDataChanged: () => void;
  groups: Array<{ value: string; label: string }>;
  onUpdateStudent: (studentId: string, data: Partial<Student>) => Promise<void>;
  canViewEnrollments?: boolean;
  detailActions?: ReactNode;
  selectionMode?: boolean;
  selectedStudentIds?: ReadonlySet<string>;
  onToggleStudentSelection?: (studentId: string) => void;
  onClearSelection?: () => void;
  onFinishSelection?: () => void;
  /** Wählt genau die gerade angezeigten Kinder aus (#2487). */
  onSelectAllVisible?: (studentIds: string[]) => void;
  /** Öffnet "Betreuung beenden" für die Auswahl. Ohne die Berechtigung
   *  "Benutzer löschen" nicht gesetzt — dann fehlt der Knopf. */
  onEndCare?: () => void;
}

const UNKNOWN_CLASS_LABEL = "Ohne Klasse";
const UNKNOWN_GROUP_LABEL = "Ohne Gruppe";
const UNKNOWN_GROUP_ID = "__without_group__";
const EMPTY_STUDENT_SELECTION: ReadonlySet<string> = new Set();

interface BulkArrivalTarget {
  filter: BulkArrivalFilter;
  label: string;
}

function keyForStudent(student: Student): string {
  return String(student.id);
}

function formatStudentName(student: Student): string {
  if (student.first_name && student.second_name) {
    return `${student.first_name} ${student.second_name}`;
  }
  return student.name || "Unbekannt";
}

function buildHeaderSubtitle(student: Student): string {
  const parts: string[] = [];
  if (student.school_class) parts.push(student.school_class);
  if (student.group_name) parts.push(student.group_name);
  return parts.join(" · ") || "Keine Klasse hinterlegt";
}

export function StudentsMasterDetail({
  students,
  bulkStudents = students,
  selectedId,
  onSelect,
  grouping,
  studentsWithArrival,
  arrivalSummaryById,
  onArrivalDataChanged,
  groups,
  onUpdateStudent,
  canViewEnrollments = false,
  detailActions,
  selectionMode = false,
  selectedStudentIds = EMPTY_STUDENT_SELECTION,
  onToggleStudentSelection,
  onClearSelection,
  onFinishSelection,
  onSelectAllVisible,
  onEndCare,
}: StudentsMasterDetailProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [bulkTarget, setBulkTarget] = useState<BulkArrivalTarget | null>(null);
  const [classTripTarget, setClassTripTarget] = useState<{
    label: string;
    students: Student[];
  } | null>(null);
  const [pickupSelectionOpen, setPickupSelectionOpen] = useState(false);
  // Photo feature gate. When off, the detail header falls back to the
  // legacy light-blue initials chip (string avatar form) instead of the
  // shared <Avatar> — matches what the header looked like before the
  // feature shipped, so opt-out schools see no visual change.
  const { enabled: photosEnabled } = useStudentPhotosEnabled();

  const groupers = useMemo<
    Partial<Record<Exclude<GroupingMode, "none">, Grouper<Student>>>
  >(
    () => ({
      class: (student) => {
        const value = student.school_class?.trim() || UNKNOWN_CLASS_LABEL;
        return { id: value, title: value };
      },
      group: (student) => {
        const title = student.group_name?.trim() || UNKNOWN_GROUP_LABEL;
        return {
          id: student.group_id ? `group-${student.group_id}` : UNKNOWN_GROUP_ID,
          title,
          sortKey: title,
        };
      },
    }),
    [],
  );

  // Unfiltered cohort maps for bulk actions — search must not shrink class/group
  // writes while the UI still names the full bucket.
  const bulkStudentsByFilter = useMemo(() => {
    const byClass = new Map<string, Student[]>();
    const byGroup = new Map<string, Student[]>();
    for (const student of bulkStudents) {
      const schoolClass = student.school_class?.trim();
      if (schoolClass) {
        const classStudents = byClass.get(schoolClass) ?? [];
        classStudents.push(student);
        byClass.set(schoolClass, classStudents);
      }
      if (student.group_id) {
        const groupStudents = byGroup.get(student.group_id) ?? [];
        groupStudents.push(student);
        byGroup.set(student.group_id, groupStudents);
      }
    }
    return { byClass, byGroup };
  }, [bulkStudents]);

  const decorateGroup = useCallback<GroupDecorator<Student>>(
    (group) => {
      // Flat group ("none" mode) keeps default formatting; only per-class/group
      // buckets get the missing-arrival warning + bulk action.
      if (group.id === "__flat__") return {};

      const missing = group.items.filter(
        (item) => !studentsWithArrival.has(keyForStudent(item)),
      );
      const variant = missing.length > 0 ? "warning" : "neutral";
      const countSuffix =
        missing.length > 0 ? `· ${missing.length} offen` : undefined;
      const groupID = group.items[0]?.group_id;
      const arrivalTarget: BulkArrivalTarget | null =
        grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL
          ? {
              filter: { type: "school_class", schoolClass: group.id },
              label: group.title,
            }
          : grouping === "group" && groupID
            ? {
                filter: { type: "group", groupId: groupID },
                label: group.title,
              }
            : null;
      const canEditArrival = arrivalTarget !== null;
      const canPlanClassTrip =
        (grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL) ||
        (grouping === "group" && group.id !== UNKNOWN_GROUP_ID);
      const classTripStudents =
        grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL
          ? (bulkStudentsByFilter.byClass.get(group.id) ?? [])
          : grouping === "group" && groupID
            ? (bulkStudentsByFilter.byGroup.get(groupID) ?? [])
            : [];
      const bulkAction =
        canEditArrival || canPlanClassTrip ? (
          <GroupActionsMenu
            label={group.title}
            onEditArrival={
              arrivalTarget ? () => setBulkTarget(arrivalTarget) : undefined
            }
            onPlanClassTrip={
              canPlanClassTrip
                ? () =>
                    setClassTripTarget({
                      label: group.title,
                      students: classTripStudents,
                    })
                : undefined
            }
          />
        ) : null;
      return { variant, countSuffix, bulkAction };
    },
    [bulkStudentsByFilter, grouping, studentsWithArrival],
  );

  const groupDefinitions = useGroupedItems(
    students,
    grouping,
    groupers,
    "Kinder",
    decorateGroup,
  );

  const selectedStudent = useMemo(
    () =>
      selectedId
        ? (students.find((student) => String(student.id) === selectedId) ??
          null)
        : null,
    [selectedId, students],
  );

  const studentsForBulkTarget = useMemo(() => {
    if (!bulkTarget) return [];
    if (bulkTarget.filter.type === "school_class") {
      return (
        bulkStudentsByFilter.byClass.get(bulkTarget.filter.schoolClass) ?? []
      );
    }
    if (bulkTarget.filter.type === "group") {
      return bulkStudentsByFilter.byGroup.get(bulkTarget.filter.groupId) ?? [];
    }
    const ids = new Set(bulkTarget.filter.studentIds);
    return bulkStudents.filter((student) => ids.has(String(student.id)));
  }, [bulkStudents, bulkStudentsByFilter, bulkTarget]);

  const selectedStudents = useMemo(
    () =>
      bulkStudents.filter((student) =>
        selectedStudentIds.has(String(student.id)),
      ),
    [bulkStudents, selectedStudentIds],
  );

  const handleBulkClose = useCallback(() => setBulkTarget(null), []);
  const handleClassTripClose = useCallback(() => setClassTripTarget(null), []);
  const handleBulkSuccess = useCallback(() => {
    onArrivalDataChanged();
  }, [onArrivalDataChanged]);

  const renderItem = (student: Student) => {
    const id = keyForStudent(student);
    const hasArrival = studentsWithArrival.has(id);
    const arrivalSummary = arrivalSummaryById.get(id);

    const subtitleParts: string[] = [];
    if (student.group_name) subtitleParts.push(student.group_name);
    // Ein eingetragener Austritt steht vorne: er ändert, was mit dem Kind noch
    // geplant werden kann (#2487). Das bloße Ende der Anmeldephase steht hier
    // nicht — es hätte fast jedes Kind und wäre damit keine Nachricht mehr.
    if (hasPlannedCareExit(student) && student.care_ends_on) {
      subtitleParts.unshift(
        `Betreuung endet am ${formatDate(student.care_ends_on)}`,
      );
    }
    if (arrivalSummary) {
      subtitleParts.push(arrivalSummary);
    } else if (!hasArrival) {
      subtitleParts.push("keine Ankunft");
    }
    const subtitleText = subtitleParts.join(" · ") || "—";
    const subtitle = !hasArrival ? (
      <span className="text-moto-orange font-medium">{subtitleText}</span>
    ) : (
      subtitleText
    );

    return (
      <DatabaseListItem
        title={formatStudentName(student)}
        subtitle={subtitle}
        isSelected={selectedId === id}
        onSelect={() => onSelect(id)}
        trailingAccessory={
          !hasArrival ? (
            <AlertCircle
              className="text-moto-orange h-4 w-4 shrink-0"
              aria-label="Ankunft fehlt"
            />
          ) : null
        }
        selectionMode={selectionMode}
        isChecked={selectedStudentIds.has(id)}
        onToggleSelection={() => onToggleStudentSelection?.(id)}
      />
    );
  };

  const listNode = (
    <div className="flex min-h-0 flex-1 flex-col">
      {selectionMode ? (
        <SelectionBar
          count={selectedStudentIds.size}
          onClear={() => onClearSelection?.()}
          onFinish={() => onFinishSelection?.()}
          onArrival={() =>
            setBulkTarget({
              filter: {
                type: "students",
                studentIds: selectedStudents.map((student) =>
                  String(student.id),
                ),
              },
              label: "Auswahl",
            })
          }
          onPickup={() => setPickupSelectionOpen(true)}
          onClassTrip={() =>
            setClassTripTarget({ label: "Auswahl", students: selectedStudents })
          }
          onSelectAllVisible={
            onSelectAllVisible
              ? () => onSelectAllVisible(students.map(keyForStudent))
              : undefined
          }
          visibleCount={students.length}
          onEndCare={onEndCare}
        />
      ) : null}
      <GroupedList
        groups={groupDefinitions}
        renderItem={renderItem}
        keyFor={keyForStudent}
        emptyState={
          <div className="text-center text-sm text-gray-500">
            Keine Kinder gefunden.
          </div>
        }
      />
    </div>
  );

  const detailNode = selectedStudent ? (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          // Object form (photo + initials fallback via shared <Avatar>) when
          // the feature is on; legacy string form (light-blue initials chip)
          // when off — restoring the pre-feature appearance for opt-out
          // schools and keeping the master-detail consistent with the other
          // domain pages (groups, roles, permissions) that use the chip.
          // Both branches derive the initials from the same name source via
          // getInitials so chip-only schools see "F L" → "FL" identically
          // to the photo-feature schools' Avatar fallback.
          avatar={
            photosEnabled
              ? {
                  name: formatStudentName(selectedStudent),
                  imageUrl: selectedStudent.photo_url ?? null,
                }
              : getInitials(
                  `${selectedStudent.first_name ?? ""} ${selectedStudent.second_name ?? ""}`.trim() ||
                    "?",
                )
          }
          title={formatStudentName(selectedStudent)}
          subtitle={buildHeaderSubtitle(selectedStudent)}
          warning={
            studentsWithArrival.has(keyForStudent(selectedStudent))
              ? null
              : "Ankunft offen"
          }
          actions={detailActions}
        />
      }
      tabs={buildTabs({
        student: selectedStudent,
        groups,
        onArrivalDataChanged,
        onUpdateStudent,
        canViewEnrollments,
      })}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  ) : (
    <EmptyDetailState
      title="Kein Kind ausgewählt"
      description="Wähle links ein Kind, um Stammdaten und Ankunftszeiten zu bearbeiten."
    />
  );

  return (
    <>
      <MasterDetailLayout
        list={listNode}
        detail={detailNode}
        selectedId={selectedId}
        onDeselect={() => onSelect(null)}
        unselectedBehavior="expand"
        mobileDrawerTitle={
          selectedStudent ? formatStudentName(selectedStudent) : "Kinder"
        }
      />
      {bulkTarget ? (
        <FilteredBulkArrivalModal
          isOpen={true}
          onClose={handleBulkClose}
          filter={bulkTarget.filter}
          filterLabel={bulkTarget.label}
          studentsInFilter={studentsForBulkTarget}
          onSuccess={handleBulkSuccess}
        />
      ) : null}
      {classTripTarget ? (
        <ClassTripBulkStatusModal
          isOpen={true}
          onClose={handleClassTripClose}
          targetLabel={classTripTarget.label}
          students={classTripTarget.students}
          onSuccess={handleBulkSuccess}
        />
      ) : null}
      {pickupSelectionOpen ? (
        <SelectionBulkPickupModal
          isOpen
          onClose={() => setPickupSelectionOpen(false)}
          studentIds={selectedStudents.map((student) => String(student.id))}
          onSuccess={handleBulkSuccess}
        />
      ) : null}
    </>
  );
}

interface SelectionBarProps {
  count: number;
  onClear: () => void;
  onFinish: () => void;
  onArrival: () => void;
  onPickup: () => void;
  onClassTrip: () => void;
  /** Anzahl der gerade angezeigten Kinder — beziffert "Alle auswählen". */
  visibleCount?: number;
  onSelectAllVisible?: () => void;
  onEndCare?: () => void;
}

function SelectionBar({
  count,
  onClear,
  onFinish,
  onArrival,
  onPickup,
  onClassTrip,
  visibleCount = 0,
  onSelectAllVisible,
  onEndCare,
}: SelectionBarProps) {
  const disabled = count === 0;
  return (
    <div
      className="moto-content-surface border-moto-green/30 sticky top-0 z-20 shrink-0 border-x-0 border-b px-2 py-2 shadow-sm"
      role="region"
      aria-label="Auswahl"
    >
      <div className="flex flex-wrap items-center gap-2">
        <CheckSquare className="text-moto-green h-4 w-4 shrink-0" aria-hidden />
        <span className="min-w-0 text-sm font-semibold text-gray-900">
          {count} ausgewählt
        </span>
        <div className="ml-auto flex flex-wrap items-center gap-1.5">
          {onSelectAllVisible && visibleCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={onSelectAllVisible}
            >
              Alle {visibleCount} auswählen
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onClear}
            disabled={disabled}
          >
            Aufheben
          </Button>
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={onFinish}
          >
            Fertig
          </Button>
        </div>
      </div>
      <div className="mt-2 flex flex-wrap items-center gap-1.5">
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onArrival}
          disabled={disabled}
        >
          Ankunftszeiten
        </Button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onPickup}
          disabled={disabled}
        >
          Gehzeiten
        </Button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={onClassTrip}
          disabled={disabled}
        >
          Klassenfahrt
        </Button>
        {onEndCare ? (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={onEndCare}
            disabled={disabled}
          >
            Betreuung beenden
          </Button>
        ) : null}
      </div>
    </div>
  );
}

interface BuildTabsArgs {
  student: Student;
  groups: Array<{ value: string; label: string }>;
  onArrivalDataChanged: () => void;
  onUpdateStudent: (studentId: string, data: Partial<Student>) => Promise<void>;
  canViewEnrollments: boolean;
}

function buildTabs({
  student,
  groups,
  onArrivalDataChanged,
  onUpdateStudent,
  canViewEnrollments,
}: BuildTabsArgs): DetailTab[] {
  const studentId = student.id;
  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <StudentStammdatenTab
          student={student}
          groups={groups}
          onSave={(data) => onUpdateStudent(studentId, data)}
          // onArrivalDataChanged already mutates the database-students-list
          // SWR cache in the parent page; reusing it here means a photo
          // upload/delete refreshes the same list state, no extra plumbing.
          onStudentRefresh={onArrivalDataChanged}
        />
      ),
    },
    {
      id: "guardians",
      label: "Erziehungsberechtigte",
      content: (
        <StudentGuardiansTab
          student={student}
          onChanged={onArrivalDataChanged}
        />
      ),
    },
    {
      id: "betreuungszeiten",
      label: "Betreuungszeiten",
      content: (
        <div className="space-y-6">
          <ArrivalScheduleManager
            key={studentId}
            studentId={studentId}
            onUpdate={onArrivalDataChanged}
          />
          <StudentAbholungTab
            studentId={studentId}
            isSick={student.sick}
            isExcused={student.excused}
            onUpdate={onArrivalDataChanged}
          />
        </div>
      ),
    },
    {
      id: "history",
      label: "Historie",
      content: <StudentHistorieTab studentId={studentId} />,
    },
  ];
  if (canViewEnrollments) {
    tabs.push({
      id: "enrollments",
      label: "Anmeldungen",
      content: <StudentEnrollmentsTab studentId={studentId} />,
    });
  }
  return tabs;
}

interface GroupActionsMenuProps {
  label: string;
  onEditArrival?: () => void;
  onPlanClassTrip?: () => void;
}

function GroupActionsMenu({
  label,
  onEditArrival,
  onPlanClassTrip,
}: GroupActionsMenuProps) {
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  return (
    <div className="relative" ref={menuRef}>
      <button
        type="button"
        onClick={(event) => {
          event.stopPropagation();
          setOpen((prev) => !prev);
        }}
        aria-label={`Aktionen für ${label}`}
        aria-haspopup="menu"
        aria-expanded={open}
        className="moto-content-surface flex h-7 w-7 items-center justify-center rounded-md border text-gray-500 hover:bg-gray-50 hover:text-gray-700"
      >
        <MoreVertical className="h-4 w-4" aria-hidden />
      </button>
      {open ? (
        <div
          role="menu"
          className="moto-content-surface absolute top-full right-0 z-50 mt-1 w-56 overflow-hidden rounded-lg border py-1 shadow-lg"
        >
          {onEditArrival ? (
            <button
              type="button"
              role="menuitem"
              onClick={(event) => {
                event.stopPropagation();
                setOpen(false);
                onEditArrival();
              }}
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
            >
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.carePlan.icon}
                tone={MOTO_CONCEPTS.carePlan.tone}
                size={18}
              />
              Ankunftszeit bearbeiten
            </button>
          ) : null}
          {onPlanClassTrip ? (
            <button
              type="button"
              role="menuitem"
              onClick={(event) => {
                event.stopPropagation();
                setOpen(false);
                onPlanClassTrip();
              }}
              className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
            >
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.classTrip.icon}
                tone={MOTO_CONCEPTS.classTrip.tone}
                size={18}
              />
              Klassenfahrt planen
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
