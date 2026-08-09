"use client";

import { Pencil } from "lucide-react";
import { useState } from "react";
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
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import type { Permission } from "@/lib/auth-helpers";
import {
  formatPermissionDisplay,
  localizeAction,
  localizeDescription,
  localizeResource,
} from "@/lib/permission-labels";

interface PermissionsMasterDetailProps {
  permissions: Permission[];
  selectedId: string | null;
  // Resolved by the page against the unfiltered list so the detail panel
  // survives a search narrowing the visible rows.
  selectedPermission: Permission | null;
  onSelect: (id: string | null) => void;
  onEditClick: () => void;
  onDeleteClick: () => void;
}

function keyForPermission(permission: Permission): string {
  return permission.id;
}

function getTitle(permission: Permission): string {
  return formatPermissionDisplay(permission.resource, permission.action);
}

function buildSubtitle(permission: Permission): string {
  const description = localizeDescription(
    permission.resource,
    permission.action,
    permission.description,
  );
  if (description?.trim()) return description;
  return `${localizeResource(permission.resource)} · ${localizeAction(
    permission.action,
  )}`;
}

export function PermissionsMasterDetail({
  permissions,
  selectedId,
  selectedPermission,
  onSelect,
  onEditClick,
  onDeleteClick,
}: PermissionsMasterDetailProps) {
  const groupDefinitions = useGroupedItems(
    permissions,
    "none",
    {},
    "Berechtigungen",
  );

  const renderItem = (permission: Permission) => (
    <DatabaseListItem
      title={getTitle(permission)}
      subtitle={buildSubtitle(permission)}
      isSelected={selectedId === permission.id}
      onSelect={() => onSelect(permission.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForPermission}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Berechtigungen gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedPermission ? (
    <PermissionDetailContent
      permission={selectedPermission}
      onEditClick={onEditClick}
      onDeleteClick={onDeleteClick}
    />
  ) : (
    <EmptyDetailState
      title="Keine Berechtigung ausgewählt"
      description="Wähle links eine Berechtigung, um die Details zu sehen."
    />
  );

  return (
    <MasterDetailLayout
      list={listNode}
      detail={detailNode}
      selectedId={selectedId}
      onDeselect={() => onSelect(null)}
      unselectedBehavior="expand"
      mobileDrawerTitle={
        selectedPermission ? getTitle(selectedPermission) : "Berechtigung"
      }
    />
  );
}

interface PermissionDetailContentProps {
  permission: Permission;
  onEditClick: () => void;
  onDeleteClick: () => void;
}

function PermissionDetailContent({
  permission,
  onEditClick,
  onDeleteClick,
}: PermissionDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");

  const headerActions = (
    <>
      <button
        type="button"
        onClick={onEditClick}
        className="flex items-center gap-1.5 rounded-md border border-gray-200 bg-white px-3 py-1.5 text-sm font-medium text-gray-700 hover:bg-gray-50"
      >
        <Pencil className="h-3.5 w-3.5" aria-hidden />
        Bearbeiten
      </button>
      <DetailDeleteButton onClick={onDeleteClick} />
    </>
  );

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: <PermissionStammdatenTab permission={permission} />,
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.permissions.icon}
              tone={MOTO_CONCEPTS.permissions.tone}
              size={36}
            />
          }
          title={getTitle(permission)}
          subtitle={permission.name || "Systemberechtigung"}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

function PermissionStammdatenTab({ permission }: { permission: Permission }) {
  const description = localizeDescription(
    permission.resource,
    permission.action,
    permission.description,
  );
  const technicalName =
    permission.name || `${permission.resource}:${permission.action}`;

  return (
    <div className="space-y-4">
      <InfoSection
        title="Berechtigungsdetails"
        icon={
          <MotoDuotoneIcon
            icon={MOTO_CONCEPTS.permissions.icon}
            tone={MOTO_CONCEPTS.permissions.tone}
            size={18}
          />
        }
        accentColor="purple"
      >
        <DataGrid>
          <DataField label="Ressource">
            {localizeResource(permission.resource)}
          </DataField>
          <DataField label="Aktion">
            {localizeAction(permission.action)}
          </DataField>
          <DataField label="Technischer Name" fullWidth mono>
            {technicalName}
          </DataField>
          <DataField label="Beschreibung" fullWidth>
            <span className="whitespace-pre-wrap">
              {description || "Keine Beschreibung"}
            </span>
          </DataField>
        </DataGrid>
      </InfoSection>
    </div>
  );
}
