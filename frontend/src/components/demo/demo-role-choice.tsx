"use client";

import { useState } from "react";
import { DemoShell } from "~/components/demo/demo-shell";
import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Radio } from "~/components/ui/radio";
import {
  DEMO_ROLE_CHOICE_HINT,
  DEMO_ROLE_CHOICE_TITLE,
  DEMO_ROLES,
  type DemoRole,
} from "~/lib/demo-access";

export function DemoRoleChoice({
  onChoose,
}: {
  readonly onChoose: (role: DemoRole) => void;
}) {
  const [selectedRole, setSelectedRole] = useState<DemoRole | null>(null);
  return (
    <DemoShell
      title={DEMO_ROLE_CHOICE_TITLE}
      description={DEMO_ROLE_CHOICE_HINT}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (selectedRole) onChoose(selectedRole);
        }}
      >
        <fieldset className="flex min-w-0 flex-col gap-3">
          <legend className="sr-only">Rolle für die Demo</legend>
          {DEMO_ROLES.map((entry) => (
            <ChoiceTile
              key={entry.role}
              as="label"
              selected={selectedRole === entry.role}
              className="gap-3 p-4 focus-within:ring-0"
            >
              <Radio
                name="demo-role"
                value={entry.role}
                checked={selectedRole === entry.role}
                onChange={() => setSelectedRole(entry.role)}
                aria-labelledby={`demo-role-${entry.role}-label`}
                aria-describedby={`demo-role-${entry.role}-description`}
                required
              />
              <span className="min-w-0 space-y-1">
                <span
                  id={`demo-role-${entry.role}-label`}
                  className="block text-sm font-semibold text-gray-900"
                >
                  {entry.label}
                </span>
                <span
                  id={`demo-role-${entry.role}-description`}
                  className="block text-sm leading-5 font-normal text-gray-600"
                >
                  {entry.description}
                </span>
              </span>
            </ChoiceTile>
          ))}
        </fieldset>
        <div className="mt-6 space-y-3">
          <Button
            type="submit"
            size="md"
            className="h-10 w-full font-semibold shadow-sm hover:shadow-sm"
            disabled={!selectedRole}
          >
            Demo starten
          </Button>
          <p className="text-center text-xs leading-5 text-gray-500">
            Sie können die Rolle später wechseln.
          </p>
        </div>
      </form>
    </DemoShell>
  );
}
