"use client";

import { DatabaseListItem } from "~/components/database/database-list-item";
import { DatabaseListLayout } from "~/components/database/database-list-layout";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { ExternalAccountBadge } from "~/components/staff/external-badge";
import { Avatar } from "~/components/ui/avatar";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { staffAvatarUrl } from "~/lib/staff-api";
import { externalStaffLabel, staffDisplayName } from "~/lib/staff-helpers";
import type { Teacher } from "@/lib/teacher-api";

interface StaffListProps {
  groupDefinitions: GroupDefinition<Teacher>[];
  /** Ziel der Personalakte für eine Zeile, samt Rückweg (`?from=`). */
  objectHref: (teacher: Teacher) => string;
  /**
   * „Konto einladen“ im Kebab einer externen Person ohne Konto (#4573).
   * Fehlt die Funktion, darf die Person niemanden einladen.
   */
  onInviteExternal?: (teacher: Teacher) => void;
}

function keyForTeacher(teacher: Teacher): string {
  return teacher.id;
}

function getStaffDisplayName(teacher: Teacher): string {
  return (
    staffDisplayName(teacher.name, teacher.first_name, teacher.last_name) ||
    "Unbekannt"
  );
}

function buildSubtitle(teacher: Teacher): string {
  if (teacher.is_external) {
    // Der Kontostand steht als eigene Marke am Zeilenende (#4573).
    return externalStaffLabel(teacher.external_organization);
  }
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
/** Kebab-Eintrag für eine externe Person ohne Konto, sonst nichts (#4573). */
function inviteActionLabel(teacher: Teacher): string | null {
  if (!teacher.is_external) return null;
  if (teacher.external_account_status === "none") return "Konto einladen";
  if (teacher.external_account_status === "invitation_pending")
    return "Einladung erneut senden";
  return null;
}

export function StaffList({
  groupDefinitions,
  objectHref,
  onInviteExternal,
}: StaffListProps) {
  return (
    <DatabaseListLayout>
      <GroupedList
        groups={groupDefinitions}
        renderItem={(teacher) => {
          const displayName = getStaffDisplayName(teacher);
          const inviteLabel = onInviteExternal
            ? inviteActionLabel(teacher)
            : null;
          const row = (
            <DatabaseListItem
              // Profilbild, ohne Bild die Initialen (#4432). Der Name steht
              // daneben, der Avatar ist also rein schmückend.
              leadingAccessory={
                <Avatar
                  imageUrl={staffAvatarUrl(teacher.id, teacher.avatar)}
                  name={displayName}
                  size="sm"
                  decorative
                />
              }
              title={displayName}
              subtitle={buildSubtitle(teacher)}
              trailingAccessory={
                teacher.is_external ? (
                  <ExternalAccountBadge
                    status={teacher.external_account_status}
                  />
                ) : undefined
              }
              isSelected={false}
              href={objectHref(teacher)}
            />
          );
          if (!inviteLabel || !onInviteExternal) return row;
          // Die Zeile bleibt der Link zur Personalakte; der Kebab steht
          // daneben, nicht im Link (BAUARTEN-SPEC Bauart 1 Regel 4).
          return (
            <div className="flex items-center gap-1 pr-2">
              <div className="min-w-0 flex-1">{row}</div>
              <OverflowMenu
                ariaLabel={`Aktionen für ${displayName}`}
                items={[
                  {
                    label: inviteLabel,
                    onClick: () => onInviteExternal(teacher),
                  },
                ]}
              />
            </div>
          );
        }}
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
