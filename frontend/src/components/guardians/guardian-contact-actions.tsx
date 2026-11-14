"use client";

import { ChevronDown, Mail, Phone } from "lucide-react";
import { Button } from "~/components/ui/button";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";

interface GuardianContactPhoneOption {
  readonly number: string;
  readonly label: string;
  readonly isPrimary?: boolean;
}

interface GuardianContactActionsProps {
  readonly email?: string;
  readonly phone?: string;
  readonly phoneNumbers?: readonly GuardianContactPhoneOption[];
  readonly contactName?: string;
}

const EMPTY_PHONE_NUMBERS: readonly GuardianContactPhoneOption[] = [];

export function buildGuardianMailtoHref(
  email: string,
  contactName?: string,
): string {
  const subject = contactName ? `Betreff: ${contactName}` : "Kontaktanfrage";
  return `mailto:${email}?subject=${encodeURIComponent(subject)}`;
}

export function buildGuardianTelHref(phoneNumber: string): string {
  const cleanPhone = phoneNumber.replaceAll(/\s+/g, "");
  return `tel:${cleanPhone}`;
}

export function GuardianContactActions({
  email,
  phone,
  phoneNumbers = EMPTY_PHONE_NUMBERS,
  contactName,
}: GuardianContactActionsProps) {
  const handleEmailClick = () => {
    if (!email) {
      return;
    }

    globalThis.location.href = buildGuardianMailtoHref(email, contactName);
  };

  const handlePhoneClick = (phoneNumber: string) => {
    if (!phoneNumber) {
      return;
    }

    globalThis.location.href = buildGuardianTelHref(phoneNumber);
  };

  const hasMultiplePhones = phoneNumbers.length > 1;
  const singlePhoneNumber = phone ?? phoneNumbers[0]?.number;
  const hasAnyPhone = Boolean(singlePhoneNumber) || phoneNumbers.length > 0;

  if (!email && !hasAnyPhone) {
    return null;
  }

  return (
    <div className="mt-3 border-t border-gray-100 pt-3">
      <p className="mb-2 text-xs text-gray-500">Kontakt aufnehmen</p>

      <div className="flex flex-wrap gap-2">
        {email && (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={handleEmailClick}
            tooltip="Öffnet Ihr Mailprogramm mit einer neuen Mail an diese Person."
          >
            <Mail className="h-3.5 w-3.5" aria-hidden="true" />
            E-Mail
          </Button>
        )}

        {hasAnyPhone &&
          (hasMultiplePhones ? (
            <OverflowMenu
              ariaLabel="Anrufen"
              triggerButton={{ variant: "primary", size: "compact" }}
              triggerContent={
                <>
                  <Phone className="h-3.5 w-3.5" aria-hidden="true" />
                  Anrufen
                  <ChevronDown className="h-3 w-3" aria-hidden="true" />
                </>
              }
              items={phoneNumbers.map((phoneOption) => ({
                label: phoneOption.number,
                description: phoneOption.isPrimary
                  ? `${phoneOption.label} · Primär`
                  : phoneOption.label,
                icon: <Phone className="size-4" aria-hidden="true" />,
                onClick: () => handlePhoneClick(phoneOption.number),
              }))}
            />
          ) : (
            <Button
              type="button"
              variant="primary"
              size="compact"
              onClick={() => handlePhoneClick(singlePhoneNumber ?? "")}
              tooltip="Öffnet Ihre Telefon-App mit der Nummer dieser Person."
            >
              <Phone className="h-3.5 w-3.5" aria-hidden="true" />
              Anrufen
            </Button>
          ))}
      </div>
    </div>
  );
}
