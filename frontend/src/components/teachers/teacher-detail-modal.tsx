"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
import { InlineDeleteConfirmation } from "~/components/ui/inline-delete-confirmation";
import { DetailModalActions } from "~/components/ui/detail-modal-actions";
import { ModalLoadingState } from "~/components/ui/modal-loading-state";
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
      /* Detail View */
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
          <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
            <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
              <svg
                className="h-3.5 w-3.5 text-orange-600 md:h-4 md:w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                />
              </svg>
              Persönliche Daten
            </h3>
            <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
              <div>
                <dt className="text-xs text-gray-500">Vorname</dt>
                <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                  {teacher.first_name}
                </dd>
              </div>
              <div>
                <dt className="text-xs text-gray-500">Nachname</dt>
                <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                  {teacher.last_name}
                </dd>
              </div>
              {teacher.email && (
                <div className="col-span-1 sm:col-span-2">
                  <dt className="text-xs text-gray-500">E-Mail</dt>
                  <dd className="mt-0.5 text-xs font-medium break-all text-gray-900 md:text-sm">
                    {teacher.email}
                  </dd>
                </div>
              )}
              {teacher.tag_id && (
                <div className="col-span-1 sm:col-span-2">
                  <dt className="text-xs text-gray-500">RFID-Karte</dt>
                  <dd className="mt-0.5 font-mono text-xs break-all text-gray-600 md:text-sm">
                    {teacher.tag_id}
                  </dd>
                </div>
              )}
              {teacher.id && (
                <div className="col-span-1 sm:col-span-2">
                  <dt className="text-xs text-gray-500">Betreuer-ID</dt>
                  <dd className="mt-0.5 font-mono text-xs break-all text-gray-600 md:text-sm">
                    {teacher.id}
                  </dd>
                </div>
              )}
            </dl>
          </div>

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
              <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-orange-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                    />
                  </svg>
                  Berufliche Informationen
                </h3>
                <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                  {trimmedRole && (
                    <div>
                      <dt className="text-xs text-gray-500">Rolle</dt>
                      <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                        {trimmedRole}
                      </dd>
                    </div>
                  )}
                  {trimmedQualifications && (
                    <div className="col-span-1 sm:col-span-2">
                      <dt className="text-xs text-gray-500">Qualifikationen</dt>
                      <dd className="mt-0.5 text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                        {trimmedQualifications}
                      </dd>
                    </div>
                  )}
                </dl>
              </div>
            );
          })()}

          {/* Staff Notes */}
          {(() => {
            const trimmedNotes = teacher.staff_notes?.trim() ?? "";
            if (trimmedNotes.length === 0) {
              return null;
            }
            return (
              <div className="rounded-xl border border-gray-100 bg-orange-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-orange-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                    />
                  </svg>
                  Notizen
                </h3>
                <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                  {trimmedNotes}
                </p>
              </div>
            );
          })()}

          {/* Timestamps */}
          {(teacher.created_at ?? teacher.updated_at) && (
            <div className="rounded-xl border border-gray-100 bg-gray-50 p-3 md:p-4">
              <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                {teacher.created_at && (
                  <div>
                    <dt className="text-xs text-gray-500">Erstellt am</dt>
                    <dd className="mt-0.5 text-xs font-medium text-gray-900 md:text-sm">
                      {new Date(teacher.created_at).toLocaleDateString(
                        "de-DE",
                        {
                          day: "2-digit",
                          month: "2-digit",
                          year: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        },
                      )}
                    </dd>
                  </div>
                )}
                {teacher.updated_at && (
                  <div>
                    <dt className="text-xs text-gray-500">Aktualisiert am</dt>
                    <dd className="mt-0.5 text-xs font-medium text-gray-900 md:text-sm">
                      {new Date(teacher.updated_at).toLocaleDateString(
                        "de-DE",
                        {
                          day: "2-digit",
                          month: "2-digit",
                          year: "numeric",
                          hour: "2-digit",
                          minute: "2-digit",
                        },
                      )}
                    </dd>
                  </div>
                )}
              </dl>
            </div>
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
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="" // No title, we'll use custom header
    >
      {renderContent()}
    </Modal>
  );
}
