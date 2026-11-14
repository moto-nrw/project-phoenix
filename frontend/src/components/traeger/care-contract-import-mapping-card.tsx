"use client";

// Zuordnung der Spalten beim Vertragsimport (#4723): je Feld eines Vertrags
// die Spalte der Datei, aus der es kommt, in drei Gruppen: Kind, Betreuung,
// Eltern und Abrechnung. moto schlägt die Spalten aus den Überschriften vor;
// jede Änderung liest die Datei neu. Sind alle Pflichtfelder zugeordnet,
// klappt die Karte zu, damit die Zeilen direkt unter der Datei stehen. Hat die Datei keine Spalte für die
// Schule oder das Modul, wählt das Büro sie einmal für alle Zeilen.

import { useState } from "react";
import { CustomSelect } from "~/components/ui/custom-select";
import { SectionCard } from "~/components/ui/section-card";
import { SectionLabel } from "~/components/ui/section-label";
import type {
  ContractImportField,
  ContractImportMapping,
  ContractImportSchool,
} from "~/lib/traeger-care-contract-import-api";
import {
  CONTRACT_IMPORT_FIELD_HINTS,
  CONTRACT_IMPORT_FIELD_LABELS,
  CONTRACT_IMPORT_GROUPS,
  REQUIRED_CONTRACT_IMPORT_FIELDS,
} from "./care-contract-import-format";

const UNMAPPED = "";

/** The module names a file can name: of the chosen school, else of all. */
function moduleNames(
  schools: readonly ContractImportSchool[],
  schoolId: string,
): string[] {
  const names = new Set<string>();
  for (const school of schools) {
    if (schoolId !== "" && school.id !== schoolId) continue;
    for (const entry of school.modules) names.add(entry.name);
  }
  return [...names].sort((a, b) => a.localeCompare(b, "de"));
}

export function CareContractImportMappingCard({
  headers,
  mapping,
  unmapped,
  schools,
  debtorNumberScope,
  disabled,
  onChange,
}: Readonly<{
  headers: readonly string[];
  mapping: ContractImportMapping;
  unmapped: readonly ContractImportField[];
  schools: readonly ContractImportSchool[];
  debtorNumberScope: "payer" | "child";
  disabled: boolean;
  onChange: (mapping: ContractImportMapping) => void;
}>) {
  const [open, setOpen] = useState(false);
  const columnOptions = [
    { value: UNMAPPED, label: "Nicht zuordnen" },
    ...headers
      .filter((header) => header !== "")
      .map((header) => ({ value: header, label: header })),
  ];
  const missing = new Set(unmapped);
  const setColumn = (field: ContractImportField, header: string) =>
    onChange({
      ...mapping,
      columns: { ...mapping.columns, [field]: header },
      // A column goes before the choice for all rows.
      schoolId: field === "school" && header !== "" ? "" : mapping.schoolId,
      moduleName: field === "module" && header !== "" ? "" : mapping.moduleName,
    });
  const hint = (field: ContractImportField): string | undefined => {
    if (field === "debtor_number") {
      return debtorNumberScope === "child"
        ? "Bei Ihnen gilt eine Nummer je Kind."
        : "Bei Ihnen gilt eine Nummer je Zahler. Geschwister teilen sie.";
    }
    return CONTRACT_IMPORT_FIELD_HINTS[field];
  };
  // Once every required field has its column the card folds away, so the
  // rows stand right under the file; it opens again on request.
  const complete = unmapped.length === 0;
  const collapsed = complete && !open;
  const mapped = Object.values(mapping.columns).filter(
    (header) => header !== UNMAPPED,
  ).length;
  const schoolForAll = mapping.columns.school === UNMAPPED;
  const moduleForAll = mapping.columns.module === UNMAPPED;
  const modules = moduleNames(schools, mapping.schoolId);

  return (
    <SectionCard
      title="Spalten zuordnen"
      description={
        collapsed
          ? `${mapped} von ${Object.keys(mapping.columns).length} Feldern sind zugeordnet, alle Pflichtfelder sind dabei.`
          : "Wählen Sie für jedes Feld die Spalte aus Ihrer Datei. moto hat schon passende Spalten vorgeschlagen."
      }
      collapsible={complete}
      collapsed={collapsed}
      onCollapsedChange={(next) => setOpen(!next)}
    >
      <div className="flex flex-col gap-6">
        {CONTRACT_IMPORT_GROUPS.map((group) => (
          <fieldset key={group.title} className="min-w-0">
            <SectionLabel as="legend" className="mb-3">
              {group.title}
            </SectionLabel>
            <div className="grid gap-x-5 gap-y-4 sm:grid-cols-2 xl:grid-cols-3">
              {group.fields.map((field) => (
                <CustomSelect
                  key={field}
                  id={`contract-import-${field}`}
                  label={CONTRACT_IMPORT_FIELD_LABELS[field]}
                  required={REQUIRED_CONTRACT_IMPORT_FIELDS.has(field)}
                  value={mapping.columns[field]}
                  options={columnOptions}
                  onChange={(header) => setColumn(field, header)}
                  disabled={disabled}
                  hint={hint(field)}
                  error={
                    missing.has(field) &&
                    field !== "school" &&
                    field !== "module"
                      ? "Bitte eine Spalte wählen."
                      : null
                  }
                />
              ))}
              {group.fields.includes("school") && schoolForAll ? (
                <CustomSelect
                  id="contract-import-school-for-all"
                  label="Schule für alle Zeilen"
                  required
                  value={mapping.schoolId}
                  options={[
                    { value: "", label: "Bitte wählen" },
                    ...schools.map((school) => ({
                      value: school.id,
                      label: school.name,
                    })),
                  ]}
                  onChange={(schoolId) =>
                    onChange({ ...mapping, schoolId, moduleName: "" })
                  }
                  disabled={disabled}
                  hint="Ihre Datei hat keine Spalte für die Schule. Dann gilt eine Schule für alle Kinder."
                  error={
                    missing.has("school")
                      ? "Bitte eine Spalte oder eine Schule wählen."
                      : null
                  }
                />
              ) : null}
              {group.fields.includes("module") && moduleForAll ? (
                <CustomSelect
                  id="contract-import-module-for-all"
                  label="Modul für alle Zeilen"
                  required
                  value={mapping.moduleName}
                  options={[
                    { value: "", label: "Bitte wählen" },
                    ...modules.map((name) => ({ value: name, label: name })),
                  ]}
                  onChange={(moduleName) =>
                    onChange({ ...mapping, moduleName })
                  }
                  disabled={disabled}
                  hint="Ihre Datei hat keine Spalte für das Modul. Dann buchen alle Kinder dieses Modul."
                  error={
                    missing.has("module")
                      ? "Bitte eine Spalte oder ein Modul wählen."
                      : null
                  }
                />
              ) : null}
            </div>
          </fieldset>
        ))}
      </div>
    </SectionCard>
  );
}
