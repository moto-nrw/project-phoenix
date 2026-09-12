"use client";

import { useEffect, useState } from "react";
import { Button } from "~/components/ui/button";
import {
  DataField,
  DataGrid,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { Textarea } from "~/components/ui/textarea";
import { getRoleDisplayName } from "~/lib/auth-helpers";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";
import { useClipboardCopy } from "~/lib/use-clipboard-copy";
import type { Teacher } from "~/lib/teacher-api";

const logger = createLogger({ component: "StaffKontoTab" });

interface KontoTabProps {
  readonly teacher: Teacher;
  /** Personalnotizen (staff:manage). Ohne das Recht fehlt der Abschnitt. */
  readonly onUpdateNotes?: (notes: string) => Promise<void>;
  /** Kontoaktionen (users:manage und ein verknüpftes Konto). */
  readonly onManageRole?: () => void;
  readonly onManageMFA?: () => void;
  readonly onManageCaregiver?: () => void;
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

/**
 * Der Reiter „Konto" der Personalakte (#3115): Rolle, Zugang, Notizen und die
 * Kontoaktionen einer Person. Das war das Pane der Datenverwaltung; jetzt ist
 * die Personalakte die einzige Objektansicht (BAUARTEN-SPEC Bauart 2), und
 * dieser Reiter trägt, was die HR-Stammdaten nicht kennen: Systemrolle,
 * RFID-Karte, Personal-ID, Notizen der Leitung und den Zugang zum Konto.
 */
export function KontoTab({
  teacher,
  onUpdateNotes,
  onManageRole,
  onManageMFA,
  onManageCaregiver,
}: KontoTabProps) {
  const displayRole = teacher.account_role
    ? getRoleDisplayName(teacher.account_role)
    : null;
  const trimmedQualifications = teacher.qualifications?.trim() ?? "";
  // Kontoaktionen gehören in den Kopf ihres Abschnitts, nicht in eine eigene
  // Zeile aus Knöpfen. Die erste bleibt sichtbar, der Rest wandert ins Menü.
  const accountActions: Array<{ label: string; onClick: () => void }> = [];
  if (onManageRole) {
    accountActions.push({ label: "Rolle verwalten", onClick: onManageRole });
  }
  if (onManageMFA) {
    accountActions.push({
      label: "Zwei-Faktor-Authentifizierung verwalten",
      onClick: onManageMFA,
    });
  }
  if (onManageCaregiver) {
    accountActions.push({
      label: "Betreuung verwalten",
      onClick: onManageCaregiver,
    });
  }
  const primaryAccountAction = accountActions[0];

  return (
    <SectionCard
      title="Konto"
      description="Zugang, Rolle und Notizen der Leitung zu dieser Person."
    >
      <div className="space-y-4">
        <InfoSection
          title="Rolle und Zugang"
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
            Feld gar nicht erst aus, also entfällt der Abschnitt ganz. */}
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
            <NotesEditor
              key={teacher.id}
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

        {primaryAccountAction ? (
          <SectionCard
            title="Konto und Zugriff"
            titleClassName="text-sm"
            headingLevel={3}
            description="Rolle, Zwei-Faktor-Authentifizierung und Betreuungszugriff dieser Person."
            actions={
              <>
                <Button
                  type="button"
                  variant="outline"
                  size="md"
                  onClick={primaryAccountAction.onClick}
                >
                  {primaryAccountAction.label}
                </Button>
                <OverflowMenu
                  ariaLabel="Weitere Kontoaktionen"
                  items={accountActions.slice(1)}
                />
              </>
            }
          />
        ) : null}
      </div>
    </SectionCard>
  );
}

/**
 * Notizen als Bearbeiten-Zustand mit einem „Speichern" unten (Bauart 2
 * Regel 4): kein Schreiben aus dem Feld heraus.
 */
function NotesEditor({
  initialNotes,
  onSave,
}: {
  readonly initialNotes: string;
  readonly onSave: (notes: string) => Promise<void>;
}) {
  const [isEditing, setIsEditing] = useState(false);
  const [notes, setNotes] = useState(initialNotes);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isEditing) setNotes(initialNotes);
  }, [initialNotes, isEditing]);

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(notes);
      setIsEditing(false);
    } catch (err) {
      logger.error("notes_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Die Notizen konnten nicht gespeichert werden.");
    } finally {
      setSaving(false);
    }
  };

  if (isEditing) {
    return (
      <div className="space-y-3">
        <FormErrorAlert message={error} />
        <Textarea
          id="staff-notes"
          name="staff-notes"
          label="Notizen der Leitung"
          value={notes}
          onChange={(event) => setNotes(event.target.value)}
          rows={4}
          placeholder="Notizen hinzufügen…"
          disabled={saving}
          autoFocus
        />
        <EditActions
          onCancel={() => {
            setNotes(initialNotes);
            setError(null);
            setIsEditing(false);
          }}
          onSave={() => void handleSave()}
          saving={saving}
        />
      </div>
    );
  }

  const trimmed = initialNotes.trim();
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      {trimmed.length > 0 ? (
        <p className="min-w-0 flex-1 text-sm break-words whitespace-pre-wrap text-gray-700">
          {trimmed}
        </p>
      ) : (
        <p className="min-w-0 flex-1 text-sm text-gray-500">
          Keine Notizen hinterlegt.
        </p>
      )}
      <Button
        type="button"
        variant="outline"
        size="compact"
        onClick={() => setIsEditing(true)}
      >
        Notizen bearbeiten
      </Button>
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
