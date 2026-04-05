"use client";

import { useEffect, useState } from "react";
import Image from "next/image";
import { Modal } from "~/components/ui/modal";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import { ModalLoadingState } from "~/components/ui/modal-loading-state";
import {
  DataField,
  DataGrid,
  InfoSection,
  DetailIcons,
} from "~/components/ui/detail-modal-components";
import type { Teacher } from "@/lib/teacher-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TeacherDetailModal" });

interface TeacherDetailModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly teacher: Teacher | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly onManageCaregiver?: () => void;
  readonly onUpdateNotes?: (notes: string) => Promise<void>;
  readonly loading?: boolean;
  /**
   * Custom click handler for delete button.
   * When provided, bypasses internal confirmation modal.
   * Use this to handle confirmation at the page level.
   */
  readonly onDeleteClick?: () => void;
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
          className="w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-xs text-gray-700 transition-colors focus:border-[#F78C10] focus:ring-1 focus:ring-[#F78C10] md:text-sm"
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
          <span className="ml-2 inline-block text-gray-400">
            <svg
              className="inline h-3 w-3"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              strokeWidth={2}
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"
              />
            </svg>
          </span>
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
  const [copied, setCopied] = useState(false);

  const handleMailto = () => {
    const subject = name ? `Betreff: ${name}` : "Kontaktanfrage";
    globalThis.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(email);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback for older browsers
      const textArea = document.createElement("textarea");
      textArea.value = email;
      textArea.style.position = "fixed";
      textArea.style.opacity = "0";
      document.body.appendChild(textArea);
      textArea.select();
      document.execCommand("copy");
      document.body.removeChild(textArea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
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
          <svg
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
            />
          </svg>
          <span className="hidden sm:inline">Schreiben</span>
        </button>
        <button
          type="button"
          onClick={handleCopy}
          className={`inline-flex items-center gap-1 rounded-lg border px-2.5 py-1.5 text-xs font-medium transition-all duration-200 ${
            copied
              ? "border-green-200 bg-green-50 text-green-700"
              : "border-gray-200 text-gray-600 hover:border-gray-300 hover:bg-gray-50 hover:text-gray-900"
          }`}
          title="E-Mail kopieren"
        >
          {copied ? (
            <svg
              className="h-3.5 w-3.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M5 13l4 4L19 7"
              />
            </svg>
          ) : (
            <svg
              className="h-3.5 w-3.5"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"
              />
            </svg>
          )}
          <span className="hidden sm:inline">
            {copied ? "Kopiert" : "Kopieren"}
          </span>
        </button>
      </div>
    </div>
  );
}

export function TeacherDetailModal({
  isOpen,
  onClose,
  teacher,
  onEdit,
  onDelete,
  onManageCaregiver,
  onUpdateNotes,
  loading = false,
  onDeleteClick,
}: TeacherDetailModalProps) {
  if (!teacher) return null;

  // Render helper for modal content
  const renderContent = () => {
    if (loading) {
      return <ModalLoadingState accentColor="orange" />;
    }

    const avatarUrl = teacher.avatar
      ? `/api/staff/${teacher.staff_id ?? teacher.id}/avatar`
      : null;

    return (
      <div className="space-y-4 md:space-y-6">
        {/* Header with Avatar */}
        <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
          <div className="relative flex h-14 w-14 flex-shrink-0 items-center justify-center overflow-hidden rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-lg font-bold text-white shadow-lg md:h-16 md:w-16 md:text-xl">
            {avatarUrl ? (
              <Image
                src={avatarUrl}
                alt={`${teacher.first_name} ${teacher.last_name}`}
                fill
                className="object-cover"
                sizes="64px"
                unoptimized
              />
            ) : (
              <>
                {teacher.first_name?.[0]}
                {teacher.last_name?.[0]}
              </>
            )}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-lg font-bold text-gray-900 md:text-xl">
              {teacher.first_name} {teacher.last_name}
            </h2>
            {teacher.role && (
              <p className="mt-0.5 text-sm text-gray-500">{teacher.role}</p>
            )}
          </div>
        </div>

        {/* Teacher Information Sections */}
        <div className="space-y-4">
          {/* Personal Information */}
          <InfoSection
            title="Persönliche Daten"
            icon={DetailIcons.person}
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

          {/* Email Section */}
          {teacher.email && (
            <InfoSection
              title="E-Mail"
              icon={DetailIcons.document}
              accentColor="orange"
            >
              <EmailActions
                email={teacher.email}
                name={`${teacher.first_name} ${teacher.last_name}`}
              />
            </InfoSection>
          )}

          {/* Professional Information */}
          {(() => {
            const trimmedRole = teacher.role?.trim() ?? "";
            const trimmedQualifications = teacher.qualifications?.trim() ?? "";
            const hasProfessionalInfo = [trimmedQualifications].some(
              (value) => value.length > 0,
            );

            if (!hasProfessionalInfo && !trimmedRole) {
              return null;
            }

            // Only show if there are qualifications (role is shown in header now)
            if (!hasProfessionalInfo) {
              return null;
            }

            return (
              <InfoSection
                title="Berufliche Informationen"
                icon={DetailIcons.briefcase}
                accentColor="orange"
              >
                <DataGrid>
                  {trimmedQualifications && (
                    <DataField label="Qualifikationen" fullWidth>
                      <span className="whitespace-pre-wrap">
                        {trimmedQualifications}
                      </span>
                    </DataField>
                  )}
                </DataGrid>
              </InfoSection>
            );
          })()}

          {/* Staff Notes (inline editable when onUpdateNotes provided) */}
          {onUpdateNotes ? (
            <InfoSection
              title="Notizen"
              icon={DetailIcons.notes}
              accentColor="orange"
            >
              <InlineNotesEditor
                initialNotes={teacher.staff_notes ?? ""}
                onSave={onUpdateNotes}
              />
            </InfoSection>
          ) : (
            (() => {
              const trimmedNotes = teacher.staff_notes?.trim() ?? "";
              if (trimmedNotes.length === 0) {
                return null;
              }
              return (
                <InfoSection
                  title="Notizen"
                  icon={DetailIcons.notes}
                  accentColor="orange"
                >
                  <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                    {trimmedNotes}
                  </p>
                </InfoSection>
              );
            })()
          )}

          {/* Timestamps */}
          {(teacher.created_at ?? teacher.updated_at) && (
            <InfoSection
              title="Zeitstempel"
              icon={DetailIcons.document}
              accentColor="gray"
            >
              <DataGrid>
                {teacher.created_at && (
                  <DataField label="Erstellt am">
                    {new Date(teacher.created_at).toLocaleDateString("de-DE", {
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
          )}
        </div>

        {onManageCaregiver ? (
          <div className="flex justify-end">
            <button
              type="button"
              onClick={onManageCaregiver}
              className="rounded-lg border border-orange-200 px-3 py-2 text-xs font-medium text-orange-700 transition-all duration-200 hover:bg-orange-50 md:text-sm"
            >
              Betreuung verwalten
            </button>
          </div>
        ) : null}

        <DetailModalActions
          onEdit={onEdit}
          onDelete={onDelete}
          onDeleteClick={onDeleteClick}
          entityName={`${teacher.first_name} ${teacher.last_name}`}
          entityType="Personal"
        />
      </div>
    );
  };

  return (
    <Modal isOpen={isOpen} onClose={onClose} title="">
      {renderContent()}
    </Modal>
  );
}
