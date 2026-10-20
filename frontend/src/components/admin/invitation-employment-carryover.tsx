import { Checkbox } from "~/components/ui/checkbox";
import type { CreateInvitationRequest } from "~/lib/invitation-helpers";

type ResetKey = "resetMasterData" | "resetQualifications" | "resetWorkTime";
const AREAS: readonly {
  key: ResetKey;
  label: string;
  retainedDescription: string;
  resetDescription: string;
}[] = [
  {
    key: "resetMasterData",
    label: "Stammdaten",
    retainedDescription: "Stammdaten und Personalnummer bleiben erhalten.",
    resetDescription: "Stammdaten und Personalnummer werden zurückgesetzt.",
  },
  {
    key: "resetQualifications",
    label: "Qualifikationen",
    retainedDescription: "Bisherige Qualifikationen bleiben erhalten.",
    resetDescription: "Bisherige Qualifikationen werden zurückgesetzt.",
  },
  {
    key: "resetWorkTime",
    label: "Arbeitszeit",
    retainedDescription:
      "Stundenkonto, Urlaub und Arbeitszeitmodell bleiben erhalten.",
    resetDescription:
      "Stundenkonto, Urlaub und Arbeitszeitmodell werden zusammen zurückgesetzt.",
  },
];

export function EmploymentCarryover({
  value,
  onChange,
  disabled,
}: {
  readonly value: CreateInvitationRequest;
  readonly onChange: (key: ResetKey, reset: boolean) => void;
  readonly disabled: boolean;
}) {
  return (
    <fieldset className="space-y-2" disabled={disabled}>
      <legend className="font-semibold">Diese Daten übernehmen</legend>
      <p>Entfernen Sie ein Häkchen, um den Bereich zurückzusetzen.</p>
      {AREAS.map(({ key, label, retainedDescription, resetDescription }) => (
        <label
          key={key}
          htmlFor={`invitation-${key}`}
          className="flex items-start gap-2"
        >
          <Checkbox
            id={`invitation-${key}`}
            checked={!value[key]}
            onChange={(event) => onChange(key, !event.target.checked)}
          />
          <span>
            <span className="font-medium">{label}</span>
            <span className="block text-sm">
              {value[key] ? resetDescription : retainedDescription}
            </span>
          </span>
        </label>
      ))}
      <p>Frühere Angaben bleiben als Historie erhalten.</p>
      {value.resetWorkTime ? (
        <p>
          Der Urlaub startet mit dem vollen Jahresanspruch ohne Übertrag. Sie
          können den Jahresanspruch danach anpassen.
        </p>
      ) : null}
      <p>Vergangene Schichten und Abwesenheiten bleiben erhalten.</p>
    </fieldset>
  );
}
