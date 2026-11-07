import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({
    data: undefined,
    error: undefined,
    isLoading: false,
    mutate: vi.fn(),
  }),
}));

import { ParentTargetPicker } from "./parent-target-picker";

describe("ParentTargetPicker", () => {
  it("names each target selector after its visible label", () => {
    render(
      <ParentTargetPicker
        targets={[]}
        groups={[]}
        activities={[]}
        schoolClasses={[]}
        studentNames={{}}
        onChange={vi.fn()}
        onSetStudentName={vi.fn()}
        kindLabel="diese Mitteilung"
        allowPendingEnrollment
        lookupLoadError={null}
      />,
    );

    for (const name of ["Klassen", "Gruppen", "AGs / Betreuung"]) {
      const selector = screen.getByRole("button", { name });
      expect(selector).toHaveAttribute("aria-labelledby");
      expect(selector).not.toHaveAttribute("aria-label");
    }
  });
});
