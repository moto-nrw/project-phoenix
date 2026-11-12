"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { Pencil, MousePointer2 } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useState } from "react";
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
import { roleEditSections } from "~/components/database/configs/roles.config";
import { DatabaseForm } from "~/components/database/database-form";
import type { FormErrorInput } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { RolePermissionsTab } from "~/components/roles/role-permissions-tab";
import {
  getBaseRoleLabel,
  getRoleDisplayDescription,
  getRoleDisplayName,
  type Role,
} from "@/lib/auth-helpers";
import { rolesConcept } from "~/lib/concepts/roles";
import { StatusBadge } from "~/components/ui/status-badge";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { Loading } from "~/components/ui/loading";

interface RolesMasterDetailProps {
  roles: Role[];
  selectedId: string | null;
  selectedRole: Role | null;
  detailLoading: boolean;
  /** Ladefehler des Details (aus `useApiLoadError`); ohne ihn wären die
   *  Zahlen der Liste ohne Berechtigungen falsch. */
  detailError?: FormErrorInput;
  /** Darf den Berechtigungskatalog laden und Rollenrechte ändern. */
  canManagePermissions?: boolean;
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
  detailError = null,
  canManagePermissions = false,
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
        <EmptyState variant="compact" title="Keine Rollen gefunden." />
      }
    />
  );

  const detailNode = selectedRole ? (
    <RoleDetailContent
      key={selectedRole.id}
      role={selectedRole}
      loading={detailLoading}
      detailError={detailError}
      canManagePermissions={canManagePermissions}
      onSaveRole={onSaveRole}
      onDeleteClick={onDeleteClick}
      onPermissionsSaved={onPermissionsSaved}
    />
  ) : (
    <EmptyState
      icon={<MousePointer2 className="h-10 w-10" aria-hidden="true" />}
      titleAs="h3"
      title="Keine Rolle ausgewählt"
      description="Wähle links eine Rolle, um die Details zu sehen."
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
      mobileDrawerTitle={
        selectedRole ? getRoleDisplayName(selectedRole.name) : "Rolle"
      }
    />
  );
}

interface RoleDetailContentProps {
  role: Role;
  loading: boolean;
  detailError: FormErrorInput;
  canManagePermissions: boolean;
  onSaveRole: (data: Partial<Role>) => Promise<void>;
  onDeleteClick: () => void;
  onPermissionsSaved: () => void | Promise<void>;
}

function RoleDetailContent({
  role,
  loading,
  detailError,
  canManagePermissions,
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

  const canEditActiveTab = activeTab !== "permissions" || canManagePermissions;
  const headerActions =
    role.isSystem || editing ? null : (
      <>
        {canEditActiveTab ? (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => setEditing(true)}
            tooltip={
              activeTab === "permissions"
                ? "Öffnet die Berechtigungen dieser Rolle zum Bearbeiten."
                : "Öffnet Name und Beschreibung der Rolle zum Bearbeiten."
            }
          >
            <Pencil className="h-3.5 w-3.5" aria-hidden />
            Bearbeiten
          </Button>
        ) : null}
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
          detailError={detailError}
          editing={editing && activeTab === "master-data"}
          onSaveRole={handleSaveRole}
          onCancelEdit={() => setEditing(false)}
        />
      ),
    },
    ...(canManagePermissions
      ? [
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
        ]
      : []),
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          icon={
            <MotoDuotoneIcon
              icon={rolesConcept.icon}
              tone={rolesConcept.tone}
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
  detailError,
  editing,
  onSaveRole,
  onCancelEdit,
}: {
  role: Role;
  loading: boolean;
  detailError: FormErrorInput;
  editing: boolean;
  onSaveRole: (data: Partial<Role>) => Promise<void>;
  onCancelEdit: () => void;
}) {
  // Speicherfehler im Formular, Namensdopplung am Feld (#2517).
  const saveErrors = useApiFormError();

  if (loading) {
    return <Loading message="Rollendaten werden geladen..." />;
  }

  // Ohne Detail fehlen die Berechtigungen: keine „0 Berechtigungen“ zeigen.
  if (detailError) {
    return <LoadErrorAlert error={detailError} />;
  }

  const description = getRoleDisplayDescription(role.name, role.description);

  if (editing) {
    return (
      <DatabaseForm<Partial<Role>>
        sections={roleEditSections}
        initialData={role}
        onSubmit={onSaveRole}
        onCancel={onCancelEdit}
        submitLabel="Speichern"
        stickyActions
        errorPath={saveErrors}
        errorObject="die Rolle"
      />
    );
  }

  return (
    <div className="space-y-4">
      <section className="moto-content-surface rounded-xl border p-4">
        <div className="flex flex-wrap items-center gap-2">
          {role.isSystem ? (
            <StatusBadge label="Vorgegeben" tone="gray" />
          ) : null}
          {!role.isSystem && !role.baseRole ? (
            <StatusColorBadge
              label="Zuordnung fehlt"
              color={LOCATION_COLORS.WARNING}
            />
          ) : null}
          <StatusBadge
            label={`${role.permissions?.length ?? 0} ${
              role.permissions?.length === 1 ? "Berechtigung" : "Berechtigungen"
            }`}
            tone="purple"
          />
        </div>
      </section>

      <InfoSection
        title="Rollendetails"
        icon={
          <MotoDuotoneIcon
            icon={rolesConcept.icon}
            tone={rolesConcept.tone}
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
          <DataField label="Gilt als">
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
          message="Vorgegebene Rollen gelten für alle Schulen. Sie können nicht bearbeitet oder gelöscht werden."
        />
      ) : null}
    </div>
  );
}
