import { renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AbsenceType } from "~/lib/absence-type-api";

const absenceTypes = vi.hoisted(() => ({
  current: [] as AbsenceType[],
}));

vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: { getAbsenceTypes: vi.fn() },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ data: absenceTypes.current }),
}));

import {
  STANDARD_ABSENCE_OPTIONS,
  useAbsenceTypeOptions,
} from "./use-absence-type-options";

const REGENERATIONSTAG: AbsenceType = {
  id: "7",
  name: "Regenerationstag",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
  carryoverUntil: null,
};

const SONDERURLAUB: AbsenceType = {
  id: "8",
  name: "Sonderurlaub",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
  carryoverUntil: null,
};

describe("useAbsenceTypeOptions", () => {
  beforeEach(() => {
    absenceTypes.current = [];
  });

  it("keeps the current allowance-backed type for a non-manager", () => {
    absenceTypes.current = [REGENERATIONSTAG, SONDERURLAUB];

    const { result } = renderHook(() =>
      useAbsenceTypeOptions(false, STANDARD_ABSENCE_OPTIONS, "custom:7"),
    );

    expect(result.current.options).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          value: "custom:7",
          label: "Regenerationstag",
        }),
      ]),
    );
    expect(result.current.options).not.toEqual(
      expect.arrayContaining([expect.objectContaining({ value: "custom:8" })]),
    );
  });
});

describe("STANDARD_ABSENCE_OPTIONS", () => {
  it("marks every standard type as fixed so none looks editable", () => {
    for (const option of STANDARD_ABSENCE_OPTIONS) {
      expect(option.fixed).toBe(true);
    }
  });

  it("omits Freizeitausgleich, which stays manager-controlled", () => {
    expect(
      STANDARD_ABSENCE_OPTIONS.map((option) => option.value),
    ).not.toContain("comp_time");
  });
});
