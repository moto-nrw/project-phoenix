"use client";

import { useCallback, useRef, useState } from "react";
import { ChevronDown, Mail, Phone } from "lucide-react";
import { Button } from "~/components/ui/button";
import { useClickOutside } from "~/lib/hooks/use-click-outside";

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
  const [isPhoneDropdownOpen, setIsPhoneDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const closePhoneDropdown = useCallback(
    () => setIsPhoneDropdownOpen(false),
    [],
  );
  useClickOutside(dropdownRef, closePhoneDropdown, isPhoneDropdownOpen);

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
    setIsPhoneDropdownOpen(false);
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
          >
            <Mail className="h-3.5 w-3.5" aria-hidden="true" />
            E-Mail
          </Button>
        )}

        {hasAnyPhone && (
          <div
            className={`relative ${isPhoneDropdownOpen ? "z-50" : ""}`}
            ref={dropdownRef}
          >
            {hasMultiplePhones ? (
              <>
                <Button
                  type="button"
                  variant="primary"
                  size="compact"
                  onClick={() => setIsPhoneDropdownOpen((isOpen) => !isOpen)}
                  aria-haspopup="menu"
                  aria-expanded={isPhoneDropdownOpen}
                >
                  <Phone className="h-3.5 w-3.5" aria-hidden="true" />
                  Anrufen
                  <ChevronDown
                    className={`h-3 w-3 transition-transform ${isPhoneDropdownOpen ? "rotate-180" : ""}`}
                    aria-hidden="true"
                  />
                </Button>

                {isPhoneDropdownOpen && (
                  <div
                    className="absolute left-0 z-50 mt-1 min-w-[200px] overflow-hidden rounded-lg border border-gray-200 bg-white py-1 shadow-lg"
                    role="menu"
                  >
                    {phoneNumbers.map((phoneOption) => (
                      <button
                        key={phoneOption.number}
                        type="button"
                        onClick={() => handlePhoneClick(phoneOption.number)}
                        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                        role="menuitem"
                      >
                        <Phone
                          className="h-4 w-4 flex-shrink-0 text-gray-400"
                          aria-hidden="true"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1">
                            <span className="text-xs text-gray-500">
                              {phoneOption.label}
                            </span>
                            {phoneOption.isPrimary && (
                              // Brand purple (LOCATION_COLORS.EXCUSED), not a
                              // generic Tailwind hue
                              <span className="rounded bg-[#7C3AED]/10 px-1 py-0.5 text-[10px] font-medium text-[#7C3AED]">
                                Primär
                              </span>
                            )}
                          </div>
                          <div className="truncate font-medium">
                            {phoneOption.number}
                          </div>
                        </div>
                      </button>
                    ))}
                  </div>
                )}
              </>
            ) : (
              <Button
                type="button"
                variant="primary"
                size="compact"
                onClick={() => handlePhoneClick(singlePhoneNumber ?? "")}
              >
                <Phone className="h-3.5 w-3.5" aria-hidden="true" />
                Anrufen
              </Button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
