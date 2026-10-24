import { describe, expect, it } from "vitest";
import type { FormField } from "~/lib/enrollment-form-schema-api";
import {
  applyFileFieldRules,
  createIncomeDeclarationFields,
  fieldTypeChangePatch,
  fileFieldDraftProblem,
  hasIncomeDeclaration,
} from "./file-field-editor";

const base: FormField = {
  key: "nachweis",
  label: "Nachweis",
  type: "select",
  options: [{ label: "A", value: "a" }],
  sort_order: 0,
  applies_to_child: true,
};

describe("file field editor rules", () => {
  it("switching to Datei makes the field guardian-level with default rules", () => {
    expect(fieldTypeChangePatch(base, "file")).toEqual({
      type: "file",
      applies_to_child: false,
      options: undefined,
      validation: undefined,
      file_options: {
        allowed_types: ["pdf", "jpg", "png"],
        max_size_mb: 10,
        multiple: false,
      },
    });
    expect(fieldTypeChangePatch(base, "text")).toEqual({
      type: "text",
      file_options: undefined,
    });
  });

  it("saves a file field without options and with clamped file rules", () => {
    const saved = applyFileFieldRules({
      ...base,
      type: "file",
      file_options: {
        allowed_types: ["png", "pdf"],
        max_size_mb: 40,
        multiple: true,
      },
    });
    expect(saved.options).toBeUndefined();
    expect(saved.applies_to_child).toBe(false);
    expect(saved.file_options).toEqual({
      allowed_types: ["pdf", "png"],
      max_size_mb: 10,
      multiple: true,
    });
    expect(
      "file_options" in
        applyFileFieldRules({
          ...base,
          file_options: {
            allowed_types: ["pdf"],
            max_size_mb: 1,
            multiple: false,
          },
        }),
    ).toBe(false);
  });

  it("blocks saving a file field without a file kind", () => {
    expect(
      fileFieldDraftProblem(
        {
          ...base,
          type: "file",
          file_options: { allowed_types: [], max_size_mb: 5, multiple: false },
        },
        2,
      ),
    ).toMatchObject({ field: "fields.2.file_options" });
    expect(fileFieldDraftProblem(base, 2)).toBeNull();
  });

  it("builds the Einkommenserklärung preset from the contract", () => {
    const [level, proof] = createIncomeDeclarationFields(3);
    expect(level).toMatchObject({
      type: "select",
      required: true,
      applies_to_child: false,
      sort_order: 3,
    });
    expect(level?.options?.map((option) => option.value)).toEqual([
      "voll",
      "stufe_1",
      "stufe_2",
      "frei",
    ]);
    expect(proof).toMatchObject({
      type: "file",
      required: true,
      applies_to_child: false,
      sort_order: 4,
      file_options: {
        allowed_types: ["pdf", "jpg", "png"],
        max_size_mb: 10,
        multiple: true,
      },
      visible_when: {
        source: "field",
        field: level?.key,
        operator: "neq",
        value: "voll",
      },
    });
    expect(hasIncomeDeclaration([level!, proof!])).toBe(true);
    expect(hasIncomeDeclaration([base])).toBe(false);
  });
});
