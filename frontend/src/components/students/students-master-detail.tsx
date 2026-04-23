"use client";

import { CalendarClock } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { ClassBulkArrivalModal } from "./class-bulk-arrival-modal";
import { StudentAbholungTab } from "./student-abholung-tab";
import { StudentArrivalEditor } from "./student-arrival-editor";
import { StudentAusnahmenTab } from "./student-ausnahmen-tab";
import { StudentDetailHeader } from "./student-detail-header";
import { StudentGuardiansTab } from "./student-guardians-tab";
import { StudentHistorieTab } from "./student-historie-tab";
import { StudentListItem } from "./student-list-item";
import { StudentStammdatenTab } from "./student-stammdaten-tab";
import type { GroupingMode } from "./students-grouping-toggle";
import type { Student } from "~/lib/api";

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

const UNGROUPED_KEY = "__none__";
const UNKNOWN_CLASS_LABEL = "Ohne Klasse";
const UNKNOWN_GROUP_LABEL = "Ohne Gruppe";

function keyForStudent(student: Student): string {
  return String(student.id);
}

function groupValueFor(student: Student, mode: GroupingMode): string {
  if (mode === "none") return UNGROUPED_KEY;
  if (mode === "class") {
    return student.school_class?.trim() || UNKNOWN_CLASS_LABEL;
  }
  return student.group_name?.trim() || UNKNOWN_GROUP_LABEL;
}

function sortGroupIds(ids: string[]): string[] {
  return [...ids].sort((a, b) => a.localeCompare(b, "de"));
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

  useEffect(() => {
    if (selectedId) {
      setActiveTab("master-data");
    }
  }, [selectedId]);

  const groupDefinitions: GroupDefinition<Student>[] = useMemo(() => {
    const buckets = new Map<string, Student[]>();
    for (const student of students) {
      const key = groupValueFor(student, grouping);
      const list = buckets.get(key) ?? [];
      list.push(student);
      buckets.set(key, list);
    }

    if (grouping === "none") {
      const flat = buckets.get(UNGROUPED_KEY) ?? [];
      if (flat.length === 0) return [];
      return [
        {
          id: UNGROUPED_KEY,
          title: `Alle Schüler (${flat.length})`,
          items: flat,
        },
      ];
    }

    const sortedIds = sortGroupIds(Array.from(buckets.keys()));
    return sortedIds.map<GroupDefinition<Student>>((id) => {
      const items = buckets.get(id) ?? [];
      const missing = items.filter(
        (item) => !studentsWithArrival.has(keyForStudent(item)),
      );
      const variant = missing.length > 0 ? "warning" : "neutral";
      const countSuffix =
        missing.length > 0 ? `· ${missing.length} offen` : undefined;
      const bulkAction =
        grouping === "class" && id !== UNKNOWN_CLASS_LABEL ? (
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation();
              setBulkClass(id);
            }}
            aria-label={`Ankunftszeiten für Klasse ${id} setzen`}
            className={
              variant === "warning"
                ? "flex items-center gap-1.5 rounded-md bg-[#F78C10] px-2 py-1 text-xs font-semibold text-white hover:bg-[#E37C00]"
                : "flex items-center gap-1.5 rounded-md border border-gray-300 bg-white px-2 py-1 text-xs font-semibold text-gray-700 hover:bg-gray-50"
            }
          >
            <CalendarClock className="h-3 w-3" aria-hidden />
            Bulk
          </button>
        ) : null;
      return {
        id,
        title:
          grouping === "class" && id !== UNKNOWN_CLASS_LABEL
            ? `Klasse ${id}`
            : id,
        items,
        variant,
        countSuffix,
        bulkAction,
      };
    });
  }, [grouping, students, studentsWithArrival]);

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
    return (
      <StudentListItem
        student={student}
        isSelected={selectedId === id}
        onSelect={() => onSelect(id)}
        hasArrival={studentsWithArrival.has(id)}
        arrivalSummary={arrivalSummaryById.get(id)}
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
          Keine Schüler gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedStudent ? (
    <DetailPanel
      header={
        <StudentDetailHeader
          student={selectedStudent}
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
      title="Keinen Schüler ausgewählt"
      description="Wähle links einen Schüler, um Stammdaten und Ankunftszeiten zu bearbeiten."
    />
  );

  return (
    <>
      <MasterDetailLayout
        list={listNode}
        detail={detailNode}
        selectedId={selectedId}
        onDeselect={() => onSelect(null)}
        mobileDrawerTitle={
          selectedStudent
            ? `${selectedStudent.second_name ?? ""} ${selectedStudent.first_name ?? ""}`.trim() ||
              "Schüler"
            : "Schüler"
        }
      />
      {bulkClass ? (
        <ClassBulkArrivalModal
          isOpen={bulkClass !== null}
          onClose={handleBulkClose}
          schoolClass={bulkClass}
          studentsInClass={studentsByClass.get(bulkClass) ?? []}
          studentsWithArrival={studentsWithArrival}
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
      id: "arrival",
      label: "Ankunft",
      content: (
        <StudentArrivalEditor
          studentId={studentId}
          onSaved={onArrivalDataChanged}
        />
      ),
    },
    {
      id: "pickup",
      label: "Abholung",
      content: (
        <StudentAbholungTab
          studentId={studentId}
          isSick={student.sick}
          isExcused={student.excused}
          onUpdate={onArrivalDataChanged}
        />
      ),
    },
    {
      id: "exceptions",
      label: "Ausnahmen",
      content: (
        <StudentAusnahmenTab
          studentId={studentId}
          onChanged={onArrivalDataChanged}
        />
      ),
    },
    {
      id: "history",
      label: "Historie",
      content: <StudentHistorieTab studentId={studentId} />,
    },
  ];
}
