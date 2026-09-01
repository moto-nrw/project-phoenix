"use client";

import { Pencil } from "lucide-react";
import { useEffect, useState } from "react";
import { DatabaseDetailHeader } from "~/components/database/database-detail-header";
import { DatabaseListItem } from "~/components/database/database-list-item";
import { DetailDeleteButton } from "~/components/database/detail-delete-button";
import {
  DetailPanel,
  type DetailTab,
} from "~/components/database/detail-panel";
import { EmptyDetailState } from "~/components/database/empty-detail-state";
import {
  GroupedList,
  type GroupDefinition,
} from "~/components/database/grouped-list";
import { MasterDetailLayout } from "~/components/database/master-detail-layout";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";
import type { Teacher } from "@/lib/teacher-api";

const logger = createLogger({ component: "StaffMasterDetail" });

interface StaffMasterDetailProps {
  groupDefinitions: GroupDefinition<Teacher>[];
  selectedId: string | null;
  selectedTeacher: Teacher | null;
  onSelect: (id: string | null) => void;
  onEditClick: () => void;
  onDeleteClick: () => void;
  onUpdateNotes?: (notes: string) => Promise<void>;
  onManageCaregiver?: () => void;
  onManageMFA?: () => void;
  onManageRole?: () => void;
}

function keyForTeacher(teacher: Teacher): string {
  return teacher.id;
}

function getDisplayName(teacher: Teacher): string {
  return (
    teacher.name ||
    `${teacher.first_name ?? ""} ${teacher.last_name ?? ""}`.trim() ||
    "Unbekannt"
  );
}

function getInitials(teacher: Teacher): string {
  if (teacher.first_name && teacher.last_name) {
    return `${teacher.first_name[0]}${teacher.last_name[0]}`.toUpperCase();
  }
  if (teacher.name) {
    return teacher.name
      .split(" ")
      .map((part) => part[0])
      .filter(Boolean)
      .slice(0, 2)
      .join("")
      .toUpperCase();
  }
  return "??";
}

function buildRoleLine(teacher: Teacher): string {
  const displayRole = teacher.account_role
    ? getRoleDisplayName(teacher.account_role)
    : null;
  const position =
    teacher.role &&
    teacher.role.toLowerCase() !== displayRole?.toLowerCase() &&
    teacher.role.toLowerCase() !== teacher.account_role?.toLowerCase()
      ? teacher.role
      : null;
  return [displayRole, position].filter(Boolean).join(" · ");
}

function buildSubtitle(teacher: Teacher): string {
  const roleLine = buildRoleLine(teacher);
  if (roleLine) return roleLine;
  if (teacher.email) return teacher.email;
  return "—";
}

export function StaffMasterDetail({
  groupDefinitions,
  selectedId,
  selectedTeacher,
  onSelect,
  onEditClick,
  onDeleteClick,
  onUpdateNotes,
  onManageCaregiver,
  onManageMFA,
  onManageRole,
}: StaffMasterDetailProps) {
  const renderItem = (teacher: Teacher) => (
    <DatabaseListItem
      title={getDisplayName(teacher)}
      subtitle={buildSubtitle(teacher)}
      isSelected={selectedId === teacher.id}
      onSelect={() => onSelect(teacher.id)}
    />
  );

  const listNode = (
    <GroupedList
      groups={groupDefinitions}
      renderItem={renderItem}
      keyFor={keyForTeacher}
      emptyState={
        <div className="text-center text-sm text-gray-500">
          Kein Personal gefunden.
        </div>
      }
    />
  );

  const detailNode = selectedTeacher ? (
    <StaffDetailContent
      teacher={selectedTeacher}
      onEditClick={onEditClick}
      onDeleteClick={onDeleteClick}
      onUpdateNotes={onUpdateNotes}
      onManageCaregiver={onManageCaregiver}
      onManageMFA={onManageMFA}
      onManageRole={onManageRole}
    />
  ) : (
    <EmptyDetailState
      title="Kein Personal ausgewählt"
      description="Wähle links eine Person, um die Details zu sehen."
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
        selectedTeacher ? getDisplayName(selectedTeacher) : "Personal"
      }
    />
  );
}

interface StaffDetailContentProps {
  teacher: Teacher;
  onEditClick: () => void;
  onDeleteClick: () => void;
  onUpdateNotes?: (notes: string) => Promise<void>;
  onManageCaregiver?: () => void;
  onManageMFA?: () => void;
  onManageRole?: () => void;
}

function StaffDetailContent({
  teacher,
  onEditClick,
  onDeleteClick,
  onUpdateNotes,
  onManageCaregiver,
  onManageMFA,
  onManageRole,
}: StaffDetailContentProps) {
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
      content: (
        <StaffStammdatenTab
          teacher={teacher}
          onUpdateNotes={onUpdateNotes}
          onManageCaregiver={onManageCaregiver}
          onManageMFA={onManageMFA}
          onManageRole={onManageRole}
        />
      ),
    },
  ];

  return (
    <DetailPanel
      header={
        <DatabaseDetailHeader
          avatar={getInitials(teacher)}
          title={getDisplayName(teacher)}
          subtitle={buildRoleLine(teacher) || "Keine Rolle hinterlegt"}
          actions={headerActions}
        />
      }
      tabs={tabs}
      activeTab={activeTab}
      onTabChange={setActiveTab}
    />
  );
}

interface StaffStammdatenTabProps {
  teacher: Teacher;
  onUpdateNotes?: (notes: string) => Promise<void>;
  onManageCaregiver?: () => void;
  onManageMFA?: () => void;
  onManageRole?: () => void;
}

function StaffStammdatenTab({
  teacher,
  onUpdateNotes,
  onManageCaregiver,
  onManageMFA,
  onManageRole,
}: StaffStammdatenTabProps) {
  const trimmedQualifications = teacher.qualifications?.trim() ?? "";

  return (
    <div className="space-y-4">
      <InfoSection
        title="Persönliche Daten"
        icon={
          <MotoDuotoneIcon
            icon={MOTO_CONCEPTS.staff.icon}
            tone={MOTO_CONCEPTS.staff.tone}
            size={18}
          />
        }
        accentColor="orange"
      >
        <DataGrid>
          <DataField label="Vorname">{teacher.first_name}</DataField>
          <DataField label="Nachname">{teacher.last_name}</DataField>
          {teacher.tag_id && (
            <DataField label="RFID-Karte" fullWidth mono>
              {teacher.tag_id}
            </DataField>
          )}
          {teacher.id && (
            <DataField label="Personal-ID" fullWidth mono>
              {teacher.id}
            </DataField>
          )}
        </DataGrid>
      </InfoSection>

      {teacher.email ? (
        <InfoSection
          title="E-Mail"
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.messages.icon}
              tone={MOTO_CONCEPTS.messages.tone}
              size={18}
            />
          }
          accentColor="blue"
        >
          <EmailActions email={teacher.email} name={getDisplayName(teacher)} />
        </InfoSection>
      ) : null}

      {trimmedQualifications ? (
        <InfoSection
          title="Berufliche Informationen"
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.staff.icon}
              tone={MOTO_CONCEPTS.staff.tone}
              size={18}
            />
          }
          accentColor="orange"
        >
          <DataGrid>
            <DataField label="Qualifikationen" fullWidth>
              <span className="whitespace-pre-wrap">
                {trimmedQualifications}
              </span>
            </DataField>
          </DataGrid>
        </InfoSection>
      ) : null}

      {/* Personalnotizen gehören zum Mitarbeiter-Datensatz und brauchen
          staff:manage (#2906). Ohne die Berechtigung liefert das Backend das
          Feld gar nicht erst aus, also entfällt die Sektion komplett statt
          eine Bearbeiten-Schaltfläche anzubieten, die 403 zurückgibt. */}
      {onUpdateNotes ? (
        <InfoSection
          title="Notizen"
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.feedback.icon}
              tone={MOTO_CONCEPTS.feedback.tone}
              size={18}
            />
          }
          accentColor="green"
        >
          <InlineNotesEditor
            initialNotes={teacher.staff_notes ?? ""}
            onSave={onUpdateNotes}
          />
        </InfoSection>
      ) : null}

      {teacher.created_at || teacher.updated_at ? (
        <InfoSection
          title="Zeitstempel"
          icon={
            <MotoDuotoneIcon
              icon={MOTO_CONCEPTS.changeHistory.icon}
              tone={MOTO_CONCEPTS.changeHistory.tone}
              size={18}
            />
          }
          accentColor="gray"
        >
          <DataGrid>
            {teacher.created_at && (
              <DataField label="Erstellt am">
                {new Date(teacher.created_at).toLocaleDateString("de-DE", {
                  timeZone: "Europe/Berlin",
                  day: "2-digit",
                  month: "2-digit",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </DataField>
            )}
            {teacher.updated_at && (
              <DataField label="Aktualisiert am">
                {new Date(teacher.updated_at).toLocaleDateString("de-DE", {
                  timeZone: "Europe/Berlin",
                  day: "2-digit",
                  month: "2-digit",
                  year: "numeric",
                  hour: "2-digit",
                  minute: "2-digit",
                })}
              </DataField>
            )}
          </DataGrid>
        </InfoSection>
      ) : null}

      {(onManageCaregiver || onManageMFA || onManageRole) && (
        <div className="flex flex-wrap justify-end gap-2">
          {onManageRole ? (
            <button
              type="button"
              onClick={onManageRole}
              className="border-moto-purple/30 text-moto-purple hover:bg-moto-purple/10 rounded-lg border px-3 py-2 text-xs font-medium transition-all duration-200 md:text-sm"
            >
              Rolle verwalten
            </button>
          ) : null}
          {onManageMFA ? (
            <button
              type="button"
              onClick={onManageMFA}
              className="border-moto-blue/30 text-moto-blue hover:bg-moto-blue/10 rounded-lg border px-3 py-2 text-xs font-medium transition-all duration-200 md:text-sm"
            >
              Zwei-Faktor-Authentifizierung verwalten
            </button>
          ) : null}
          {onManageCaregiver ? (
            <button
              type="button"
              onClick={onManageCaregiver}
              className="border-moto-orange/30 text-moto-orange hover:bg-moto-orange/10 rounded-lg border px-3 py-2 text-xs font-medium transition-all duration-200 md:text-sm"
            >
              Betreuung verwalten
            </button>
          ) : null}
        </div>
      )}
    </div>
  );
}

function InlineNotesEditor({
  initialNotes,
  onSave,
}: {
  initialNotes: string;
  onSave: (notes: string) => Promise<void>;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [notes, setNotes] = useState(initialNotes);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (!isEditing) {
      setNotes(initialNotes);
    }
  }, [initialNotes, isEditing]);

  const handleSave = async () => {
    try {
      setSaving(true);
      await onSave(notes);
      setIsEditing(false);
    } catch (err) {
      logger.error("notes_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setSaving(false);
    }
  };

  const handleCancel = () => {
    setNotes(initialNotes);
    setIsEditing(false);
  };

  if (isEditing) {
    return (
      <div className="space-y-2">
        <textarea
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          className="focus:border-moto-orange focus:ring-moto-orange w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs text-gray-700 transition-colors focus:ring-1 md:text-sm"
          rows={3}
          placeholder="Notizen hinzufügen..."
          disabled={saving}
          autoFocus
        />
        <div className="flex justify-end gap-2">
          <button
            type="button"
            onClick={handleCancel}
            disabled={saving}
            className="rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-50 disabled:opacity-50"
          >
            Abbrechen
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className="rounded-lg bg-gray-900 px-2.5 py-1.5 text-xs font-medium text-white transition-colors hover:bg-gray-700 disabled:opacity-50"
          >
            {saving ? "Speichern..." : "Speichern"}
          </button>
        </div>
      </div>
    );
  }

  const trimmed = initialNotes.trim();
  return (
    <button
      type="button"
      onClick={() => setIsEditing(true)}
      className="group w-full text-left"
      title="Klicken zum Bearbeiten"
    >
      {trimmed.length > 0 ? (
        <p className="text-xs break-words whitespace-pre-wrap text-gray-700 group-hover:text-gray-900 md:text-sm">
          {trimmed}
        </p>
      ) : (
        <p className="text-xs text-gray-400 italic group-hover:text-gray-600 md:text-sm">
          Notizen hinzufügen...
        </p>
      )}
    </button>
  );
}

function EmailActions({ email, name }: { email: string; name: string }) {
  const { copied, copy } = useClipboardCopy("StaffEmailActions", 2000);

  const handleMailto = () => {
    const subject = name ? `Betreff: ${name}` : "Kontaktanfrage";
    globalThis.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
  };

  return (
    <div className="flex items-center gap-3">
      <span className="min-w-0 flex-1 truncate text-sm text-gray-900">
        {email}
      </span>
      <div className="flex flex-shrink-0 items-center gap-1.5">
        <button
          type="button"
          onClick={handleMailto}
          className="inline-flex items-center gap-1 rounded-lg border border-gray-200 px-2.5 py-1.5 text-xs font-medium text-gray-600 transition-all duration-200 hover:border-gray-300 hover:bg-gray-50 hover:text-gray-900"
          title="E-Mail schreiben"
        >
          Schreiben
        </button>
        <button
          type="button"
          onClick={() => void copy(email)}
          className={`inline-flex items-center gap-1 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-all duration-200 ${
            copied
              ? "border-moto-green/30 bg-moto-green/10 text-moto-green"
              : "border-gray-200 text-gray-600 hover:border-gray-300 hover:bg-gray-50 hover:text-gray-900"
          }`}
          title="E-Mail kopieren"
        >
          {copied ? "Kopiert" : "Kopieren"}
        </button>
      </div>
    </div>
  );
}
