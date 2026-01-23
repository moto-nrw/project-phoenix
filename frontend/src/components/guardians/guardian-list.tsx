"use client";

import type {
  GuardianWithRelationship,
  PhoneNumber,
} from "@/lib/guardian-helpers";
import {
  getGuardianFullName,
  getRelationshipTypeLabel,
  PHONE_TYPE_LABELS,
} from "@/lib/guardian-helpers";
import { ModernContactActions } from "~/components/simple/student";
import {
  Trash2,
  Edit,
  UserCheck,
  Phone,
  AlertCircle,
  Mail,
} from "lucide-react";

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
            <EmailItem
              email={guardian.email}
              guardianName={getGuardianFullName(guardian)}
            />
            {/* Display flexible phone numbers */}
            {guardian.phoneNumbers && guardian.phoneNumbers.length > 0 ? (
              guardian.phoneNumbers.map((phone) => (
                <PhoneItem key={phone.id} phone={phone} />
              ))
            ) : (
              <InfoItem
                label="Telefon"
                value="Nicht angegeben"
                icon={<Phone className="h-4 w-4" />}
              />
            )}
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
              phone={getPrimaryPhone(guardian)}
              phoneNumbers={
                guardian.phoneNumbers?.map((p) => ({
                  number: p.phoneNumber,
                  label: getPhoneLabel(p),
                  isPrimary: p.isPrimary,
                })) ?? []
              }
              studentName={getGuardianFullName(guardian)}
            />
          </div>
        </div>
      ))}
    </div>
  );
}

// Helper component for displaying email (clickable)
function EmailItem({
  email,
  guardianName,
}: Readonly<{ email?: string; guardianName: string }>) {
  if (!email) {
    return (
      <div className="min-w-0">
        <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
          <Mail className="h-4 w-4" />
          <span>E-Mail</span>
        </div>
        <p className="text-sm font-medium text-gray-900">Nicht angegeben</p>
      </div>
    );
  }

  const subject = `Betreff: ${guardianName}`;
  const mailtoUrl = `mailto:${email}?subject=${encodeURIComponent(subject)}`;

  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
        <Mail className="h-4 w-4" />
        <span>E-Mail</span>
      </div>
      <a
        href={mailtoUrl}
        className="text-sm font-medium break-words text-gray-900 underline decoration-gray-300 underline-offset-2 transition-colors hover:text-blue-600 hover:decoration-blue-600"
      >
        {email}
      </a>
    </div>
  );
}

// Helper to get primary phone number for contact actions
function getPrimaryPhone(
  guardian: GuardianWithRelationship,
): string | undefined {
  if (guardian.phoneNumbers && guardian.phoneNumbers.length > 0) {
    // Prefer primary phone, otherwise first phone
    const primary = guardian.phoneNumbers.find((p) => p.isPrimary);
    return primary?.phoneNumber ?? guardian.phoneNumbers[0]?.phoneNumber;
  }
  return undefined;
}

// Helper to get phone label with type and custom label
function getPhoneLabel(phone: PhoneNumber): string {
  const typeLabel = PHONE_TYPE_LABELS[phone.phoneType] ?? phone.phoneType;
  if (phone.label) {
    return `${typeLabel} (${phone.label})`;
  }
  return typeLabel;
}

// Helper component for displaying phone numbers (clickable)
function PhoneItem({ phone }: Readonly<{ phone: PhoneNumber }>) {
  const cleanPhone = phone.phoneNumber.replaceAll(/\s+/g, "");

  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
        <Phone className="h-4 w-4" />
        <span>{getPhoneLabel(phone)}</span>
        {phone.isPrimary && (
          <span className="ml-1 rounded bg-purple-100 px-1 py-0.5 text-[10px] font-medium text-purple-700">
            Primär
          </span>
        )}
      </div>
      <a
        href={`tel:${cleanPhone}`}
        className="text-sm font-medium break-words text-gray-900 underline decoration-gray-300 underline-offset-2 transition-colors hover:text-blue-600 hover:decoration-blue-600"
      >
        {phone.phoneNumber}
      </a>
    </div>
  );
}

// Helper component for displaying information items
function InfoItem({
  label,
  value,
  icon,
}: Readonly<{
  label: string;
  value: string;
  icon?: React.ReactNode;
}>) {
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
