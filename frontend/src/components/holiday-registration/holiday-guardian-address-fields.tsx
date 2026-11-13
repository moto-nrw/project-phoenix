"use client";

// Die Felder der Anschrift der Eltern (#5625): Straße und Hausnummer, PLZ und
// Ort, mit den Regeln des Formulars (Pflicht oder freiwillig, PLZ mit fünf
// Ziffern) und Fehlern am Feld. Das öffentliche Formular und die Bearbeitung
// durch das Träger-Büro (#5629) nutzen dieselben Felder; die Beschriftung
// bringt der Aufrufer mit, damit das Modul keine Übersetzungen lädt.

import { Input, type InputControlSize } from "~/components/ui/input";

/** The postal address as a form edits it: empty parts, never null. */
export interface GuardianAddressDraft {
  readonly street: string;
  readonly postalCode: string;
  readonly city: string;
}

/** The labels of the three parts. */
export interface GuardianAddressLabels {
  readonly street: string;
  readonly postalCode: string;
  readonly city: string;
}

export function GuardianAddressFields({
  address,
  required,
  labels,
  hints,
  fieldError,
  onChange,
  controlSize,
}: Readonly<{
  address: GuardianAddressDraft;
  required: boolean;
  labels: GuardianAddressLabels;
  /** A line under a part, such as where its value came from (#6682). */
  hints?: Partial<Record<keyof GuardianAddressDraft, string>>;
  fieldError: (name: string) => string | undefined;
  onChange: (address: GuardianAddressDraft) => void;
  /** The Träger-App sets its fields to 40 px; the parents' form keeps the default. */
  controlSize?: InputControlSize;
}>) {
  return (
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-[minmax(0,1fr)_8rem_minmax(0,1fr)]">
      <Input
        controlSize={controlSize}
        name="guardian_address.street"
        required={required}
        label={labels.street}
        autoComplete="street-address"
        maxLength={100}
        value={address.street}
        hint={hints?.street}
        error={fieldError("guardian_address.street")}
        onChange={(event) =>
          onChange({ ...address, street: event.target.value })
        }
      />
      <Input
        controlSize={controlSize}
        name="guardian_address.postal_code"
        required={required}
        label={labels.postalCode}
        autoComplete="postal-code"
        inputMode="numeric"
        maxLength={5}
        value={address.postalCode}
        hint={hints?.postalCode}
        error={fieldError("guardian_address.postal_code")}
        onChange={(event) =>
          onChange({ ...address, postalCode: event.target.value })
        }
      />
      <Input
        controlSize={controlSize}
        name="guardian_address.city"
        required={required}
        label={labels.city}
        autoComplete="address-level2"
        maxLength={100}
        value={address.city}
        hint={hints?.city}
        error={fieldError("guardian_address.city")}
        onChange={(event) => onChange({ ...address, city: event.target.value })}
      />
    </div>
  );
}
