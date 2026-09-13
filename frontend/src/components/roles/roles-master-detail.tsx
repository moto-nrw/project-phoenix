"use client";

import { Pencil } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import { DetailLoadingSpinner } from "~/components/database/detail-loading-spinner";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import { GroupedList } from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import { useGroupedItems } from "~/components/database/use-grouped-items";
import { rolesConfig } from "~/components/database/configs/roles.config";
import { DatabaseForm } from "~/components/ui/database/database-form";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { RolePermissionsTab } from "~/components/roles/role-permissions-tab";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import {
  getBaseRoleLabel,
  getRoleDisplayDescription,
  getRoleDisplayName,
  type Role,
} from "@/lib/auth-helpers";

interface RolesMasterDetailProps {
  roles: Role[];
  selectedId: string | null;
  selectedRole: Role | null;
  detailLoading: boolean;
  onSelect: (id: string | null) => void;
  /** Speichert die im Detailbereich bearbeiteten Stammdaten. */
  onSaveRole: (data: Partial<Role>) => Promise<void>;
  onDeleteClick: () => void;
  /** Nach dem Speichern der Berechtigungen: Liste und Detail neu laden. */
  onPermissionsSaved: () => void | Promise<void>;
}

function keyForRole(role: Role): string {
  return role.id;
}

function buildSubtitle(role: Role): string {
  const description = getRoleDisplayDescription(role.name, role.description);
  return description?.trim() || "Keine Beschreibung";
}

export function RolesMasterDetail({
  roles,
  selectedId,
  selectedRole,
  detailLoading,
  onSelect,
  onSaveRole,
  onDeleteClick,
  onPermissionsSaved,
}: RolesMasterDetailProps) {
  const groupDefinitions = useGroupedItems(roles, "none", {}, "Rollen");

  const renderItem = (role: Role) => (
    <DatabaseListItem
      title={getRoleDisplayName(role.name)}
      subtitle={buildSubtitle(role)}
      isSelected={selectedId === role.id}
      onSelect={() => onSelect(role.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForRole}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Keine Rollen gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedRole ? (
    <RoleDetailContent
      key={selectedRole.id}
      role={selectedRole}
      loading={detailLoading}
      onSaveRole={onSaveRole}
      onDeleteClick={onDeleteClick}
      onPermissionsSaved={onPermissionsSaved}
    />
  ) : (
    <EmptyDetailState
      title="Keine Rolle ausgewählt"
      description="Wähle links eine Rolle, um die Details zu sehen."
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
        selectedRole ? getRoleDisplayName(selectedRole.name) : "Rolle"
      }
    />
  );
}

interface RoleDetailContentProps {
  role: Role;
  loading: boolean;
  onSaveRole: (data: Partial<Role>) => Promise<void>;
  onDeleteClick: () => void;
  onPermissionsSaved: () => void | Promise<void>;
}

function RoleDetailContent({
  role,
  loading,
  onSaveRole,
  onDeleteClick,
  onPermissionsSaved,
}: RoleDetailContentProps) {
  const [activeTab, setActiveTab] = useState<string>("master-data");
  // Bearbeitet wird am Objekt, nicht in einem Modal daneben (BAUARTEN-SPEC
  // Bauart 2 Regel 3): „Bearbeiten" schaltet den OFFENEN Reiter um, Stammdaten
  // wie Berechtigungen (#3116). Ein Reiterwechsel beendet den Zustand, der
  // Entwurf gehört zum Reiter.
  const [editing, setEditing] = useState(false);

  const handleSaveRole = async (data: Partial<Role>) => {
    await onSaveRole(data);
    setEditing(false);
  };

  const handlePermissionsSaved = async () => {
    await onPermissionsSaved();
    setEditing(false);
  };

  const handleTabChange = (id: string) => {
    setEditing(false);
    setActiveTab(id);
  };

  const headerActions =
    role.isSystem || editing ? null : (
      <>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => setEditing(true)}
        >
          <Pencil className="h-3.5 w-3.5" aria-hidden />
          Bearbeiten
        </Button>
        <DetailDeleteButton onClick={onDeleteClick} />
      </>
    );

  const tabs: DetailTab[] = [
    {
      id: "master-data",
      label: "Stammdaten",
      content: (
        <RoleStammdatenTab
          role={role}
          loading={loading}
          editing={editing && activeTab === "master-data"}
          onSaveRole={handleSaveRole}
          onCancelEdit={() => setEditing(false)}
        />
      ),
    },
    {
      id: "permissions",
      label: "Berechtigungen",
      content: (
        <RolePermissionsTab
          key={role.id}
          role={role}
          editing={editing && activeTab === "permissions"}
          onSaved={handlePermissionsSaved}
          onCancelEdit={() => setEditing(false)}
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
              icon={MOTO_CONCEPTS.roles.icon}
              tone={MOTO_CONCEPTS.roles.tone}
              size={36}
            />
          }
          title={getRoleDisplayName(role.name)}
          subtitle={buildSubtitle(role)}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={handleTabChange}
    />
  );
}

function RoleStammdatenTab({
  role,
  loading,
  editing,
  onSaveRole,
  onCancelEdit,
}: {
  role: Role;
  loading: boolean;
  editing: boolean;
  onSaveRole: (data: Partial<Role>) => Promise<void>;
  onCancelEdit: () => void;
}) {
  if (loading) {
    return <DetailLoadingSpinner label="Rollendaten werden geladen..." />;
  }

  const description = getRoleDisplayDescription(role.name, role.description);

  if (editing) {
    return (
      <DatabaseForm<Partial<Role>>
        sections={rolesConfig.form.sections}
        initialData={role}
        onSubmit={onSaveRole}
        onCancel={onCancelEdit}
        submitLabel="Speichern"
        stickyActions
      />
    );
  }

  return (
    <div className="space-y-4">
      <section className="rounded-lg border border-gray-200 bg-white p-4">
        <div className="flex flex-wrap items-center gap-2">
          {role.isSystem ? (
            <span className="rounded-full bg-gray-100 px-2.5 py-1 text-xs font-medium text-gray-600">
              Systemrolle
            </span>
          ) : null}
          {!role.isSystem && !role.baseRole ? (
            <span className="bg-moto-amber-soft text-moto-amber-strong rounded-full px-2.5 py-1 text-xs font-medium">
              Zuordnung fehlt
            </span>
          ) : null}
          <span className="bg-moto-purple-soft text-moto-purple-strong rounded-full px-2.5 py-1 text-xs font-medium">
            {role.permissions?.length ?? 0} Berechtigungen
          </span>
        </div>
      </section>

      <InfoSection
        title="Rollendetails"
        icon={
          <MotoDuotoneIcon
            icon={MOTO_CONCEPTS.roles.icon}
            tone={MOTO_CONCEPTS.roles.tone}
            size={18}
          />
        }
        accentColor="purple"
      >
        <DataGrid>
          <DataField label="Name">{getRoleDisplayName(role.name)}</DataField>
          <DataField label="Berechtigungen">
            {role.permissions?.length ?? 0}
          </DataField>
          <DataField label="Systemrolle">
            {getBaseRoleLabel(role.baseRole)}
          </DataField>
          <DataField label="Beschreibung" fullWidth>
            <span className="whitespace-pre-wrap">
              {description || "Keine Beschreibung"}
            </span>
          </DataField>
        </DataGrid>
      </InfoSection>

      {role.isSystem ? (
        <Alert
          type="info"
          message="System-Rollen können nicht bearbeitet oder gelöscht werden."
        />
      ) : null}
    </div>
  );
}
