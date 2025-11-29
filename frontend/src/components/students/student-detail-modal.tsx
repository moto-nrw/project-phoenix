"use client";

import { useState, useEffect } from "react";
import { Modal } from "~/components/ui/modal";
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
  isOpen: boolean;
  onClose: () => void;
  student: Student | null;
  onEdit: () => void;
  onDelete: () => void;
  loading?: boolean;
  error?: string | null;
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
  const { guardians, additionalInfo } = student
    ? parseExtraInfo(student)
    : { guardians: null, additionalInfo: null };

  // Reset confirmation state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setShowDeleteConfirm(false);
    }
  }, [isOpen]);

  if (!student) return null;

  const handleDeleteClick = () => {
    setShowDeleteConfirm(true);
  };

  const handleCancelDelete = () => {
    setShowDeleteConfirm(false);
  };

  const handleConfirmDelete = () => {
    setShowDeleteConfirm(false);
    onDelete();
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title="" // No title, we'll use custom header
    >
      {error ? (
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
      ) : loading ? (
        <div className="flex items-center justify-center py-12">
          <div className="flex flex-col items-center gap-4">
            <div className="h-12 w-12 animate-spin rounded-full border-2 border-gray-200 border-t-[#5080D8]"></div>
            <p className="text-gray-600">Daten werden geladen...</p>
          </div>
        </div>
      ) : showDeleteConfirm ? (
        /* Delete Confirmation View */
        <div className="space-y-6">
          {/* Warning Icon */}
          <div className="flex justify-center">
            <div className="flex h-16 w-16 items-center justify-center rounded-full bg-red-100">
              <svg
                className="h-8 w-8 text-red-600"
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
          </div>

          {/* Confirmation Message */}
          <div className="space-y-3 text-center">
            <h3 className="text-xl font-bold text-gray-900">
              Schüler löschen?
            </h3>
            <p className="text-sm text-gray-700">
              Möchten Sie den Schüler{" "}
              <strong>
                {student.first_name} {student.second_name}
              </strong>{" "}
              wirklich löschen?
            </p>
            <p className="text-sm font-medium text-red-600">
              Diese Aktion kann nicht rückgängig gemacht werden.
            </p>
          </div>

          {/* Action Buttons */}
          <div className="flex gap-3 border-t border-gray-100 pt-4">
            <button
              type="button"
              onClick={handleCancelDelete}
              className="flex-1 rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100"
            >
              Abbrechen
            </button>
            <button
              type="button"
              onClick={handleConfirmDelete}
              className="flex-1 rounded-lg bg-red-600 px-4 py-2 text-sm font-medium text-white transition-all duration-200 hover:scale-105 hover:bg-red-700 hover:shadow-lg active:scale-100"
            >
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
                Löschen
              </span>
            </button>
          </div>
        </div>
      ) : (
        /* Detail View */
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
            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
              <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
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
                    {student.first_name}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Nachname</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {student.second_name}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Klasse</dt>
                  <dd className="mt-0.5 text-sm font-medium text-gray-900">
                    {student.school_class ?? "Nicht angegeben"}
                  </dd>
                </div>
                <div>
                  <dt className="text-xs text-gray-500">Gruppe</dt>
                  <dd className="mt-0.5 text-sm font-medium break-words text-gray-900">
                    {student.group_name ?? "Keine Gruppe"}
                  </dd>
                </div>
                {student.id && (
                  <div className="col-span-1 sm:col-span-2">
                    <dt className="text-xs text-gray-500">Schüler-ID</dt>
                    <dd className="mt-0.5 font-mono text-xs break-all text-gray-600 md:text-sm">
                      {student.id}
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Guardian Information */}
            {guardians && guardians.length > 0 ? (
              <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-purple-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                    />
                  </svg>
                  Erziehungsberechtigte
                </h3>
                <div className="space-y-4">
                  {guardians.map((guardian, index) => (
                    <div
                      key={guardian.id}
                      className={`${index > 0 ? "border-t border-purple-100 pt-4" : ""}`}
                    >
                      <div className="mb-2 text-xs font-semibold text-purple-700">
                        {guardian.relationship}{" "}
                        {guardians.length > 1 ? `${index + 1}` : ""}
                      </div>
                      <dl className="space-y-2">
                        {guardian.name && (
                          <div>
                            <dt className="text-xs text-gray-500">Name</dt>
                            <dd className="mt-0.5 text-sm font-medium text-gray-900">
                              {guardian.name}
                            </dd>
                          </div>
                        )}
                        {guardian.email && (
                          <div>
                            <dt className="text-xs text-gray-500">E-Mail</dt>
                            <dd className="mt-0.5 text-sm font-medium text-gray-900">
                              {guardian.email}
                            </dd>
                          </div>
                        )}
                        {guardian.phone && (
                          <div>
                            <dt className="text-xs text-gray-500">Telefon</dt>
                            <dd className="mt-0.5 text-sm font-medium text-gray-900">
                              {guardian.phone}
                            </dd>
                          </div>
                        )}
                        {guardian.contact && (
                          <div>
                            <dt className="text-xs text-gray-500">
                              Zusätzliche Kontaktinfo
                            </dt>
                            <dd className="mt-0.5 text-sm font-medium text-gray-900">
                              {guardian.contact}
                            </dd>
                          </div>
                        )}
                      </dl>
                    </div>
                  ))}
                </div>
              </div>
            ) : (
              [
                student.name_lg,
                student.contact_lg,
                student.guardian_email,
                student.guardian_phone,
              ].some((v) => typeof v === "string" && v.length > 0) && (
                /* Legacy guardian display for backwards compatibility */
                <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                  <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                    <svg
                      className="h-3.5 w-3.5 text-purple-600 md:h-4 md:w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                      />
                    </svg>
                    Erziehungsberechtigter
                  </h3>
                  <dl className="space-y-3">
                    {student.name_lg && (
                      <div>
                        <dt className="text-xs text-gray-500">Name</dt>
                        <dd className="mt-0.5 text-sm font-medium text-gray-900">
                          {student.name_lg}
                        </dd>
                      </div>
                    )}
                    {student.contact_lg && (
                      <div>
                        <dt className="text-xs text-gray-500">Kontakt</dt>
                        <dd className="mt-0.5 text-sm font-medium text-gray-900">
                          {student.contact_lg}
                        </dd>
                      </div>
                    )}
                    {student.guardian_email && (
                      <div>
                        <dt className="text-xs text-gray-500">E-Mail</dt>
                        <dd className="mt-0.5 text-sm font-medium text-gray-900">
                          {student.guardian_email}
                        </dd>
                      </div>
                    )}
                    {student.guardian_phone && (
                      <div>
                        <dt className="text-xs text-gray-500">Telefon</dt>
                        <dd className="mt-0.5 text-sm font-medium text-gray-900">
                          {student.guardian_phone}
                        </dd>
                      </div>
                    )}
                  </dl>
                </div>
              )
            )}

            {/* Health Information */}
            {student.health_info && (
              <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-red-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"
                    />
                  </svg>
                  Gesundheitsinformationen
                </h3>
                <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                  {student.health_info}
                </p>
              </div>
            )}

            {/* Supervisor Notes */}
            {student.supervisor_notes && (
              <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-amber-600 md:h-4 md:w-4"
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
                  Betreuernotizen
                </h3>
                <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                  {student.supervisor_notes}
                </p>
              </div>
            )}

            {/* Additional Information parsed from structured extra_info */}
            {additionalInfo && additionalInfo.trim().length > 0 && (
              <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                  <svg
                    className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                    />
                  </svg>
                  Elternnotizen
                </h3>
                <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                  {additionalInfo}
                </p>
              </div>
            )}

            {/* Fallback: show raw extra_info only when it didn't contain guardians or structured notes */}
            {student.extra_info &&
              !guardians &&
              !(additionalInfo && additionalInfo.trim().length > 0) && (
                <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
                  <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                    <svg
                      className="h-3.5 w-3.5 text-blue-600 md:h-4 md:w-4"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                      />
                    </svg>
                    Elternnotizen
                  </h3>
                  <p className="text-xs break-words whitespace-pre-wrap text-gray-700 md:text-sm">
                    {student.extra_info}
                  </p>
                </div>
              )}

            {/* Privacy & Data Retention */}
            <div className="rounded-xl border border-gray-100 bg-blue-50/30 p-3 md:p-4">
              <h3 className="mb-2 flex items-center gap-2 text-xs font-semibold text-gray-900 md:mb-3 md:text-sm">
                <svg
                  className="h-3.5 w-3.5 text-gray-600 md:h-4 md:w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"
                  />
                </svg>
                Datenschutz
              </h3>
              <dl className="grid grid-cols-1 gap-x-3 gap-y-2 sm:grid-cols-2 md:gap-x-4 md:gap-y-3">
                <div>
                  <dt className="text-xs text-gray-500">
                    Einwilligung erteilt
                  </dt>
                  <dd className="mt-0.5 text-xs font-medium md:text-sm">
                    {student.privacy_consent_accepted ? (
                      <span className="inline-flex items-center gap-1 text-green-700">
                        <svg
                          className="h-3.5 w-3.5 md:h-4 md:w-4"
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
                        Ja
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 text-gray-900">
                        <svg
                          className="h-3.5 w-3.5 md:h-4 md:w-4"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                        Nein
                      </span>
                    )}
                  </dd>
                </div>
                {student.data_retention_days && (
                  <div>
                    <dt className="text-xs text-gray-500">
                      Aufbewahrungsdauer
                    </dt>
                    <dd className="mt-0.5 text-xs font-medium text-gray-900 md:text-sm">
                      {student.data_retention_days} Tage
                    </dd>
                  </div>
                )}
              </dl>
            </div>

            {/* Bus Status */}
            {student.bus && (
              <div className="rounded-xl border border-orange-200 bg-orange-50 p-3 md:p-4">
                <div className="flex items-center gap-2">
                  <svg
                    className="h-4 w-4 flex-shrink-0 text-orange-600 md:h-5 md:w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                    />
                  </svg>
                  <span className="text-xs font-medium text-orange-900 md:text-sm">
                    Fährt mit dem Bus
                  </span>
                </div>
              </div>
            )}
          </div>

          {/* Action Buttons */}
          <div className="sticky bottom-0 -mx-4 mt-4 -mb-4 flex gap-2 border-t border-gray-100 bg-white/95 px-4 py-3 backdrop-blur-sm md:-mx-6 md:mt-6 md:-mb-6 md:gap-3 md:px-6 md:py-4">
            <button
              type="button"
              onClick={handleDeleteClick}
              className="rounded-lg border border-red-300 px-3 py-2 text-xs font-medium text-red-700 transition-all duration-200 hover:border-red-400 hover:bg-red-50 hover:shadow-md active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
            >
              <span className="flex items-center gap-2">
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                  />
                </svg>
                Löschen
              </span>
            </button>

            <button
              type="button"
              onClick={onEdit}
              className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 md:px-4 md:text-sm md:hover:scale-105"
            >
              <span className="flex items-center justify-center gap-2">
                <svg
                  className="h-4 w-4"
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
                Bearbeiten
              </span>
            </button>
          </div>
        </div>
      )}
    </Modal>
  );
}
