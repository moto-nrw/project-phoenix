"use client";

import { useMemo, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { useGroupedItems } from "~/components/database/use-grouped-items";
import { DatabaseForm } from "~/components/ui/database/database-form";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { groupsConfig } from "@/components/database/configs/groups.config";
import { configToFormSection } from "@/lib/database/types";
import type { Group } from "@/lib/group-helpers";

interface GroupsMasterDetailProps {
  groups: Group[];
  selectedId: string | null;
  // Resolved by the page against the unfiltered list so the detail panel
  // survives a search narrowing the visible rows.
  selectedGroup: Group | null;
  onSelect: (id: string | null) => void;
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onDeleteClick: () => void;
}

function keyForGroup(group: Group): string {
  return group.id;
}

function buildGroupSubtitle(group: Group): string {
  return group.room_name?.trim() || "Kein Gruppenraum";
}

export function GroupsMasterDetail({
  groups,
  selectedId,
  selectedGroup,
  onSelect,
  onSaveGroup,
  onDeleteClick,
}: GroupsMasterDetailProps) {
  const groupDefinitions = useGroupedItems(groups, "none", {}, "Gruppen");

  const renderItem = (group: Group) => (
    <DatabaseListItem
      title={group.name}
      subtitle={buildGroupSubtitle(group)}
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
        <div className="text-center text-sm text-gray-500">
          Keine Gruppen gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedGroup ? (
    <GroupDetailContent
      group={selectedGroup}
      onSaveGroup={onSaveGroup}
      onDeleteClick={onDeleteClick}
    />
  ) : (
    <EmptyDetailState
      title="Keine Gruppe ausgewählt"
      description="Wähle links eine Gruppe, um Stammdaten direkt im Detailbereich zu bearbeiten."
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
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onDeleteClick: () => void;
}

function GroupDetailContent({
  group,
  onSaveGroup,
  onDeleteClick,
}: GroupDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  const [formResetCounter, setFormResetCounter] = useState(0);

  const handleSaveGroup = async (data: Partial<Group>) => {
    await onSaveGroup(data);
    setFormResetCounter((n) => n + 1);
  };

  const headerActions = <DetailDeleteButton onClick={onDeleteClick} />;

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <GroupStammdatenTab
          group={group}
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
              icon={MOTO_CONCEPTS.groups.icon}
              tone={MOTO_CONCEPTS.groups.tone}
              size={36}
            />
          }
          title={group.name}
          subtitle={buildGroupSubtitle(group)}
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
  onSaveGroup: (data: Partial<Group>) => Promise<void>;
  onResetForm: () => void;
  formResetKey: string;
}

function GroupStammdatenTab({
  group,
  onSaveGroup,
  onResetForm,
  formResetKey,
}: GroupStammdatenTabProps) {
  const sections = useMemo(
    () => groupsConfig.form.sections.map(configToFormSection),
    [],
  );

  return (
    <div className="space-y-6">
      <GroupSummary group={group} />
      <DatabaseForm
        key={formResetKey}
        sections={sections}
        initialData={group}
        onSubmit={onSaveGroup}
        onCancel={onResetForm}
        submitLabel="Speichern"
        stickyActions
      />
    </div>
  );
}

function GroupSummary({ group }: { group: Group }) {
  const supervisorNames =
    group.supervisors && group.supervisors.length > 0
      ? group.supervisors.map((s) => s.name).join(", ")
      : "Keine Gruppenleitung zugewiesen";

  return (
    <section className="rounded-lg border border-gray-200 bg-white p-4">
      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          {group.room_name ?? "Kein Gruppenraum"}
        </span>
        <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
          {group.student_count ?? 0} Kinder
        </span>
      </div>

      <dl className="mt-4">
        <div>
          <dt className="text-xs font-medium text-gray-500">Gruppenleitung</dt>
          <dd className="mt-0.5 text-sm whitespace-pre-wrap text-gray-900">
            {supervisorNames}
          </dd>
        </div>
      </dl>
    </section>
  );
}
