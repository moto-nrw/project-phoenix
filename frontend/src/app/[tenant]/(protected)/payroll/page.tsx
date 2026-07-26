"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import useSWR from "swr";
import { Alert } from "~/components/ui/alert";
import { CustomSelect } from "~/components/ui/custom-select";
import { Input } from "~/components/ui/input";
import { Loading } from "~/components/ui/loading";
import { useRequirePermission } from "~/lib/hooks/use-require-permission";
import {
  duplicateLohnartNumbers,
  fetchPayrollStatus,
  type PayrollStatus,
} from "~/lib/payroll-api";
import { setSettingValue } from "~/lib/settings-api";

// Abrechnung (#1417 Tranche 2b): maintenance surface for the DATEV payroll
// foundation — Lohnart mapping and LODAS client identifiers. Values are
// settings-backed (config.setting_values, audited via config.setting_audit);
// this page is the hand-written panel over them, following the
// personalization-tab precedent instead of the auto-generated settings tab.
//
// There are deliberately NO defaults: Lohnartnummern are mandant-specific,
// and an invented preset would silently bill wrong. Empty is the honest
// starting state; the later DATEV writers refuse to export while required
// pieces are missing.

const UNIT_OPTIONS = [
  { value: "", label: "Nicht festgelegt" },
  { value: "stunden", label: "Stunden" },
  { value: "tage", label: "Tage" },
];

export default function PayrollPage() {
  const { isReady, isLoading: permissionLoading } = useRequirePermission([
    "config:manage",
  ]);

  const {
    data: status,
    error,
    mutate,
  } = useSWR(isReady ? "payroll-status" : null, fetchPayrollStatus);

  const [saveError, setSaveError] = useState<string | null>(null);
  const saveQueues = useRef(new Map<string, Promise<void>>());

  const save = useCallback(
    (key: string, value: string) => {
      const previous = saveQueues.current.get(key) ?? Promise.resolve();
      const next = previous
        .then(async () => {
          setSaveError(null);
          const message = await setSettingValue(key, value);
          if (message) {
            setSaveError(message);
          }
          await mutate();
        })
        .catch(() => {
          setSaveError("Einstellung konnte nicht gespeichert werden.");
        });

      saveQueues.current.set(key, next);
      void next.finally(() => {
        if (saveQueues.current.get(key) === next) {
          saveQueues.current.delete(key);
        }
      });
      return next;
    },
    [mutate],
  );

  if (permissionLoading || !isReady) {
    return <Loading fullPage />;
  }
  if (error) {
    return (
      <div className="mx-auto max-w-4xl px-4 py-8">
        <Alert
          type="error"
          message="Die Abrechnungs-Konfiguration konnte nicht geladen werden."
        />
      </div>
    );
  }
  if (!status) {
    return <Loading fullPage />;
  }

  const duplicates = duplicateLohnartNumbers(status);

  return (
    <div className="mx-auto max-w-4xl space-y-5 px-4 py-6">
      <div>
        <h1 className="text-xl font-bold text-gray-900">Abrechnung</h1>
        <p className="mt-1 text-sm text-gray-500">
          Zuordnung der Zeiterfassungs-Kategorien zu den Lohnarten des
          Lohnsystems und DATEV-Mandantendaten. Grundlage für den späteren
          DATEV-Export (LODAS und Lohn und Gehalt).
        </p>
      </div>

      <ReadinessCard status={status} />

      {saveError && <Alert type="error" message={saveError} />}
      {duplicates.length > 0 && (
        <Alert
          type="warning"
          message={`Lohnartnummer ${duplicates.join(", ")} ist mehreren Kategorien zugeordnet. Das ist zulässig, führt aber meist zu doppelt gebuchten Stunden — bitte prüfen.`}
        />
      )}

      <LohnartenCard status={status} onSave={save} />
      <DatevCard status={status} onSave={save} />
    </div>
  );
}

function ReadinessCard({ status }: { readonly status: PayrollStatus }) {
  const items: { label: string; ok: boolean; detail: string }[] = [
    {
      label: "Lohnarten",
      ok: status.configuredCategories > 0,
      detail: `${status.configuredCategories} von ${status.totalCategories} Kategorien konfiguriert`,
    },
    {
      label: "DATEV-Mandant (nur LODAS)",
      ok: status.lodasHeaderComplete,
      detail: status.lodasHeaderComplete
        ? "Berater- und Mandantennummer gesetzt"
        : "Berater- oder Mandantennummer fehlt",
    },
    {
      label: "Personalnummern",
      ok: status.staffWithoutPersonnelNumber === 0,
      detail:
        status.staffWithoutPersonnelNumber === 0
          ? `Alle ${status.staffTotal} Beschäftigten haben eine Personalnummer`
          : `${status.staffWithoutPersonnelNumber} von ${status.staffTotal} Beschäftigten ohne Personalnummer (Pflege im Stammdaten-Tab der Person)`,
    },
  ];

  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <h2 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
        Vollständigkeit
      </h2>
      <ul className="mt-3 space-y-2">
        {items.map((item) => (
          <li key={item.label} className="flex items-start gap-2 text-sm">
            <span
              aria-hidden
              className={`mt-0.5 inline-block h-2.5 w-2.5 shrink-0 rounded-full ${
                item.ok ? "bg-[#83CD2D]" : "bg-[#F78C10]"
              }`}
            />
            <span>
              <span className="font-medium text-gray-800">{item.label}:</span>{" "}
              <span className="text-gray-600">{item.detail}</span>
            </span>
          </li>
        ))}
      </ul>
      <p className="mt-3 text-xs text-gray-500">
        Ohne vollständige Konfiguration erzeugt der spätere DATEV-Export keine
        Datei. Eine Kategorie ohne Lohnartnummer wird nicht exportiert.
      </p>
    </div>
  );
}

function LohnartenCard({
  status,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly onSave: (key: string, value: string) => Promise<void>;
}) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <h2 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
        Lohnarten
      </h2>
      <p className="mt-1 text-xs text-gray-500">
        Mandantenspezifische Lohnartnummern aus dem Lohnsystem des Trägers (1
        bis 4 Ziffern). Für Krank, Urlaub und Fortbildung zusätzlich die
        Einheit, die die Lohnart erwartet.
      </p>
      <div className="mt-4 overflow-x-auto">
        <table className="w-full min-w-[28rem] text-sm">
          <thead>
            <tr className="border-b border-gray-200 text-left text-xs font-semibold tracking-wide text-gray-400 uppercase">
              <th className="py-2 pr-4">Kategorie</th>
              <th className="py-2 pr-4">Lohnartnummer</th>
              <th className="py-2">Einheit</th>
            </tr>
          </thead>
          <tbody>
            {status.categories.map((cat) => (
              <LohnartRow key={cat.id} cat={cat} onSave={onSave} />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function LohnartRow({
  cat,
  onSave,
}: {
  readonly cat: PayrollStatus["categories"][number];
  readonly onSave: (key: string, value: string) => Promise<void>;
}) {
  const [number, setNumber] = useState(cat.number);
  useEffect(() => setNumber(cat.number), [cat.number]);

  const valid = number.trim() === "" || /^\d{1,4}$/.test(number.trim());

  return (
    <tr className="border-b border-gray-100 last:border-0">
      <td className="py-2 pr-4 font-medium text-gray-800">{cat.label}</td>
      <td className="py-2 pr-4">
        <Input
          aria-label={`Lohnartnummer ${cat.label}`}
          value={number}
          onChange={(e) => setNumber(e.target.value)}
          onBlur={() => {
            const trimmed = number.trim();
            if (valid && trimmed !== cat.number) {
              void onSave(cat.settingKey, trimmed);
            }
          }}
          placeholder="Nicht konfiguriert"
          inputMode="numeric"
          className="h-9 max-w-[10rem] py-1 text-sm tabular-nums"
        />
        {!valid && (
          <p className="mt-1 text-xs text-[#FF3130]">1 bis 4 Ziffern.</p>
        )}
      </td>
      <td className="py-2">
        {cat.unitRequired && cat.unitSettingKey ? (
          <CustomSelect
            ariaLabel={`Einheit ${cat.label}`}
            value={cat.unit}
            options={UNIT_OPTIONS}
            onChange={(next) => {
              if (next !== cat.unit && cat.unitSettingKey) {
                void onSave(cat.unitSettingKey, next);
              }
            }}
            triggerClassName="moto-content-surface h-9 w-auto min-w-[10rem] hover:border-gray-300"
          />
        ) : (
          <span className="text-xs text-gray-400">Stunden (fest)</span>
        )}
      </td>
    </tr>
  );
}

function DatevCard({
  status,
  onSave,
}: {
  readonly status: PayrollStatus;
  readonly onSave: (key: string, value: string) => Promise<void>;
}) {
  return (
    <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
      <h2 className="text-sm font-semibold tracking-wide text-gray-400 uppercase">
        DATEV-Mandant
      </h2>
      <p className="mt-1 text-xs text-gray-500">
        Kennzahlen für den Kopf der LODAS-Datei. Lohn und Gehalt benötigt sie
        nicht.
      </p>
      <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
        <DatevNumberField
          label="Beraternummer"
          settingKey="payroll.datev_beraternummer"
          current={status.beraternummer}
          maxDigits={7}
          onSave={onSave}
        />
        <DatevNumberField
          label="Mandantennummer"
          settingKey="payroll.datev_mandantennummer"
          current={status.mandantennummer}
          maxDigits={5}
          onSave={onSave}
        />
      </div>
    </div>
  );
}

function DatevNumberField({
  label,
  settingKey,
  current,
  maxDigits,
  onSave,
}: {
  readonly label: string;
  readonly settingKey: string;
  readonly current: string;
  readonly maxDigits: number;
  readonly onSave: (key: string, value: string) => Promise<void>;
}) {
  const [value, setValue] = useState(current);
  useEffect(() => setValue(current), [current]);

  const valid =
    value.trim() === "" ||
    new RegExp(String.raw`^\d{1,${maxDigits}}$`).test(value.trim());

  return (
    <div>
      <label
        htmlFor={`datev-${settingKey}`}
        className="mb-1 block text-sm font-medium text-gray-700"
      >
        {label}
      </label>
      <Input
        id={`datev-${settingKey}`}
        value={value}
        onChange={(e) => setValue(e.target.value)}
        onBlur={() => {
          const trimmed = value.trim();
          if (valid && trimmed !== current) {
            void onSave(settingKey, trimmed);
          }
        }}
        placeholder="Nicht konfiguriert"
        inputMode="numeric"
        className="h-9 py-1 text-sm tabular-nums"
      />
      {!valid && (
        <p className="mt-1 text-xs text-[#FF3130]">
          1 bis {maxDigits} Ziffern.
        </p>
      )}
    </div>
  );
}
