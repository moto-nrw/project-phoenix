"use client";

import { MousePointer2 } from "lucide-react";
import { EmptyState } from "~/components/ui/empty-state";
import { useMemo, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { useGroupedItems } from "~/components/database/use-grouped-items";
import { DatabaseForm } from "~/components/database/database-form";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { useApiFormError } from "~/contexts/ToastContext";
import { groupsConfig } from "@/components/database/configs/groups.config";
import { configToFormSection } from "@/lib/database/types";
import type { Group } from "@/lib/group-helpers";
import { groupsConcept } from "~/lib/concepts/groups";
import { StatusBadge } from "~/components/ui/status-badge";
import { Alert } from "~/components/ui/alert";
import {
  holidayGuestGroupHint,
  type HolidayGuestGroup,
  type HolidayGuestGroupHint,
} from "~/lib/holiday-host-api";
import { berlinTodayISO } from "~/lib/date-helpers";

interface GroupsMasterDetailProps {
  groups: Group[];
  selectedId: string | null;
  // Resolved by the page against the unfiltered list so the detail panel
  // survives a search narrowing the visible rows.
  selectedGroup: Group | null;
  onSelect: (id: string | null) => void;
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onDeleteClick: () => void;
  /**
   * Feriengruppen nach Gruppen-ID, die nächste Woche zuerst (ADR 0120,
   * Nachtrag 2026-11-11). Eine Gruppe darin bekommt einen Hinweis, damit sie
   * vor ihrer Woche nicht als leere Gruppe ohne Erklärung dasteht.
   */
  holidayGroups?: ReadonlyMap<string, HolidayGuestGroup>;
}

function keyForGroup(group: Group): string {
  return group.id;
}

function buildGroupSubtitle(
  group: Group,
  hint?: HolidayGuestGroupHint | null,
): string {
  const room = group.room_name?.trim() || "Kein Gruppenraum";
  return hint ? `${room} · ${hint.label}` : room;
}

function hintFor(
  group: Group,
  holidayGroups: ReadonlyMap<string, HolidayGuestGroup> | undefined,
): HolidayGuestGroupHint | null {
  const week = holidayGroups?.get(group.id);
  return week
    ? holidayGuestGroupHint(week, berlinTodayISO(), group.student_count ?? 0)
    : null;
}

export function GroupsMasterDetail({
  groups,
  selectedId,
  selectedGroup,
  onSelect,
  onSaveGroup,
  onDeleteClick,
  holidayGroups,
}: GroupsMasterDetailProps) {
  const groupDefinitions = useGroupedItems(groups, "none", {}, "Gruppen");

  const renderItem = (group: Group) => (
    <DatabaseListItem
      title={group.name}
      subtitle={buildGroupSubtitle(group, hintFor(group, holidayGroups))}
      isSelected={selectedId === group.id}
      onSelect={() => onSelect(group.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForGroup}
      emptyState={
        <EmptyState variant="compact" title="Keine Gruppen gefunden." />
      }
    />
  );

  const detailNode = selectedGroup ? (
    <GroupDetailContent
      group={selectedGroup}
      hint={hintFor(selectedGroup, holidayGroups)}
      onSaveGroup={onSaveGroup}
      onDeleteClick={onDeleteClick}
    />
  ) : (
    <EmptyState
      icon={<MousePointer2 className="h-10 w-10" aria-hidden="true" />}
      titleAs="h3"
      title="Keine Gruppe ausgewählt"
      description="Wähle links eine Gruppe, um Stammdaten direkt im Detailbereich zu bearbeiten."
      className="h-full justify-center"
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={selectedGroup?.name ?? "Gruppe"}
    />
  );
}

interface GroupDetailContentProps {
  group: Group;
  hint: HolidayGuestGroupHint | null;
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onDeleteClick: () => void;
}

function GroupDetailContent({
  group,
  hint,
  onSaveGroup,
  onDeleteClick,
}: GroupDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [formResetCounter, setFormResetCounter] = useState(0);

  const handleSaveGroup = async (data: Partial<Group>) => {
    await onSaveGroup(data);
    setFormResetCounter((n) => n + 1);
  };

  const headerActions = (
    <DetailDeleteButton
      onClick={onDeleteClick}
      tooltip="Löscht eine Gruppe ohne Kinder, moto fragt vorher nach."
    />
  );

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <GroupStammdatenTab
          // Eine andere Gruppe beginnt ohne den Fehler der vorigen.
          key={group.id}
          group={group}
          hint={hint}
          onSaveGroup={handleSaveGroup}
          onResetForm={() => setFormResetCounter((n) => n + 1)}
          formResetKey={`${group.id}:${formResetCounter}`}
        />
      ),
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={groupsConcept.icon}
              tone={groupsConcept.tone}
              size={36}
            />
          }
          title={group.name}
          subtitle={buildGroupSubtitle(group, hint)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

interface GroupStammdatenTabProps {
  group: Group;
  hint: HolidayGuestGroupHint | null;
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onResetForm: () => void;
  formResetKey: string;
}

function GroupStammdatenTab({
  group,
  hint,
  onSaveGroup,
  onResetForm,
  formResetKey,
}: GroupStammdatenTabProps) {
  const sections = useMemo(
    () => groupsConfig.form.sections.map(configToFormSection),
    [],
  );
  // Speicherfehler im Formular, Feldfehler am Feld (#2517).
  const saveErrors = useApiFormError();

  return (
    <div className="space-y-6">
      <GroupSummary group={group} hint={hint} />
      <DatabaseForm
        key={formResetKey}
        sections={sections}
        initialData={group}
        onSubmit={onSaveGroup}
        onCancel={() => {
          saveErrors.clear();
          onResetForm();
        }}
        submitLabel="Speichern"
        stickyActions
        errorPath={saveErrors}
        errorObject="die Gruppe"
      />
    </div>
  );
}

function GroupSummary({
  group,
  hint,
}: {
  group: Group;
  hint: HolidayGuestGroupHint | null;
}) {
  const supervisorNames =
    group.supervisors && group.supervisors.length > 0
      ? group.supervisors.map((s) => s.name).join(", ")
      : "Keine Gruppenleitung zugewiesen";

  return (
    <section className="moto-content-surface rounded-xl border p-4">
      <div className="flex flex-wrap items-center gap-2">
        <StatusBadge
          label={group.room_name ?? "Kein Gruppenraum"}
          tone="gray"
        />
        <StatusBadge label={`${group.student_count ?? 0} Kinder`} tone="gray" />
        {hint ? <StatusBadge label={hint.label} tone="blue" /> : null}
      </div>

      {hint ? (
        <Alert type="info" message={hint.message} className="mt-4" />
      ) : null}

      <div className="mt-4">
        <DataGrid>
          <DataField label="Gruppenleitung" fullWidth>
            <span className="whitespace-pre-wrap">{supervisorNames}</span>
          </DataField>
        </DataGrid>
      </div>
    </section>
  );
}
