"use client";

import { AlertCircle, CheckSquare } from "lucide-react";
import { useCallback, useMemo, useState } from "react";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DatabaseListLayout } from "~/components/database/database-list-layout";
import { GroupedList } from "~/components/database/grouped-list";
import {
  useGroupedItems,
  type GroupDecorator,
  type Grouper,
} from "~/components/database/use-grouped-items";
import { Button } from "~/components/ui/button";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { hasPlannedCareExit } from "~/lib/care-exit-api";
import { formatDate } from "~/lib/date-helpers";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type { Student } from "~/lib/api";
import type { BulkArrivalFilter } from "~/lib/student-arrival-api";
import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";
import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";
import { SelectionBulkPickupModal } from "./selection-bulk-pickup-modal";

export type GroupingMode = "class" | "group" | "none";

/**
 * Die Sammlung der Kinderdaten (BAUARTEN-SPEC Bauart 1): gruppierte Liste,
 * Sammelaktionen je Klasse oder Gruppe und die Mehrfachauswahl. Jede Zeile
 * führt auf die Kindakte `/students/[id]` — die einzige Objektansicht des
 * Kindes (Bauart 2, #3115). Was das einzelne Kind betrifft (Stammdaten,
 * Betreuung beenden, Löschen), liegt dort und nicht mehr in einem Pane.
 */
interface StudentsListProps {
  students: Student[];
  /** Ungefilterte Quelle für Sammelaktionen: die Suche darf keine Schreibvorgänge verengen. */
  bulkStudents?: Student[];
  grouping: GroupingMode;
  studentsWithArrival: Set<string>;
  arrivalSummaryById: Map<string, string>;
  onArrivalDataChanged: () => void;
  /** Ziel der Kindakte für eine Zeile, samt Rückweg (`?from=`). */
  objectHref: (student: Student) => string;
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

export function StudentsList({
  students,
  bulkStudents = students,
  grouping,
  studentsWithArrival,
  arrivalSummaryById,
  onArrivalDataChanged,
  objectHref,
  selectionMode = false,
  selectedStudentIds = EMPTY_STUDENT_SELECTION,
  onToggleStudentSelection,
  onClearSelection,
  onFinishSelection,
  onSelectAllVisible,
  onEndCare,
}: StudentsListProps) {
  const [bulkTarget, setBulkTarget] = useState<BulkArrivalTarget | null>(null);
  const [classTripTarget, setClassTripTarget] = useState<{
    label: string;
    students: Student[];
  } | null>(null);
  const [pickupSelectionOpen, setPickupSelectionOpen] = useState(false);

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

  // Ungefilterte Kohorten für die Sammelaktionen: die Suche verengt keine
  // Klassen- oder Gruppenschreibvorgänge, solange die Fläche den ganzen
  // Verband benennt.
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
      // Die flache Liste behält ihre Vorgabe; nur Klassen- und Gruppenblöcke
      // tragen die Warnung für fehlende Ankünfte und die Sammelaktion.
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
      const canPlanClassTrip =
        (grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL) ||
        (grouping === "group" && group.id !== UNKNOWN_GROUP_ID);
      const classTripStudents =
        grouping === "class" && group.id !== UNKNOWN_CLASS_LABEL
          ? (bulkStudentsByFilter.byClass.get(group.id) ?? [])
          : grouping === "group" && groupID
            ? (bulkStudentsByFilter.byGroup.get(groupID) ?? [])
            : [];
      const items = [
        ...(arrivalTarget
          ? [
              {
                label: "Ankunftszeit bearbeiten",
                icon: (
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.carePlan.icon}
                    tone={MOTO_CONCEPTS.carePlan.tone}
                    size={18}
                  />
                ),
                onClick: () => setBulkTarget(arrivalTarget),
              },
            ]
          : []),
        ...(canPlanClassTrip
          ? [
              {
                label: "Klassenfahrt planen",
                icon: (
                  <MotoDuotoneIcon
                    icon={MOTO_CONCEPTS.classTrip.icon}
                    tone={MOTO_CONCEPTS.classTrip.tone}
                    size={18}
                  />
                ),
                onClick: () =>
                  setClassTripTarget({
                    label: group.title,
                    students: classTripStudents,
                  }),
              },
            ]
          : []),
      ];
      const bulkAction =
        items.length > 0 ? (
          <OverflowMenu
            ariaLabel={`Aktionen für ${group.title}`}
            items={items}
            triggerSize="sm"
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
    // Kein Gedankenstrich als Platzhalter: eine Zeile, die nur „–" sagt,
    // sagt nichts. Gibt es nichts zu ergänzen, entfällt die Zeile.
    const subtitleText = subtitleParts.join(" · ");
    const subtitle = !subtitleText ? undefined : !hasArrival ? (
      <span className="text-moto-orange font-medium">{subtitleText}</span>
    ) : (
      subtitleText
    );

    return (
      <DatabaseListItem
        title={formatStudentName(student)}
        subtitle={subtitle}
        isSelected={false}
        href={objectHref(student)}
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

  return (
    <>
      <DatabaseListLayout>
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
              setClassTripTarget({
                label: "Auswahl",
                students: selectedStudents,
              })
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
      </DatabaseListLayout>
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
