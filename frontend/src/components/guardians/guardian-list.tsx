"use client";

import type {
  GuardianAccountStatus,
  GuardianWithRelationship,
  PhoneNumber,
} from "@/lib/guardian-helpers";
import {
  getGuardianFullName,
  getRelationshipTypeLabel,
  getLanguageLabel,
  PHONE_TYPE_LABELS,
} from "@/lib/guardian-helpers";
import {
  GuardianContactActions,
  buildGuardianMailtoHref,
  buildGuardianTelHref,
} from "./guardian-contact-actions";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  Phone,
  AlertCircle,
  Mail,
  MapPin,
  Shield,
  Globe,
  Send,
} from "lucide-react";

// Account-status pill: a brand-colored dot + neutral label, matching the
// app's connection-indicator idiom. Routes color through LOCATION_COLORS.
const ACCOUNT_STATUS_META: Record<
  GuardianAccountStatus,
  { label: string; dot: string }
> = {
  active: { label: "Konto aktiv", dot: LOCATION_COLORS.GROUP_ROOM },
  active_no_access: {
    label: "Konto aktiv, kein Portalzugriff",
    dot: LOCATION_COLORS.SCHOOLYARD,
  },
  pending: { label: "Einladung offen", dot: LOCATION_COLORS.SICK },
  none: { label: "Kein Konto", dot: LOCATION_COLORS.UNKNOWN },
};

// Label + tooltip for the invite action per account status. "active_no_access"
// reuses the invite flow: the backend answers with the restricted-contact
// confirmation, so the action reads as granting access, not re-inviting.
const INVITE_ACTION_META: Record<
  Exclude<GuardianAccountStatus, "active">,
  { label: string; title: string }
> = {
  active_no_access: {
    label: "Zugriff gewähren",
    title: "Vollen Zugriff auf dieses Kind im Elternportal gewähren",
  },
  pending: { label: "Erneut einladen", title: "Einladung erneut senden" },
  none: { label: "Einladen", title: "Zum Elternportal einladen" },
};

function AccountStatusBadge({
  status,
}: Readonly<{ status?: GuardianAccountStatus }>) {
  const meta = ACCOUNT_STATUS_META[status ?? "none"];
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
      <span
        className="h-2 w-2 rounded-full"
        style={{ backgroundColor: meta.dot }}
      />
      {meta.label}
    </span>
  );
}

interface GuardianListProps {
  readonly guardians: ReadonlyArray<GuardianWithRelationship>;
  readonly onEdit?: (guardian: GuardianWithRelationship) => void;
  readonly onInvite?: (guardian: GuardianWithRelationship) => void;
  readonly invitingGuardianId?: string | null;
  readonly readOnly?: boolean;
  readonly showRelationship?: boolean;
}

export default function GuardianList({
  guardians,
  onEdit,
  onInvite,
  invitingGuardianId,
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
                  <span className="inline-flex items-center rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-700">
                    Primär
                  </span>
                )}
                <AccountStatusBadge status={guardian.accountStatus} />
              </h4>
              {showRelationship && (
                <p className="mt-1 text-xs text-gray-600 sm:text-sm">
                  {getRelationshipTypeLabel(guardian.relationshipType)}
                </p>
              )}
            </div>

            {!readOnly && (
              <div className="flex flex-shrink-0 items-center gap-1">
                {/* Social-worker contacts are school-managed: the backend
                    refuses to invite or upgrade them, so no action is shown.
                    A pending entry for an ACCOUNT holder is a role-upgrade
                    request awaiting approval in the queue — there is no email
                    to re-send and acting here would bypass the queue. */}
                {onInvite &&
                  guardian.accountStatus !== "active" &&
                  guardian.guardianRole !== "social_worker" &&
                  !(
                    guardian.accountStatus === "pending" && guardian.hasAccount
                  ) &&
                  guardian.email && (
                    <button
                      type="button"
                      onClick={() => onInvite(guardian)}
                      disabled={invitingGuardianId === guardian.id}
                      className="text-moto-green-strong hover:bg-moto-green/10 inline-flex items-center gap-1 rounded-lg px-2.5 py-1.5 text-sm font-medium transition-colors disabled:opacity-50"
                      title={
                        INVITE_ACTION_META[guardian.accountStatus ?? "none"]
                          .title
                      }
                    >
                      <Send className="h-4 w-4" />
                      <span className="hidden sm:inline">
                        {
                          INVITE_ACTION_META[guardian.accountStatus ?? "none"]
                            .label
                        }
                      </span>
                    </button>
                  )}
                {onEdit && (
                  <button
                    type="button"
                    onClick={() => onEdit(guardian)}
                    className="rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-100"
                  >
                    Bearbeiten
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

          {/* Address — use explicit check because nullish coalescing treats "" as present */}
          {[
            guardian.addressStreet,
            guardian.addressCity,
            guardian.addressPostalCode,
          ].some((v) => v && v.trim() !== "") && (
            <div className="mt-2 border-t border-gray-100 pt-2 sm:mt-3 sm:pt-3">
              <div className="flex items-start gap-1.5 text-xs text-gray-500 sm:text-sm">
                <MapPin className="mt-0.5 h-3.5 w-3.5 flex-shrink-0" />
                <span className="text-gray-700">
                  {[
                    guardian.addressStreet,
                    [guardian.addressPostalCode, guardian.addressCity]
                      .filter(Boolean)
                      .join(" "),
                  ]
                    .filter(Boolean)
                    .join(", ")}
                </span>
              </div>
            </div>
          )}

          {/* Badges: Flags and Language */}
          <div className="mt-2 flex flex-wrap gap-1.5 border-t border-gray-100 pt-2 sm:mt-3 sm:pt-3">
            {guardian.canPickup && (
              <span className="bg-moto-green/15 text-moto-green-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
                <Shield className="h-3 w-3" />
                Abholberechtigt
              </span>
            )}
            {guardian.isEmergencyContact && (
              <span className="bg-moto-red/10 text-moto-red-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
                <AlertCircle className="h-3 w-3" />
                Notfallkontakt
              </span>
            )}
            {guardian.languagePreference && (
              <span className="bg-moto-blue/15 text-moto-blue-strong inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-medium">
                <Globe className="h-3 w-3" />
                {getLanguageLabel(guardian.languagePreference)}
              </span>
            )}
          </div>

          {/* Notes */}
          {guardian.notes && (
            <div className="mt-2 rounded-lg bg-gray-50 px-3 py-2 text-xs text-gray-600 sm:text-sm">
              {guardian.notes}
            </div>
          )}

          {/* Contact Actions */}
          <div className="mt-2 sm:mt-3">
            <GuardianContactActions
              email={guardian.email}
              phone={getPrimaryPhone(guardian)}
              phoneNumbers={
                guardian.phoneNumbers?.map((p) => ({
                  number: p.phoneNumber,
                  label: getPhoneLabel(p),
                  isPrimary: p.isPrimary,
                })) ?? []
              }
              contactName={getGuardianFullName(guardian)}
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

  const mailtoUrl = buildGuardianMailtoHref(email, guardianName);

  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
        <Mail className="h-4 w-4" />
        <span>E-Mail</span>
      </div>
      <a
        href={mailtoUrl}
        className="hover:text-moto-blue-strong hover:decoration-moto-blue-strong text-sm font-medium break-words text-gray-900 underline decoration-gray-300 underline-offset-2 transition-colors"
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
  return (
    <div className="min-w-0">
      <div className="mb-1 flex items-center gap-1 text-xs text-gray-500">
        <Phone className="h-4 w-4" />
        <span>{getPhoneLabel(phone)}</span>
        {phone.isPrimary && (
          <span className="ml-1 rounded bg-gray-100 px-1 py-0.5 text-[10px] font-medium text-gray-700">
            Primär
          </span>
        )}
      </div>
      <a
        href={buildGuardianTelHref(phone.phoneNumber)}
        className="hover:text-moto-blue-strong hover:decoration-moto-blue-strong text-sm font-medium break-words text-gray-900 underline decoration-gray-300 underline-offset-2 transition-colors"
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
