"use client";

import type { GuardianWithRelationship } from "@/lib/guardian-helpers";
import {
  getGuardianFullName,
  getRelationshipTypeLabel,
} from "@/lib/guardian-helpers";
import { ModernContactActions } from "~/components/simple/student";
import { Trash2, Edit, UserCheck, Phone, AlertCircle } from "lucide-react";

interface GuardianListProps {
  readonly guardians: ReadonlyArray<GuardianWithRelationship>;
  readonly onEdit?: (guardian: GuardianWithRelationship) => void;
  readonly onDelete?: (guardian: GuardianWithRelationship) => void;
  readonly readOnly?: boolean;
  readonly showRelationship?: boolean;
}

export default function GuardianList({
  guardians,
  onEdit,
  onDelete,
  readOnly = false,
  showRelationship = true,
}: GuardianListProps) {
  if (guardians.length === 0) {
    return (
      <div className="py-6 text-center text-gray-500 sm:py-8">
        <AlertCircle className="mx-auto mb-2 h-10 w-10 text-gray-400 sm:h-12 sm:w-12" />
        <p className="text-sm sm:text-base">
          Keine Erziehungsberechtigten zugewiesen
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {guardians.map((guardian) => (
        <div
          key={guardian.id}
          className="rounded-lg border border-gray-200 bg-white p-3 transition-colors sm:p-4"
        >
          {/* Header with name and actions */}
          <div className="mb-3 flex items-start justify-between gap-2">
            <div className="min-w-0 flex-1">
              <h4 className="flex flex-wrap items-center gap-2 text-base font-semibold sm:text-lg">
                <span className="break-words">
                  {getGuardianFullName(guardian)}
                </span>
                {guardian.isPrimary && (
                  <span className="inline-flex items-center gap-1 rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-800">
                    <UserCheck className="h-3 w-3" />
                    Primär
                  </span>
                )}
              </h4>
              {showRelationship && (
                <p className="mt-1 text-xs text-gray-600 sm:text-sm">
                  {getRelationshipTypeLabel(guardian.relationshipType)}
                </p>
              )}
            </div>

            {!readOnly && (
              <div className="flex flex-shrink-0 gap-1 sm:gap-2">
                {onEdit && (
                  <button
                    onClick={() => onEdit(guardian)}
                    className="rounded-lg p-1.5 text-blue-600 transition-colors hover:bg-blue-50 sm:p-2"
                    title="Bearbeiten"
                  >
                    <Edit className="h-4 w-4" />
                  </button>
                )}
                {onDelete && (
                  <button
                    onClick={() => onDelete(guardian)}
                    className="rounded-lg p-1.5 text-red-600 transition-colors hover:bg-red-50 sm:p-2"
                    title="Entfernen"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                )}
              </div>
            )}
          </div>

          {/* Contact Information */}
          <div className="grid grid-cols-1 gap-2 sm:gap-3 md:grid-cols-2">
            <InfoItem
              label="E-Mail"
              value={guardian.email ?? "Nicht angegeben"}
              icon={
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
                    d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"
                  />
                </svg>
              }
            />
            <InfoItem
              label="Telefon"
              value={guardian.phone ?? "Nicht angegeben"}
              icon={<Phone className="h-4 w-4" />}
            />
            <InfoItem
              label="Mobiltelefon"
              value={guardian.mobilePhone ?? "Nicht angegeben"}
              icon={<Phone className="h-4 w-4" />}
            />
          </div>

          {/* Additional Information */}
          {showRelationship && guardian.isEmergencyContact && (
            <div className="mt-2 border-t border-gray-200 pt-2 text-xs sm:mt-3 sm:pt-3 sm:text-sm">
              <span className="inline-flex items-center gap-1 text-red-600">
                <AlertCircle className="h-3 w-3" />
                Notfallkontakt
              </span>
            </div>
          )}

          {/* Contact Actions */}
          <div className="mt-2 sm:mt-3">
            <ModernContactActions
              email={guardian.email}
              phone={guardian.phone ?? guardian.mobilePhone}
              studentName={getGuardianFullName(guardian)}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

// Helper component for displaying information items
function InfoItem({
  label,
  value,
  icon,
}: {
  label: string;
  value: string;
  icon?: React.ReactNode;
}) {
  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
        {icon}
        <span>{label}</span>
      </div>
      <p className="text-sm font-medium break-words text-gray-900">{value}</p>
    </div>
  );
}
