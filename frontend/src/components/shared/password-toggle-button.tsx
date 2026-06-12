import { Eye, EyeOff } from "lucide-react";

interface PasswordToggleButtonProps {
  readonly showPassword: boolean;
  readonly onToggle: () => void;
  // Optional localized aria-labels. This component is shared across the German
  // staff/operator login pages (rendered pre-auth, WITHOUT a NextIntlClientProvider)
  // and the localized parents portal. It stays hook-free on purpose — calling
  // useTranslations here would crash the provider-less staff/operator pages. The
  // German defaults keep those callers unchanged; the parent callers, which run
  // under a provider, pass already-resolved localized strings.
  readonly showLabel?: string;
  readonly hideLabel?: string;
}

export function PasswordToggleButton({
  showPassword,
  onToggle,
  showLabel = "Passwort anzeigen",
  hideLabel = "Passwort verbergen",
}: PasswordToggleButtonProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute top-1/2 right-2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      aria-label={showPassword ? hideLabel : showLabel}
    >
      {showPassword ? (
        <EyeOff className="h-5 w-5" aria-hidden="true" />
      ) : (
        <Eye className="h-5 w-5" aria-hidden="true" />
      )}
    </button>
  );
}
