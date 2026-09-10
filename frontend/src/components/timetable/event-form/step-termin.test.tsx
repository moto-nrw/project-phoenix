import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { StepTermin } from "./step-termin";
import { emptyForm } from "./form-model";
import type { ActivityCategory } from "~/lib/activity-helpers";
import { ToastProvider } from "~/contexts/ToastContext";

const categories: ActivityCategory[] = [
  {
    id: "1",
    name: "Sport",
    isSystem: false,
    created_at: new Date("2026-08-01T10:00:00Z"),
    updated_at: new Date("2026-08-01T10:00:00Z"),
  },
];

function renderStep(
  overrides: Partial<React.ComponentProps<typeof StepTermin>> = {},
) {
  const update = vi.fn();
  render(
    <ToastProvider>
      <StepTermin
        form={{ ...emptyForm("2026-08-03"), type: "care" }}
        update={update}
        fieldErrors={{}}
        rooms={[]}
        categories={categories}
        loadingRefs={false}
        expanded
        isSeriesFlow
        isEditingSeries={false}
        quickPreset=""
        listKindTouched={createRef<boolean>() as React.RefObject<boolean>}
        canManageCategories
        canManagePlanningTracks
        {...overrides}
      />
    </ToastProvider>,
  );
  return { update };
}

describe("StepTermin — Kategorie (#2131, #3114)", () => {
  it("selects a category", () => {
    const { update } = renderStep();

    fireEvent.click(screen.getByRole("combobox", { name: "Kategorie" }));
    fireEvent.click(screen.getByRole("option", { name: "Sport" }));

    expect(update).toHaveBeenCalledWith("categoryId", "1");
  });

  it("offers no create entry in the field — the catalogue has a route now", () => {
    renderStep();

    fireEvent.click(screen.getByRole("combobox", { name: "Kategorie" }));

    expect(
      screen.queryByRole("option", { name: /Neue Kategorie anlegen/ }),
    ).not.toBeInTheDocument();
  });

  it("links to the catalogue in a second window so the draft survives", () => {
    renderStep();

    const link = screen.getByRole("link", {
      name: /Terminkategorien verwalten/,
    });

    // Der Pfad trägt den Mandanten, wie jeder interne Link.
    expect(link).toHaveAttribute(
      "href",
      expect.stringContaining("/database/categories") as unknown as string,
    );
    expect(link).toHaveAttribute("target", "_blank");
    expect(link).toHaveAttribute("rel", "noopener noreferrer");
  });

  it("hides the catalogue link without the category permission", () => {
    renderStep({ canManageCategories: false });

    expect(
      screen.queryByRole("link", { name: /Kategorien verwalten/ }),
    ).not.toBeInTheDocument();
  });

  it("keeps rendering the field error", () => {
    renderStep({
      fieldErrors: { categoryId: "Bitte eine Kategorie auswählen." },
    });

    expect(
      screen.getByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
  });
});

describe("StepTermin — Planungsspur (#3114)", () => {
  it("links to the catalogue in a second window", () => {
    renderStep();

    const link = screen.getByRole("link", {
      name: /Planungsspuren verwalten/,
    });

    expect(link).toHaveAttribute(
      "href",
      expect.stringContaining("/database/planning-tracks") as unknown as string,
    );
    expect(link).toHaveAttribute("target", "_blank");
  });

  it("hides the catalogue link without the planning permission", () => {
    renderStep({ canManagePlanningTracks: false });

    expect(
      screen.queryByRole("link", { name: /Planungsspuren verwalten/ }),
    ).not.toBeInTheDocument();
  });
});
