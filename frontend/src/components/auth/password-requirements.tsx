import { Check, Circle } from "lucide-react";

export interface PasswordRequirementStatus {
  readonly label: string;
  readonly met: boolean;
}

/**
 * Die Passwortanforderungen unter einem neuen Passwort, je Regel mit Haken,
 * sobald sie erfüllt ist. Geteilt von den Einladungsformularen.
 */
export function PasswordRequirements({
  requirements,
}: {
  readonly requirements: readonly PasswordRequirementStatus[];
}) {
  return (
    <div className="rounded-lg border border-gray-200 bg-gray-50 p-3">
      <p className="mb-2 text-xs font-medium text-gray-700">
        Passwortanforderungen
      </p>
      <div className="grid grid-cols-2 gap-x-3 gap-y-1.5">
        {requirements.map((requirement) => (
          <div
            key={requirement.label}
            className="flex items-center gap-1.5 text-xs"
          >
            <span
              className={`flex h-4 w-4 flex-shrink-0 items-center justify-center rounded-full border ${
                requirement.met
                  ? "border-moto-green bg-moto-green/10 text-moto-green-strong"
                  : "border-gray-300 bg-white text-gray-400"
              }`}
              aria-hidden="true"
            >
              {requirement.met ? (
                <Check className="h-3 w-3" />
              ) : (
                <Circle className="h-3 w-3" />
              )}
            </span>
            <span
              className={requirement.met ? "text-gray-700" : "text-gray-500"}
            >
              {requirement.label}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
