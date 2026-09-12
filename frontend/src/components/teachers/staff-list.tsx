"use client";

import { DatabaseListItem } from "~/components/database/database-list-item";
import { DatabaseListLayout } from "~/components/database/database-list-layout";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import type { Teacher } from "@/lib/teacher-api";

interface StaffListProps {
  groupDefinitions: GroupDefinition<Teacher>[];
  /** Ziel der Personalakte für eine Zeile, samt Rückweg (`?from=`). */
  objectHref: (teacher: Teacher) => string;
}

function keyForTeacher(teacher: Teacher): string {
  return teacher.id;
}

function getStaffDisplayName(teacher: Teacher): string {
  return (
    teacher.name ||
    `${teacher.first_name ?? ""} ${teacher.last_name ?? ""}`.trim() ||
    "Unbekannt"
  );
}

function buildSubtitle(teacher: Teacher): string {
  const displayRole = teacher.account_role
    ? getRoleDisplayName(teacher.account_role)
    : null;
  const position =
    teacher.role &&
    teacher.role.toLowerCase() !== displayRole?.toLowerCase() &&
    teacher.role.toLowerCase() !== teacher.account_role?.toLowerCase()
      ? teacher.role
      : null;
  const roleLine = [displayRole, position].filter(Boolean).join(" · ");
  if (roleLine) return roleLine;
  if (teacher.email) return teacher.email;
  return "–";
}

/**
 * Die Sammlung des Personals in der Datenverwaltung (BAUARTEN-SPEC Bauart 1):
 * gruppierte Liste, jede Zeile ein Link auf die Personalakte `/staff/[id]`,
 * die einzige Objektansicht einer Person (#3115).
 */
export function StaffList({ groupDefinitions, objectHref }: StaffListProps) {
  return (
    <DatabaseListLayout>
      <GroupedList
        groups={groupDefinitions}
        renderItem={(teacher) => (
          <DatabaseListItem
            title={getStaffDisplayName(teacher)}
            subtitle={buildSubtitle(teacher)}
            isSelected={false}
            href={objectHref(teacher)}
          />
        )}
        keyFor={keyForTeacher}
        emptyState={
          <div className="text-center text-sm text-gray-500">
            Kein Personal gefunden.
          </div>
        }
      />
    </DatabaseListLayout>
  );
}
