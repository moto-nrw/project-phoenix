"use client";

import { useState, useRef, useEffect } from "react";
import { ChevronDown } from "lucide-react";

interface PhoneOption {
  number: string;
  label: string;
  isPrimary?: boolean;
}

interface ModernContactActionsProps {
  readonly email?: string;
  readonly phone?: string;
  readonly phoneNumbers?: PhoneOption[];
  readonly studentName?: string;
}

export function ModernContactActions({
  email,
  phone,
  phoneNumbers = [],
  studentName,
}: ModernContactActionsProps) {
  const [isPhoneDropdownOpen, setIsPhoneDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsPhoneDropdownOpen(false);
      }
    }

    if (isPhoneDropdownOpen) {
      document.addEventListener("mousedown", handleClickOutside);
      return () =>
        document.removeEventListener("mousedown", handleClickOutside);
    }
  }, [isPhoneDropdownOpen]);

  const handleEmailClick = () => {
    if (email) {
      const subject = studentName
        ? `Betreff: ${studentName}`
        : "Kontaktanfrage";
      globalThis.location.href = `mailto:${email}?subject=${encodeURIComponent(subject)}`;
    }
  };

  const handlePhoneClick = (phoneNumber: string) => {
    const cleanPhone = phoneNumber.replaceAll(/\s+/g, "");
    globalThis.location.href = `tel:${cleanPhone}`;
    setIsPhoneDropdownOpen(false);
  };

  // Determine if we have multiple phone numbers
  const hasMultiplePhones = phoneNumbers.length > 1;
  const hasAnyPhone = phone ?? phoneNumbers.length > 0;

  // If no contact methods available, don't render anything
  if (!email && !hasAnyPhone) {
    return null;
  }

  return (
    <div className="mt-3 border-t border-gray-100 pt-3">
      <p className="mb-2 text-xs text-gray-500">Kontakt aufnehmen</p>

      <div className="flex flex-wrap gap-2">
        {email && (
          <button
            onClick={handleEmailClick}
            className="inline-flex items-center gap-1.5 rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-all duration-200 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 sm:hover:scale-105"
          >
            <svg
              className="h-3.5 w-3.5"
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
            E-Mail
          </button>
        )}

        {hasAnyPhone && (
          <div
            className={`relative ${isPhoneDropdownOpen ? "z-50" : ""}`}
            ref={dropdownRef}
          >
            {hasMultiplePhones ? (
              // Dropdown button for multiple phone numbers
              <>
                <button
                  onClick={() => setIsPhoneDropdownOpen(!isPhoneDropdownOpen)}
                  className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 sm:hover:scale-105"
                >
                  <svg
                    className="h-3.5 w-3.5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
                    />
                  </svg>
                  Anrufen
                  <ChevronDown
                    className={`h-3 w-3 transition-transform ${isPhoneDropdownOpen ? "rotate-180" : ""}`}
                  />
                </button>

                {/* Dropdown menu */}
                {isPhoneDropdownOpen && (
                  <div className="absolute left-0 z-50 mt-1 min-w-[200px] rounded-lg border border-gray-200 bg-white py-1 shadow-lg">
                    {phoneNumbers.map((phoneOption) => (
                      <button
                        key={phoneOption.number}
                        onClick={() => handlePhoneClick(phoneOption.number)}
                        className="flex w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50"
                      >
                        <svg
                          className="h-4 w-4 flex-shrink-0 text-gray-400"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
                          />
                        </svg>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1">
                            <span className="text-xs text-gray-500">
                              {phoneOption.label}
                            </span>
                            {phoneOption.isPrimary && (
                              <span className="rounded bg-purple-100 px-1 py-0.5 text-[10px] font-medium text-purple-700">
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
              // Single phone button (no dropdown)
              <button
                onClick={() =>
                  handlePhoneClick(phone ?? phoneNumbers[0]?.number ?? "")
                }
                className="inline-flex items-center gap-1.5 rounded-lg bg-gray-900 px-3 py-1.5 text-xs font-medium text-white transition-all duration-200 hover:bg-gray-700 hover:shadow-lg active:scale-100 sm:hover:scale-105"
              >
                <svg
                  className="h-3.5 w-3.5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z"
                  />
                </svg>
                Anrufen
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
