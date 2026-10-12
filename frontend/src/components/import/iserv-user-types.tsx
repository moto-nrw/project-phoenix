"use client";

import { Users } from "lucide-react";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { SectionCard } from "~/components/ui/section-card";

/** One "Benutzertyp" of an IServ user export, as the preview lists it. */
interface IServUserType {
  value: string;
  count: number;
  selected: boolean;
}

/** What the preview reports about an IServ user export (#1446). */
export interface IServSummary {
  user_types: IServUserType[];
  deactivated_count: number;
}

/** The chosen user types, as the import form field sends them. */
export function selectedIServUserTypes(summary: IServSummary): string[] {
  return summary.user_types
    .filter((userType) => userType.selected)
    .map((userType) => userType.value);
}

function personCount(count: number): string {
  return count === 1 ? "1 Person" : `${count} Personen`;
}

interface IServUserTypesProps {
  summary: IServSummary;
  disabled?: boolean;
  onChange: (selected: string[]) => void;
}

/**
 * Lets the person pick which IServ user types are children. IServ lets every
 * school name its user types itself, so moto only preselects the types that
 * read like pupils.
 */
export function IServUserTypes({
  summary,
  disabled = false,
  onChange,
}: Readonly<IServUserTypesProps>) {
  const selected = selectedIServUserTypes(summary);

  const toggle = (value: string) => {
    onChange(
      selected.includes(value)
        ? selected.filter((entry) => entry !== value)
        : [...selected, value],
    );
  };

  return (
    <SectionCard
      title="Welche iServ-Benutzer sind Kinder?"
      icon={Users}
      description="Die Datei kommt aus iServ. Wählen Sie, wer als Kind importiert wird."
    >
      <div className="grid gap-2 sm:grid-cols-2">
        {summary.user_types.map((userType) => (
          <ChoiceTile
            key={userType.value}
            selected={userType.selected}
            disabled={disabled}
            className="justify-between"
          >
            <span className="flex items-center gap-3">
              <Checkbox
                checked={userType.selected}
                disabled={disabled}
                onChange={() => toggle(userType.value)}
              />
              {userType.value === "" ? "Ohne Benutzertyp" : userType.value}
            </span>
            <span className="text-gray-500">{personCount(userType.count)}</span>
          </ChoiceTile>
        ))}
      </div>
      {selected.length === 0 ? (
        <p className="mt-3 text-sm text-gray-600">
          Noch nichts gewählt. Wählen Sie mindestens einen Benutzertyp.
        </p>
      ) : null}
      {summary.deactivated_count > 0 ? (
        <p className="mt-3 text-sm text-gray-500">
          {summary.deactivated_count === 1
            ? "1 gesperrtes iServ-Konto wird nicht übernommen."
            : `${summary.deactivated_count} gesperrte iServ-Konten werden nicht übernommen.`}
        </p>
      ) : null}
    </SectionCard>
  );
}
