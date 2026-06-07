import { Eye, EyeOff } from "lucide-react";

interface PasswordToggleButtonProps {
  readonly showPassword: boolean;
  readonly onToggle: () => void;
}

export function PasswordToggleButton({
  showPassword,
  onToggle,
}: PasswordToggleButtonProps) {
  return (
    <button
      type="button"
      onClick={onToggle}
      className="absolute top-1/2 right-2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-gray-500 transition-colors hover:bg-gray-100 hover:text-gray-800 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
      aria-label={showPassword ? "Passwort verbergen" : "Passwort anzeigen"}
    >
      {showPassword ? (
        <EyeOff className="h-5 w-5" aria-hidden="true" />
      ) : (
        <Eye className="h-5 w-5" aria-hidden="true" />
      )}
    </button>
  );
}
