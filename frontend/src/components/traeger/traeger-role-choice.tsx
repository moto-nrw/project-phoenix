"use client";

import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import {
  TRAEGER_ROLES,
  TRAEGER_ROLE_LABELS,
  type TraegerRole,
} from "~/lib/traeger-roles";

/** What each Träger role may do, in one line (ADR 0084 section 4). */
const ROLE_DESCRIPTIONS: Readonly<Record<TraegerRole, string>> = {
  administration:
    "Darf alles verwalten, auch Personen, Einstellungen und Anmeldungen.",
  billing: "Darf Beiträge abrechnen und Finanz-Daten exportieren.",
  personnel: "Darf das Personal aller Schulen verwalten.",
  read_only: "Darf die Übersicht und die Berichte ansehen.",
};

/**
 * The Träger roles as checkboxes (#4650). A person may hold several; at
 * least one is required.
 */
export function TraegerRoleChoice({
  value,
  onChange,
  disabled = false,
  error,
}: Readonly<{
  value: readonly TraegerRole[];
  onChange: (roles: TraegerRole[]) => void;
  disabled?: boolean;
  error?: string;
}>) {
  const toggle = (role: TraegerRole) => {
    const next = value.includes(role)
      ? value.filter((held) => held !== role)
      : [...value, role];
    onChange(TRAEGER_ROLES.filter((known) => next.includes(known)));
  };

  return (
    <fieldset
      className="space-y-2 border-0 p-0"
      aria-describedby={error ? "traeger-roles-error" : undefined}
    >
      <legend className="mb-2 text-sm font-medium text-gray-700">
        Rolle (mindestens eine)
      </legend>
      {TRAEGER_ROLES.map((role) => (
        <ChoiceTile
          key={role}
          tone="green"
          selected={value.includes(role)}
          disabled={disabled}
          className="items-start gap-3 p-3"
        >
          <Checkbox
            name="roles"
            value={role}
            checked={value.includes(role)}
            disabled={disabled}
            aria-invalid={error ? true : undefined}
            onChange={() => toggle(role)}
          />
          <span className="flex flex-col">
            <span className="font-medium text-gray-900">
              {TRAEGER_ROLE_LABELS[role]}
            </span>
            <span className="text-sm text-gray-600">
              {ROLE_DESCRIPTIONS[role]}
            </span>
          </span>
        </ChoiceTile>
      ))}
      <FieldErrorHint id="traeger-roles-error" message={error} />
    </fieldset>
  );
}
