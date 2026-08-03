import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { CREATE_CATEGORY_OPTION, StepTermin } from "./step-termin";
import { emptyForm } from "./form-model";
import type { ActivityCategory } from "~/lib/activity-helpers";

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
  const onManageCategories = vi.fn();
  render(
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
      onManageCategories={onManageCategories}
      {...overrides}
    />,
  );
  return { update, onManageCategories };
}

describe("StepTermin — Kategorie (#2131)", () => {
  it("offers creating the missing category without leaving the Termin", () => {
    const { onManageCategories, update } = renderStep();

    // The dropdown is a CustomSelect; open it, then pick the last entry.
    fireEvent.click(screen.getByRole("combobox", { name: "Kategorie" }));
    fireEvent.click(
      screen.getByRole("option", { name: "+ Neue Kategorie anlegen" }),
    );

    expect(onManageCategories).toHaveBeenCalledWith("create");
    // The sentinel must never land in the form state as a category id.
    expect(update).not.toHaveBeenCalledWith(
      "categoryId",
      CREATE_CATEGORY_OPTION,
    );
  });

  it("selects a real category normally", () => {
    const { update, onManageCategories } = renderStep();

    fireEvent.click(screen.getByRole("combobox", { name: "Kategorie" }));
    fireEvent.click(screen.getByRole("option", { name: "Sport" }));

    expect(update).toHaveBeenCalledWith("categoryId", "1");
    expect(onManageCategories).not.toHaveBeenCalled();
  });

  it("opens the manage dialog on the list from the Verwalten link", () => {
    const { onManageCategories } = renderStep();

    fireEvent.click(screen.getByRole("button", { name: "Verwalten" }));

    expect(onManageCategories).toHaveBeenCalledWith("list");
  });

  it("hides management controls without the category permission", () => {
    renderStep({ canManageCategories: false });

    expect(
      screen.queryByRole("button", { name: "Verwalten" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("combobox", { name: "Kategorie" }));
    expect(
      screen.queryByRole("option", { name: "+ Neue Kategorie anlegen" }),
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
