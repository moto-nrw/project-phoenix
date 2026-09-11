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
  overrunPolicy: "block",
};

const SONDERURLAUB: AbsenceType = {
  id: "8",
  name: "Sonderurlaub",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
  overrunPolicy: "warn",
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
