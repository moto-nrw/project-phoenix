"use client";

import { AlertCircle, CalendarClock, MoreVertical } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
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
import { ClassBulkArrivalModal } from "./class-bulk-arrival-modal";
import { ArrivalScheduleManager } from "./arrival-schedule-manager";
import { StudentAbholungTab } from "./student-abholung-tab";
import { StudentGuardiansTab } from "./student-guardians-tab";
import { StudentHistorieTab } from "./student-historie-tab";
import { StudentStammdatenTab } from "./student-stammdaten-tab";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { getInitials } from "~/lib/format-utils";
import type { Student } from "~/lib/api";

export type GroupingMode = "class" | "group" | "none";

interface StudentsMasterDetailProps {
  students: Student[];
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  grouping: GroupingMode;
  studentsWithArrival: Set<string>;
  arrivalSummaryById: Map<string, string>;
  onArrivalDataChanged: () => void;
  groups: Array<{ value: string; label: string }>;
  onUpdateStudent: (studentId: string, data: Partial<Student>) => Promise<void>;
  detailActions?: ReactNode;
}

const UNKNOWN_CLASS_LABEL = "Ohne Klasse";
const UNKNOWN_GROUP_LABEL = "Ohne Gruppe";

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
  selectedId,
  onSelect,
  grouping,
  studentsWithArrival,
  arrivalSummaryById,
  onArrivalDataChanged,
  groups,
  onUpdateStudent,
  detailActions,
}: StudentsMasterDetailProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [bulkClass, setBulkClass] = useState<string | null>(null);
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
        const value = student.group_name?.trim() || UNKNOWN_GROUP_LABEL;
        return { id: value, title: value };
      },
    }),
    [],
  );

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
      const bulkAction =
        grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL ? (
          <ClassActionsMenu
            schoolClass={group.id}
            onEditArrival={() => setBulkClass(group.id)}
          />
        ) : null;
      return { variant, countSuffix, bulkAction };
    },
    [grouping, studentsWithArrival],
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

  const studentsByClass = useMemo(() => {
    const map = new Map<string, Student[]>();
    for (const student of students) {
      const key = student.school_class?.trim();
      if (!key) continue;
      const list = map.get(key) ?? [];
      list.push(student);
      map.set(key, list);
    }
    return map;
  }, [students]);

  const handleBulkClose = useCallback(() => setBulkClass(null), []);
  const handleBulkSuccess = useCallback(() => {
    onArrivalDataChanged();
  }, [onArrivalDataChanged]);

  const renderItem = (student: Student) => {
    const id = keyForStudent(student);
    const hasArrival = studentsWithArrival.has(id);
    const arrivalSummary = arrivalSummaryById.get(id);

    const subtitleParts: string[] = [];
    if (student.group_name) subtitleParts.push(student.group_name);
    if (arrivalSummary) {
      subtitleParts.push(arrivalSummary);
    } else if (!hasArrival) {
      subtitleParts.push("keine Ankunft");
    }
    const subtitleText = subtitleParts.join(" · ") || "—";
    const subtitle = !hasArrival ? (
      <span className="font-medium text-[#F78C10]">{subtitleText}</span>
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
              className="h-4 w-4 shrink-0 text-[#F78C10]"
              aria-label="Ankunft fehlt"
            />
          ) : null
        }
      />
    );
  };

  const listNode = (
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
      {bulkClass ? (
        <ClassBulkArrivalModal
          isOpen={bulkClass !== null}
          onClose={handleBulkClose}
          schoolClass={bulkClass}
          studentsInClass={studentsByClass.get(bulkClass) ?? []}
          onSuccess={handleBulkSuccess}
        />
      ) : null}
    </>
  );
}

interface BuildTabsArgs {
  student: Student;
  groups: Array<{ value: string; label: string }>;
  onArrivalDataChanged: () => void;
  onUpdateStudent: (studentId: string, data: Partial<Student>) => Promise<void>;
}

function buildTabs({
  student,
  groups,
  onArrivalDataChanged,
  onUpdateStudent,
}: BuildTabsArgs): DetailTab[] {
  const studentId = student.id;
  return [
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
}

interface ClassActionsMenuProps {
  schoolClass: string;
  onEditArrival: () => void;
}

function ClassActionsMenu({
  schoolClass,
  onEditArrival,
}: ClassActionsMenuProps) {
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
        aria-label={`Aktionen für Klasse ${schoolClass}`}
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
            <CalendarClock className="h-4 w-4 text-gray-500" aria-hidden />
            Ankunftszeit bearbeiten
          </button>
        </div>
      ) : null}
    </div>
  );
}
