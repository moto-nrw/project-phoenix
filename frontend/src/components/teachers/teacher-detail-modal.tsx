"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { InlineDeleteConfirmation } from "~/components/ui/inline-delete-confirmation";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import { ModalLoadingState } from "~/components/ui/modal-loading-state";
import {
  DataField,
  DataGrid,
  InfoSection,
  DetailIcons,
} from "~/components/ui/detail-modal-components";
import type { Teacher } from "@/lib/teacher-api";

interface TeacherDetailModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly teacher: Teacher | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
}

export function TeacherDetailModal({
  isOpen,
  onClose,
  teacher,
  onEdit,
  onDelete,
  loading = false,
}: TeacherDetailModalProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Reset confirmation state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setShowDeleteConfirm(false);
    }
  }, [isOpen]);

  if (!teacher) return null;

  // Render helper for modal content
  const renderContent = () => {
    if (loading) {
      return <ModalLoadingState accentColor="orange" />;
    }

    if (showDeleteConfirm) {
      return (
        <InlineDeleteConfirmation
          title="Betreuer löschen?"
          onCancel={() => setShowDeleteConfirm(false)}
          onConfirm={() => {
            setShowDeleteConfirm(false);
            onDelete();
          }}
        >
          <p className="text-sm text-gray-700">
            Möchten Sie den Betreuer{" "}
            <strong>
              {teacher.first_name} {teacher.last_name}
            </strong>{" "}
            wirklich löschen?
          </p>
        </InlineDeleteConfirmation>
      );
    }

    return (
      <div className="space-y-4 md:space-y-6">
        {/* Header with Avatar */}
        <div className="flex items-center gap-3 border-b border-gray-100 pb-3 md:gap-4 md:pb-4">
          <div className="flex h-14 w-14 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-[#F78C10] to-[#e57a00] text-lg font-bold text-white shadow-lg md:h-16 md:w-16 md:text-xl">
            {teacher.first_name?.[0]}
            {teacher.last_name?.[0]}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-lg font-bold text-gray-900 md:text-xl">
              {teacher.first_name} {teacher.last_name}
            </h2>
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
              {teacher.email && (
                <DataField label="E-Mail" fullWidth>
                  {teacher.email}
                </DataField>
              )}
              {teacher.tag_id && (
                <DataField label="RFID-Karte" fullWidth mono>
                  {teacher.tag_id}
                </DataField>
              )}
              {teacher.id && (
                <DataField label="Betreuer-ID" fullWidth mono>
                  {teacher.id}
                </DataField>
              )}
            </DataGrid>
          </InfoSection>

          {/* Professional Information */}
          {(() => {
            const trimmedRole = teacher.role?.trim() ?? "";
            const trimmedQualifications = teacher.qualifications?.trim() ?? "";
            const hasProfessionalInfo = [
              trimmedRole,
              trimmedQualifications,
            ].some((value) => value.length > 0);

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
                  {trimmedRole && (
                    <DataField label="Rolle">{trimmedRole}</DataField>
                  )}
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

          {/* Staff Notes */}
          {(() => {
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
          })()}

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

        <DetailModalActions
          onEdit={onEdit}
          onDelete={onDelete}
          onDeleteClick={() => setShowDeleteConfirm(true)}
          entityName={`${teacher.first_name} ${teacher.last_name}`}
          entityType="Betreuer"
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
