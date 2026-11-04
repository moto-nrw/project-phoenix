import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import {
  DateOfBirthPicker,
  type DateOfBirthTranslator,
} from "./date-of-birth-picker";

const tr = Object.assign((key: string) => key, {
  raw: () => ["Januar", "September"],
}) as DateOfBirthTranslator;

describe("DateOfBirthPicker", () => {
  it("lays out the selects with the grid the layout test measures", () => {
    // e2e/layout/date-of-birth-picker.spec.ts checks these classes in a
    // browser (#5333); this keeps the component and that copy in step.
    render(<DateOfBirthPicker value="" onChange={() => {}} tr={tr} />);

    const grid = screen.getByRole("combobox", {
      name: "structured.month",
    }).parentElement!.parentElement!;
    expect(grid).toHaveClass(
      "grid",
      "grid-cols-1",
      "gap-2",
      "@sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_minmax(0,1fr)]",
    );
    expect(grid.parentElement).toHaveClass("@container", "mt-1");
  });
});
