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
  InfoText,
  DetailIcons,
} from "~/components/ui/detail-modal-components";
import type { Student } from "@/lib/api";

interface Guardian {
  id: string;
  name: string;
  contact: string;
  email: string;
  phone: string;
  relationship: string;
}

interface StudentDetailModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly student: Student | null;
  readonly onEdit: () => void;
  readonly onDelete: () => void;
  readonly loading?: boolean;
  readonly error?: string | null;
}

export function StudentDetailModal({
  isOpen,
  onClose,
  student,
  onEdit,
  onDelete,
  loading = false,
  error = null,
}: StudentDetailModalProps) {
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Type guards for safe JSON parsing
  const isGuardian = (g: unknown): g is Guardian => {
    return (
      typeof g === "object" &&
      g !== null &&
      typeof (g as { id?: unknown }).id === "string" &&
      typeof (g as { name?: unknown }).name === "string"
    );
  };
  const isGuardiansPayload = (
    x: unknown,
  ): x is { guardians: Guardian[]; additionalInfo?: string } => {
    if (typeof x !== "object" || x === null) return false;
    const arr = (x as { guardians?: unknown }).guardians;
    return Array.isArray(arr) && arr.every(isGuardian);
  };
  // Parse guardians and additional notes from student extra_info
  const parseExtraInfo = (
    s: Student,
  ): { guardians: Guardian[] | null; additionalInfo: string | null } => {
    try {
      if (s.extra_info) {
        const parsed: unknown = JSON.parse(s.extra_info);
        if (isGuardiansPayload(parsed)) {
          return {
            guardians: parsed.guardians,
            additionalInfo:
              typeof (parsed as { additionalInfo?: unknown }).additionalInfo ===
              "string"
                ? (parsed as { additionalInfo?: string }).additionalInfo!
                : null,
          };
        }
      }
    } catch {
      // Ignore parse errors and fall through
    }
    return { guardians: null, additionalInfo: null };
  };
  // Parse guardians and additional info safely
  let guardians: Guardian[] | null = null;
  let additionalInfo: string | null = null;
  if (student) {
    const parsed = parseExtraInfo(student);
    guardians = parsed.guardians;
    additionalInfo = parsed.additionalInfo;
  }

  // Reset confirmation state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setShowDeleteConfirm(false);
    }
  }, [isOpen]);

  if (!student) return null;

  // Render helper for modal content
  const renderContent = () => {
    if (error) {
      return (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4 text-center">
            <div className="flex h-12 w-12 items-center justify-center rounded-full bg-red-100">
              <svg
                className="h-6 w-6 text-red-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                />
              </svg>
            </div>
            <div className="space-y-2">
              <p className="font-medium text-gray-900">Fehler beim Laden</p>
              <p className="max-w-xs text-sm text-gray-600">{error}</p>
            </div>
            <button
              onClick={onClose}
              className="mt-2 rounded-lg bg-gray-100 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200"
            >
              Schließen
            </button>
          </div>
        </div>
      );
    }

    if (loading) {
      return <ModalLoadingState accentColor="blue" />;
    }

    if (showDeleteConfirm) {
      return (
        <InlineDeleteConfirmation
          title="Schüler löschen?"
          onCancel={() => setShowDeleteConfirm(false)}
          onConfirm={() => {
            setShowDeleteConfirm(false);
            onDelete();
          }}
        >
          <p className="text-sm text-gray-700">
            Möchten Sie den Schüler{" "}
            <strong>
              {student.first_name} {student.second_name}
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
          <div className="flex h-14 w-14 flex-shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-[#5080D8] to-[#4070c8] text-lg font-bold text-white shadow-lg md:h-16 md:w-16 md:text-xl">
            {student.first_name?.[0]}
            {student.second_name?.[0]}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-lg font-bold text-gray-900 md:text-xl">
              {student.first_name} {student.second_name}
            </h2>
            {student.school_class && (
              <p className="mt-0.5 text-xs text-gray-500 md:text-sm">
                Klasse {student.school_class}
              </p>
            )}
          </div>
        </div>

        {/* Student Information Sections */}
        <div className="space-y-4">
          {/* Personal Information */}
          <InfoSection
            title="Persönliche Daten"
            icon={DetailIcons.person}
            accentColor="blue"
          >
            <DataGrid>
              <DataField label="Vorname">{student.first_name}</DataField>
              <DataField label="Nachname">{student.second_name}</DataField>
              <DataField label="Klasse">
                {student.school_class ?? "Nicht angegeben"}
              </DataField>
              <DataField label="Gruppe">
                {student.group_name ?? "Keine Gruppe"}
              </DataField>
              {student.id && (
                <DataField label="Schüler-ID" fullWidth mono>
                  {student.id}
                </DataField>
              )}
            </DataGrid>
          </InfoSection>

          {/* Guardian Information */}
          {guardians && guardians.length > 0 ? (
            <InfoSection
              title="Erziehungsberechtigte"
              icon={DetailIcons.group}
              accentColor="blue"
            >
              <div className="space-y-4">
                {guardians.map((guardian, index) => (
                  <div
                    key={guardian.id}
                    className={
                      index > 0 ? "border-t border-purple-100 pt-4" : ""
                    }
                  >
                    <div className="mb-2 text-xs font-semibold text-purple-700">
                      {guardian.relationship}{" "}
                      {guardians.length > 1 ? `${index + 1}` : ""}
                    </div>
                    <dl className="space-y-2">
                      {guardian.name && (
                        <DataField label="Name">{guardian.name}</DataField>
                      )}
                      {guardian.email && (
                        <DataField label="E-Mail">{guardian.email}</DataField>
                      )}
                      {guardian.phone && (
                        <DataField label="Telefon">{guardian.phone}</DataField>
                      )}
                      {guardian.contact && (
                        <DataField label="Zusätzliche Kontaktinfo">
                          {guardian.contact}
                        </DataField>
                      )}
                    </dl>
                  </div>
                ))}
              </div>
            </InfoSection>
          ) : (
            [
              student.name_lg,
              student.contact_lg,
              student.guardian_email,
              student.guardian_phone,
            ].some((v) => typeof v === "string" && v.length > 0) && (
              <InfoSection
                title="Erziehungsberechtigter"
                icon={DetailIcons.group}
                accentColor="blue"
              >
                <dl className="space-y-3">
                  {student.name_lg && (
                    <DataField label="Name">{student.name_lg}</DataField>
                  )}
                  {student.contact_lg && (
                    <DataField label="Kontakt">{student.contact_lg}</DataField>
                  )}
                  {student.guardian_email && (
                    <DataField label="E-Mail">
                      {student.guardian_email}
                    </DataField>
                  )}
                  {student.guardian_phone && (
                    <DataField label="Telefon">
                      {student.guardian_phone}
                    </DataField>
                  )}
                </dl>
              </InfoSection>
            )
          )}

          {/* Health Information */}
          {student.health_info && (
            <InfoSection
              title="Gesundheitsinformationen"
              icon={DetailIcons.heart}
              accentColor="blue"
            >
              <InfoText>{student.health_info}</InfoText>
            </InfoSection>
          )}

          {/* Supervisor Notes */}
          {student.supervisor_notes && (
            <InfoSection
              title="Betreuernotizen"
              icon={DetailIcons.notes}
              accentColor="blue"
            >
              <InfoText>{student.supervisor_notes}</InfoText>
            </InfoSection>
          )}

          {/* Additional Information parsed from structured extra_info */}
          {additionalInfo && additionalInfo.trim().length > 0 && (
            <InfoSection
              title="Elternnotizen"
              icon={DetailIcons.document}
              accentColor="blue"
            >
              <InfoText>{additionalInfo}</InfoText>
            </InfoSection>
          )}

          {/* Fallback: show raw extra_info only when it didn't contain guardians or structured notes */}
          {student.extra_info &&
            !guardians &&
            !(additionalInfo && additionalInfo.trim().length > 0) && (
              <InfoSection
                title="Elternnotizen"
                icon={DetailIcons.document}
                accentColor="blue"
              >
                <InfoText>{student.extra_info}</InfoText>
              </InfoSection>
            )}

          {/* Pickup Status */}
          {student.pickup_status && (
            <InfoSection
              title="Abholstatus"
              icon={DetailIcons.home}
              accentColor="green"
            >
              <p className="text-xs font-medium text-gray-900 md:text-sm">
                {student.pickup_status}
              </p>
            </InfoSection>
          )}

          {/* Privacy & Data Retention */}
          <InfoSection
            title="Datenschutz"
            icon={DetailIcons.lock}
            accentColor="blue"
          >
            <DataGrid>
              <DataField label="Einwilligung erteilt">
                {student.privacy_consent_accepted ? (
                  <span className="inline-flex items-center gap-1 text-green-700">
                    <span className="h-3.5 w-3.5 md:h-4 md:w-4">
                      {DetailIcons.check}
                    </span>
                    <span>Ja</span>
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 text-gray-900">
                    <span className="h-3.5 w-3.5 md:h-4 md:w-4">
                      {DetailIcons.x}
                    </span>
                    <span>Nein</span>
                  </span>
                )}
              </DataField>
              {student.data_retention_days && (
                <DataField label="Aufbewahrungsdauer">
                  {student.data_retention_days} Tage
                </DataField>
              )}
            </DataGrid>
          </InfoSection>

          {/* Bus Status */}
          {student.bus && (
            <div className="rounded-xl border border-orange-200 bg-orange-50 p-3 md:p-4">
              <div className="flex items-center gap-2">
                <span className="h-4 w-4 flex-shrink-0 text-orange-600 md:h-5 md:w-5">
                  {DetailIcons.bus}
                </span>
                <span className="text-xs font-medium text-orange-900 md:text-sm">
                  Fährt mit dem Bus
                </span>
              </div>
            </div>
          )}
        </div>

        <DetailModalActions
          onEdit={onEdit}
          onDelete={onDelete}
          onDeleteClick={() => setShowDeleteConfirm(true)}
          entityName={`${student.first_name} ${student.second_name}`}
          entityType="Schüler"
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
