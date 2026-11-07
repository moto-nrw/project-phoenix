"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { StatusColorBadge } from "~/components/ui/status-color-badge";
import { StatusBadge } from "~/components/ui/status-badge";
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
import type { GuardianEmailState } from "@/lib/guardian-api";
import {
  GUARDIAN_STOPPED_COPY,
  STOP_SHARED_NOTE,
  guardianAddressNextStep,
  mailStatusCopy,
} from "~/lib/email-delivery-status";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { displayEmail } from "~/lib/demo-build";
import {
  Phone,
  AlertCircle,
  Mail,
  MapPin,
  Pencil,
  Shield,
  Globe,
  Send,
  MailCheck,
  MailX,
} from "lucide-react";
import {
  OverflowMenu,
  type OverflowMenuEntry,
} from "~/components/ui/page-header/OverflowMenu";
import { InsetPanel } from "~/components/ui/inset-panel";

// Account-status pill: the tint carries the status (no dot, #2476). Routes
// color through LOCATION_COLORS.
const ACCOUNT_STATUS_META: Record<
  GuardianAccountStatus,
  { label: string; color: string }
> = {
  active: { label: "Konto aktiv", color: LOCATION_COLORS.GROUP_ROOM },
  active_no_access: {
    label: "Konto aktiv, kein Portalzugriff",
    color: LOCATION_COLORS.SCHOOLYARD,
  },
  pending: { label: "Einladung offen", color: LOCATION_COLORS.WARNING },
  none: { label: "Kein Konto", color: LOCATION_COLORS.UNKNOWN },
};

// Visible label for the invite action per account status. "active_no_access"
// reuses the invite flow: the backend answers with the restricted-contact
// confirmation, so the action reads as granting access, not re-inviting.
const INVITE_ACTION_LABEL: Record<
  Exclude<GuardianAccountStatus, "active">,
  string
> = {
  active_no_access: "Zugriff gewähren",
  pending: "Erneut einladen",
  none: "Einladen",
};

function getInviteActionLabel(status?: GuardianAccountStatus) {
  return INVITE_ACTION_LABEL[status === "active" ? "none" : (status ?? "none")];
}

function AccountStatusBadge({
  status,
}: Readonly<{ status?: GuardianAccountStatus }>) {
  const meta = ACCOUNT_STATUS_META[status ?? "none"];
  return (
    <StatusColorBadge label={meta.label} color={meta.color} compact wrap />
  );
}

interface GuardianListProps {
  readonly guardians: ReadonlyArray<GuardianWithRelationship>;
  readonly onEdit?: (guardian: GuardianWithRelationship) => void;
  readonly onInvite?: (guardian: GuardianWithRelationship) => void;
  readonly invitingGuardianId?: string | null;
  /** Mail state per guardian ID; a guardian without an entry shows none. */
  readonly emailStates?: Readonly<Record<string, GuardianEmailState>>;
  readonly onStopEmails?: (guardian: GuardianWithRelationship) => void;
  readonly onLiftEmailBlock?: (guardian: GuardianWithRelationship) => void;
  readonly onResumeEmails?: (guardian: GuardianWithRelationship) => void;
  readonly readOnly?: boolean;
  readonly showRelationship?: boolean;
}

function canInviteGuardian(
  guardian: GuardianWithRelationship,
  onInvite: GuardianListProps["onInvite"],
) {
  return Boolean(
    onInvite &&
    guardian.accountStatus !== "active" &&
    guardian.guardianRole !== "social_worker" &&
    !(guardian.accountStatus === "pending" && guardian.hasAccount) &&
    displayEmail(guardian.email),
  );
}

function inviteNeedsEmail(
  guardian: GuardianWithRelationship,
  onInvite: GuardianListProps["onInvite"],
) {
  return Boolean(
    onInvite &&
    guardian.accountStatus !== "active" &&
    guardian.guardianRole !== "social_worker" &&
    !(guardian.accountStatus === "pending" && guardian.hasAccount) &&
    !guardian.email,
  );
}

export default function GuardianList({
  guardians,
  onEdit,
  onInvite,
  invitingGuardianId,
  emailStates,
  onStopEmails,
  onLiftEmailBlock,
  onResumeEmails,
  readOnly = false,
  showRelationship = true,
}: GuardianListProps) {
  if (guardians.length === 0) {
    return (
      <EmptyState
        icon={<AlertCircle className="h-10 w-10 sm:h-12 sm:w-12" />}
        title="Keine Erziehungsberechtigten zugewiesen"
        className="py-6 sm:py-8"
      />
    );
  }

  // Alle Zeilenaktionen stehen im Kebab (BAUARTEN-SPEC Bauart 1 Regel 4).
  // Social-worker contacts are school-managed: the backend refuses to invite
  // or upgrade them, so no invite entry is offered. A pending entry for an
  // ACCOUNT holder is a role-upgrade request awaiting approval in the queue —
  // there is no email to re-send and acting here would bypass the queue.
  const buildGuardianMenu = (
    guardian: GuardianWithRelationship,
    showInvite: boolean,
  ): OverflowMenuEntry[] => {
    const items: OverflowMenuEntry[] = [];
    if (showInvite) {
      const isInviting = invitingGuardianId === guardian.id;
      items.push({
        label: isInviting
          ? "Wird eingeladen"
          : getInviteActionLabel(guardian.accountStatus),
        icon: <Send className="h-4 w-4" aria-hidden />,
        disabled: isInviting,
        onClick: () => onInvite?.(guardian),
      });
    }
    if (onEdit) {
      items.push({
        label: "Bearbeiten",
        icon: <Pencil className="h-4 w-4" aria-hidden />,
        onClick: () => onEdit(guardian),
      });
    }
    const mark = emailStates?.[guardian.id]?.addressMark;
    if (guardian.email && mark === "failed" && onLiftEmailBlock) {
      items.push({
        label: "Sperre aufheben",
        icon: <MailCheck className="h-4 w-4" aria-hidden />,
        onClick: () => onLiftEmailBlock(guardian),
      });
    }
    const stopped = emailStates?.[guardian.id]?.emailsStopped;
    if (stopped && onResumeEmails) {
      items.push({
        label: "Mails wieder senden",
        icon: <MailCheck className="h-4 w-4" aria-hidden />,
        onClick: () => onResumeEmails(guardian),
      });
    }
    if (guardian.email && !stopped && emailStates && onStopEmails) {
      items.push({
        label: "Mails stoppen",
        icon: <MailX className="h-4 w-4" aria-hidden />,
        onClick: () => onStopEmails(guardian),
      });
    }
    return items;
  };

  return (
    <div className="space-y-3">
      {guardians.map((guardian) => {
        const emailState = emailStates?.[guardian.id];
        const mailCopy = mailStatusCopy(
          emailState?.mailOutcome,
          emailState?.addressMark,
          false,
          emailState?.providerBlock,
        );
        const stopped = emailState?.emailsStopped === true;
        // A new invitation would change nothing: the note below says why.
        const showInvite =
          canInviteGuardian(guardian, onInvite) && !emailState?.sendBlock;
        const showMissingEmailHint = inviteNeedsEmail(guardian, onInvite);
        const menuItems = buildGuardianMenu(guardian, showInvite);

        return (
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
                    <StatusBadge label="Primär" tone="gray" compact />
                  )}
                  <AccountStatusBadge status={guardian.accountStatus} />
                  {mailCopy && (
                    <StatusBadge
                      label={mailCopy.label}
                      tone={mailCopy.tone}
                      compact
                    />
                  )}
                  {stopped && (
                    <StatusBadge
                      label={GUARDIAN_STOPPED_COPY.label}
                      tone={GUARDIAN_STOPPED_COPY.tone}
                      compact
                    />
                  )}
                </h4>
                {showRelationship && (
                  <p className="mt-1 text-xs text-gray-600 sm:text-sm">
                    {getRelationshipTypeLabel(guardian.relationshipType)}
                  </p>
                )}
              </div>

              {!readOnly && menuItems.length > 0 && (
                <div
                  className="flex flex-shrink-0 flex-wrap items-center justify-end gap-1"
                  data-setup-tour={showInvite ? "guardian-menu" : undefined}
                >
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${getGuardianFullName(guardian)}`}
                    items={menuItems}
                  />
                </div>
              )}
            </div>

            {showMissingEmailHint && !readOnly && (
              <p className="-mt-1 mb-3 text-sm text-gray-600">
                Für die Einladung fehlt die E-Mail-Adresse.
              </p>
            )}

            {mailCopy?.explanation && (
              <p className="-mt-1 mb-3 text-sm text-gray-600">
                {mailCopy.explanation}
                {emailState?.addressMark && !readOnly
                  ? ` ${guardianAddressNextStep(emailState.addressMark)}`
                  : null}
              </p>
            )}

            {stopped && (
              <p className="-mt-1 mb-3 text-sm text-gray-600">
                {GUARDIAN_STOPPED_COPY.explanation}
                {emailState?.stopShared ? ` ${STOP_SHARED_NOTE}` : null}
                {readOnly
                  ? null
                  : " Mit „Mails wieder senden“ heben Sie den Stopp auf."}
              </p>
            )}

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
                <StatusBadge
                  label={pickupBadgeLabel(guardian.pickupWeekdays)}
                  tone="green"
                  icon={Shield}
                  compact
                  wrap
                />
              )}
              {guardian.isEmergencyContact && (
                <StatusBadge
                  label="Notfallkontakt"
                  tone="red"
                  icon={AlertCircle}
                  compact
                />
              )}
              {guardian.languagePreference && (
                <StatusBadge
                  label={getLanguageLabel(guardian.languagePreference)}
                  tone="blue"
                  icon={Globe}
                  compact
                />
              )}
            </div>

            {/* Notes */}
            {guardian.notes && (
              <InsetPanel
                padding="compact"
                className="mt-2 text-xs text-gray-600 sm:text-sm"
              >
                {guardian.notes}
              </InsetPanel>
            )}

            {/* Contact Actions */}
            <div className="mt-2 sm:mt-3">
              <GuardianContactActions
                email={displayEmail(guardian.email)}
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
        );
      })}
    </div>
  );
}

// Helper component for displaying email (clickable); none for a demo seed address (#5033)
function EmailItem({
  email,
  guardianName,
}: Readonly<{ email?: string; guardianName: string }>) {
  if (!email || !displayEmail(email)) {
    return email ? null : (
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

const PICKUP_WEEKDAY_SHORT = ["", "Mo", "Di", "Mi", "Do", "Fr"] as const;

/** „Abholberechtigt“, bei Wochentagen mit den Tagen (#4780). */
function pickupBadgeLabel(weekdays: readonly number[] | undefined): string {
  if (!weekdays || weekdays.length === 0 || weekdays.length === 5) {
    return "Abholberechtigt";
  }
  const days = weekdays
    .map((day) => PICKUP_WEEKDAY_SHORT[day] ?? "")
    .filter(Boolean)
    .join(", ");
  return `Abholberechtigt · nur ${days}`;
}
