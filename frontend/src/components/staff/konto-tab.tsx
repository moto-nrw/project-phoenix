"use client";

import { useState } from "react";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { useFormError } from "~/components/ui/form-error";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { SectionCard } from "~/components/ui/section-card";
import { Textarea } from "~/components/ui/textarea";
import type { AccountRoleAssignment } from "~/lib/account-role-assignment";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";
import type { Teacher } from "~/lib/teacher-api";

const logger = createLogger({ component: "StaffKontoTab" });

/**
 * Was der Bearbeiten-Zustand speichert. Nur die Felder, die die Person
 * ändern darf, sind gesetzt: ohne users:update fehlen Vorname und Nachname,
 * ohne Betreuungsprofil die Position, ohne Rollenwechsel `role_id`.
 */
export interface KontoDraft {
  readonly first_name?: string;
  readonly last_name?: string;
  readonly role?: string | null;
  readonly staff_notes: string;
  /** Neue Systemrolle, nur wenn sie sich von der aktuellen unterscheidet. */
  readonly role_id?: number;
}

/** Der Bearbeiten-Zustand des Reiters (staff:manage). */
export interface KontoEditing {
  /**
   * Vorname und Nachname liegen am Personen-Datensatz (PUT /api/users/{id},
   * users:update). Wer nur staff:manage hat, sieht sie als Anzeige (#2906).
   */
  readonly canEditPersonFields: boolean;
  /** Positionen der Schule als Vorschläge; erst geladen, wenn bearbeitet wird. */
  readonly existingPositions: readonly string[];
  /** Systemrolle (users:manage und ein verknüpftes Konto). */
  readonly canEditRole: boolean;
  /** Rollen des Kontos; `undefined`, solange sie laden. */
  readonly roleAssignment?: AccountRoleAssignment;
  /** Die Rollen konnten nicht geladen werden; das Feld bleibt dann aus. */
  readonly roleAssignmentError?: boolean;
  /** Meldet, ob der Reiter gerade bearbeitet; die Seite lädt dann die Vorschläge. */
  readonly onEditingChange: (editing: boolean) => void;
  /** Speichert den Entwurf; wirft mit einer lesbaren Meldung, wenn es scheitert. */
  readonly onSave: (draft: KontoDraft) => Promise<void>;
}

interface KontoTabProps {
  readonly teacher: Teacher;
  /** Ohne staff:manage gibt es keinen Bearbeiten-Zustand und keine Notizen. */
  readonly editing?: KontoEditing;
}

interface Draft {
  firstName: string;
  lastName: string;
  position: string;
  notes: string;
  roleId: string;
}

function formatTimestamp(iso: string): string {
  return new Date(iso).toLocaleDateString("de-DE", {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Position lebt auf users.teachers; ohne Betreuungsprofil kann sie nirgends
 *  gespeichert werden, das Feld fehlt dann statt kommentarlos zu verwerfen. */
function hasTeacherProfile(teacher: Teacher): boolean {
  return teacher.is_teacher ?? Boolean(teacher.teacher_id);
}

const staffIcon = (
  <MotoDuotoneIcon
    icon={MOTO_CONCEPTS.staff.icon}
    tone={MOTO_CONCEPTS.staff.tone}
    size={18}
  />
);

/**
 * Der Reiter „Konto" der Personalakte (#3115): Rolle, Zugang und Notizen einer
 * Person. Bearbeitet wird am Objekt (BAUARTEN-SPEC Bauart 2 Regeln 3 und 4,
 * #3116): „Bearbeiten" schaltet den Reiter in EINEN Bearbeiten-Zustand mit
 * Name, Systemrolle, Position und Notizen und EINEM „Speichern" unten. Die
 * Kontoaktionen mit eigenem Ablauf (Zwei-Faktor-Authentifizierung,
 * Betreuung) stehen im Kebab der Kopfkarte.
 */
export function KontoTab({ teacher, editing }: KontoTabProps) {
  const [draft, setDraft] = useState<Draft | null>(null);
  const [fieldErrors, setFieldErrors] = useState<{
    firstName?: string;
    lastName?: string;
  }>({});
  const [saving, setSaving] = useState(false);
  const [error, setError] = useFormError();

  const displayRole = teacher.account_role
    ? getRoleDisplayName(teacher.account_role)
    : null;
  const trimmedQualifications = teacher.qualifications?.trim() ?? "";
  const trimmedNotes = teacher.staff_notes?.trim() ?? "";
  const isEditing = editing !== undefined && draft !== null;

  const startEditing = () => {
    if (!editing) return;
    setDraft({
      firstName: teacher.first_name ?? "",
      lastName: teacher.last_name ?? "",
      position: teacher.role ?? "",
      notes: teacher.staff_notes ?? "",
      roleId: "",
    });
    setFieldErrors({});
    setError(null);
    editing.onEditingChange(true);
  };

  const stopEditing = () => {
    setDraft(null);
    setFieldErrors({});
    setError(null);
    editing?.onEditingChange(false);
  };

  const patchDraft = (patch: Partial<Draft>) =>
    setDraft((current) => (current ? { ...current, ...patch } : current));

  const handleSave = async () => {
    if (!editing || !draft) return;
    const assignment = editing.roleAssignment;
    const currentRoleId = assignment?.currentRoleIds[0];
    // Solange niemand gewählt hat, gilt die aktuelle Rolle.
    const selectedRoleId =
      draft.roleId === "" ? currentRoleId : Number(draft.roleId);

    const nextFieldErrors: typeof fieldErrors = {};
    if (editing.canEditPersonFields) {
      if (!draft.firstName.trim()) {
        nextFieldErrors.firstName = "Vorname ist erforderlich.";
      }
      if (!draft.lastName.trim()) {
        nextFieldErrors.lastName = "Nachname ist erforderlich.";
      }
    }
    setFieldErrors(nextFieldErrors);
    if (Object.keys(nextFieldErrors).length > 0) {
      setError("Bitte prüfen Sie die markierten Felder.");
      return;
    }

    const payload: KontoDraft = {
      staff_notes: draft.notes,
      ...(editing.canEditPersonFields
        ? {
            first_name: draft.firstName.trim(),
            last_name: draft.lastName.trim(),
          }
        : {}),
      ...(hasTeacherProfile(teacher)
        ? { role: draft.position.trim() || null }
        : {}),
      ...(editing.canEditRole &&
      assignment &&
      !assignment.currentIsLehrkraft &&
      selectedRoleId !== undefined &&
      !assignment.currentRoleIds.includes(selectedRoleId)
        ? { role_id: selectedRoleId }
        : {}),
    };

    setSaving(true);
    setError(null);
    try {
      await editing.onSave(payload);
      stopEditing();
    } catch (err) {
      logger.error("konto_save_failed", {
        staff_id: teacher.id,
        error: err instanceof Error ? err.message : String(err),
      });
      setError(
        err instanceof Error && err.message
          ? err.message
          : "Die Änderungen konnten nicht gespeichert werden.",
      );
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard
      title="Konto"
      description="Zugang, Rolle und Notizen der Leitung zu dieser Person."
      actions={
        editing && !isEditing ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={startEditing}
          >
            Bearbeiten
          </Button>
        ) : undefined
      }
    >
      {isEditing && draft && editing ? (
        <KontoEditForm
          teacher={teacher}
          draft={draft}
          editing={editing}
          fieldErrors={fieldErrors}
          error={error}
          saving={saving}
          onPatch={patchDraft}
          onCancel={stopEditing}
          onSave={() => void handleSave()}
        />
      ) : (
        <div className="space-y-4">
          <InfoSection
            title="Rolle und Zugang"
            icon={staffIcon}
            accentColor="orange"
          >
            <DataGrid>
              <DataField label="Systemrolle">
                {displayRole ?? "Keine Rolle hinterlegt"}
              </DataField>
              <DataField label="Position">
                {teacher.role?.trim() || "–"}
              </DataField>
              {teacher.specialization?.trim() ? (
                <DataField label="Fachrichtung">
                  {teacher.specialization}
                </DataField>
              ) : null}
              {teacher.tag_id ? (
                <DataField label="RFID-Karte" mono>
                  {teacher.tag_id}
                </DataField>
              ) : null}
              <DataField label="Personal-ID" mono>
                {teacher.id}
              </DataField>
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
              <EmailActions
                email={teacher.email}
                name={
                  teacher.name ||
                  `${teacher.first_name ?? ""} ${teacher.last_name ?? ""}`.trim()
                }
              />
            </InfoSection>
          ) : null}

          {trimmedQualifications ? (
            <InfoSection
              title="Berufliche Informationen"
              icon={staffIcon}
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
              staff:manage (#2906). Ohne die Berechtigung liefert das Backend
              das Feld gar nicht erst aus, also entfällt der Abschnitt ganz. */}
          {editing ? (
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
              {trimmedNotes ? (
                <p className="text-sm break-words whitespace-pre-wrap text-gray-700">
                  {trimmedNotes}
                </p>
              ) : (
                <p className="text-sm text-gray-500">
                  Keine Notizen hinterlegt.
                </p>
              )}
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
                {teacher.created_at ? (
                  <DataField label="Erstellt am">
                    {formatTimestamp(teacher.created_at)}
                  </DataField>
                ) : null}
                {teacher.updated_at ? (
                  <DataField label="Aktualisiert am">
                    {formatTimestamp(teacher.updated_at)}
                  </DataField>
                ) : null}
              </DataGrid>
            </InfoSection>
          ) : null}
        </div>
      )}
    </SectionCard>
  );
}

/**
 * Der Bearbeiten-Zustand: Name, Rolle und Zugang, Notizen als Felder, ein
 * `EditActions` unten. Fehler stehen im Alert oben und am Feld (Regel 5).
 */
function KontoEditForm({
  teacher,
  draft,
  editing,
  fieldErrors,
  error,
  saving,
  onPatch,
  onCancel,
  onSave,
}: {
  readonly teacher: Teacher;
  readonly draft: Draft;
  readonly editing: KontoEditing;
  readonly fieldErrors: { firstName?: string; lastName?: string };
  readonly error: ReturnType<typeof useFormError>[0];
  readonly saving: boolean;
  readonly onPatch: (patch: Partial<Draft>) => void;
  readonly onCancel: () => void;
  readonly onSave: () => void;
}) {
  const assignment = editing.roleAssignment;
  const currentRoleId = assignment?.currentRoleIds[0];
  const roleValue =
    draft.roleId !== ""
      ? draft.roleId
      : currentRoleId === undefined
        ? ""
        : String(currentRoleId);
  const showPosition = hasTeacherProfile(teacher);
  const displayRole = teacher.account_role
    ? getRoleDisplayName(teacher.account_role)
    : "Keine Rolle hinterlegt";

  return (
    <form
      className="space-y-4"
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        onSave();
      }}
    >
      <FormErrorAlert message={error} />

      <InfoSection title="Person" icon={staffIcon} accentColor="orange">
        {editing.canEditPersonFields ? (
          <div className="grid gap-3 md:grid-cols-2">
            <Input
              controlSize="compact"
              label="Vorname"
              name="konto-first-name"
              value={draft.firstName}
              onChange={(event) => onPatch({ firstName: event.target.value })}
              error={fieldErrors.firstName}
              autoComplete="given-name"
              disabled={saving}
            />
            <Input
              controlSize="compact"
              label="Nachname"
              name="konto-last-name"
              value={draft.lastName}
              onChange={(event) => onPatch({ lastName: event.target.value })}
              error={fieldErrors.lastName}
              autoComplete="family-name"
              disabled={saving}
            />
          </div>
        ) : (
          // Ohne users:update sind Name und RFID-Karte nur Anzeige, und sehen
          // auch so aus, statt als Feld, das beim Speichern scheitert.
          <div className="space-y-3">
            <DataGrid>
              <DataField label="Vorname">{draft.firstName || "–"}</DataField>
              <DataField label="Nachname">{draft.lastName || "–"}</DataField>
            </DataGrid>
            <p className="text-xs text-gray-600">
              Name und RFID-Karte ändert die OGS-Leitung.
            </p>
          </div>
        )}
      </InfoSection>

      <InfoSection
        title="Rolle und Zugang"
        icon={staffIcon}
        accentColor="orange"
      >
        <div className="space-y-3">
          {editing.canEditRole ? (
            <RoleField
              assignment={assignment}
              loadFailed={editing.roleAssignmentError === true}
              displayRole={displayRole}
              value={roleValue}
              disabled={saving}
              onChange={(next) => onPatch({ roleId: next })}
            />
          ) : (
            <DataGrid>
              <DataField label="Systemrolle">{displayRole}</DataField>
            </DataGrid>
          )}
          {showPosition ? (
            <div>
              <Input
                controlSize="compact"
                label="Position"
                name="konto-position"
                list="konto-position-suggestions"
                value={draft.position}
                onChange={(event) => onPatch({ position: event.target.value })}
                placeholder="z. B. Pädagogische Fachkraft, OGS-Büro"
                disabled={saving}
              />
              {editing.existingPositions.length > 0 ? (
                <datalist id="konto-position-suggestions">
                  {editing.existingPositions.map((position) => (
                    <option key={position} value={position} />
                  ))}
                </datalist>
              ) : null}
            </div>
          ) : null}
          {teacher.tag_id ? (
            <DataGrid>
              <DataField label="RFID-Karte" mono>
                {teacher.tag_id}
              </DataField>
            </DataGrid>
          ) : null}
        </div>
      </InfoSection>

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
        <Textarea
          name="konto-notes"
          label="Notizen der Leitung"
          value={draft.notes}
          onChange={(event) => onPatch({ notes: event.target.value })}
          rows={4}
          placeholder="Notizen hinzufügen…"
          disabled={saving}
        />
      </InfoSection>

      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}

function RoleField({
  assignment,
  loadFailed,
  displayRole,
  value,
  disabled,
  onChange,
}: {
  readonly assignment: AccountRoleAssignment | undefined;
  readonly loadFailed: boolean;
  readonly displayRole: string;
  readonly value: string;
  readonly disabled: boolean;
  readonly onChange: (next: string) => void;
}) {
  if (loadFailed) {
    return (
      <div className="space-y-1">
        <DataGrid>
          <DataField label="Systemrolle">{displayRole}</DataField>
        </DataGrid>
        <p className="text-xs text-gray-600">
          Die Rollen konnten nicht geladen werden. Die Systemrolle bleibt, wie
          sie ist.
        </p>
      </div>
    );
  }
  if (assignment?.currentIsLehrkraft) {
    return (
      <div className="space-y-1">
        <DataGrid>
          <DataField label="Systemrolle">{displayRole}</DataField>
        </DataGrid>
        <p className="text-xs text-gray-600">
          Lehrkraft-Zugänge haben kein Betreuungsprofil und können hier nicht
          umgestellt werden. Für einen Wechsel den Zugang deaktivieren und die
          Person neu anlegen oder einladen.
        </p>
      </div>
    );
  }
  return (
    <div>
      <label
        id="konto-role-label"
        htmlFor="konto-role"
        className="mb-2 block text-sm font-medium text-gray-700"
      >
        Systemrolle
      </label>
      <CustomSelect
        id="konto-role"
        ariaLabelledBy="konto-role-label"
        value={value}
        onChange={onChange}
        options={(assignment?.options ?? []).map((option) => ({
          value: String(option.id),
          label: option.name,
        }))}
        placeholder={assignment ? "Rolle auswählen…" : "Rollen werden geladen…"}
        disabled={disabled || !assignment}
      />
    </div>
  );
}

function EmailActions({
  email,
  name,
}: {
  readonly email: string;
  readonly name: string;
}) {
  const { copied, copy } = useClipboardCopy("StaffEmailActions", 2000);

  const handleMailto = () => {
    const subject = name ? `Betreff: ${name}` : "Kontaktanfrage";
    globalThis.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <span className="min-w-0 flex-1 truncate text-sm text-gray-900">
        {email}
      </span>
      <div className="flex shrink-0 items-center gap-1.5">
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={handleMailto}
        >
          Schreiben
        </Button>
        <Button
          type="button"
          variant="outline"
          size="compact"
          onClick={() => void copy(email)}
        >
          {copied ? "Kopiert" : "Kopieren"}
        </Button>
      </div>
    </div>
  );
}
