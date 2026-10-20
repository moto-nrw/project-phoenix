import { Checkbox } from "~/components/ui/checkbox";
import type { CreateInvitationRequest } from "~/lib/invitation-helpers";

type ResetKey = "resetMasterData" | "resetQualifications" | "resetWorkTime";
const AREAS: readonly { key: ResetKey; label: string; description: string }[] =
  [
    {
      key: "resetMasterData",
      label: "Stammdaten",
      description: "Einschließlich Personalnummer.",
    },
    {
      key: "resetQualifications",
      label: "Qualifikationen",
      description: "Bisherige Qualifikationen gelten weiter.",
    },
    {
      key: "resetWorkTime",
      label: "Arbeitszeit",
      description:
        "Stundenkonto, Urlaub und Arbeitszeitmodell bleiben zusammen erhalten.",
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
      {AREAS.map(({ key, label, description }) => (
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
            <span className="block text-sm">{description}</span>
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
