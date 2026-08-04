import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { MultiCheckboxSelect } from "./multi-checkbox-select";

const OPTIONS = [
  { value: "1", label: "Bärengruppe" },
  { value: "2", label: "Sternengruppe" },
  { value: "3", label: "Sonnengruppe" },
  { value: "4", label: "Fußball-AG" },
];

function openMenu() {
  fireEvent.click(screen.getByRole("button", { name: "Auswahl" }));
}

describe("MultiCheckboxSelect (searchable)", () => {
  it("exposes validation state and its error description on the trigger", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        ariaInvalid
        ariaDescribedBy="selection_error"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
      />,
    );

    const trigger = screen.getByRole("button", { name: "Auswahl" });
    expect(trigger).toHaveAttribute("aria-invalid", "true");
    expect(trigger).toHaveAttribute("aria-describedby", "selection_error");
  });

  it("filters options by the search term", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
        searchable
      />,
    );
    openMenu();
    fireEvent.change(screen.getByPlaceholderText("Suchen..."), {
      target: { value: "gruppe" },
    });
    expect(screen.getByText("Bärengruppe")).toBeInTheDocument();
    expect(screen.getByText("Sternengruppe")).toBeInTheDocument();
    expect(screen.queryByText("Fußball-AG")).not.toBeInTheDocument();
  });

  it("shows an empty state for no matches", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
        searchable
      />,
    );
    openMenu();
    fireEvent.change(screen.getByPlaceholderText("Suchen..."), {
      target: { value: "xyz" },
    });
    expect(screen.getByText("Keine Treffer.")).toBeInTheDocument();
  });

  it("selects all filtered options", () => {
    const onChange = vi.fn();
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={["4"]}
        options={OPTIONS}
        onChange={onChange}
        searchable
      />,
    );
    openMenu();
    fireEvent.change(screen.getByPlaceholderText("Suchen..."), {
      target: { value: "gruppe" },
    });
    fireEvent.click(screen.getByText("Alle Treffer auswählen"));
    const next = onChange.mock.calls[0]?.[0] as string[];
    expect(next.sort()).toEqual(["1", "2", "3", "4"]);
  });

  it("clears only the filtered options when all matches are selected", () => {
    const onChange = vi.fn();
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={["1", "2", "3", "4"]}
        options={OPTIONS}
        onChange={onChange}
        searchable
      />,
    );
    openMenu();
    fireEvent.change(screen.getByPlaceholderText("Suchen..."), {
      target: { value: "gruppe" },
    });
    fireEvent.click(screen.getByText("Treffer abwählen"));
    expect(onChange).toHaveBeenCalledWith(["4"]);
  });

  it("renders no search chrome without the searchable flag", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
      />,
    );
    openMenu();
    expect(screen.queryByPlaceholderText("Suchen...")).not.toBeInTheDocument();
    expect(screen.queryByText("Alle auswählen")).not.toBeInTheDocument();
  });

  it("clips the scroll area inside the rounded menu surface", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
        searchable
      />,
    );
    openMenu();

    const menu = screen.getByRole("dialog", { name: "Auswahl" });
    expect(menu).toHaveClass("overflow-hidden", "rounded-xl", "py-2");
    expect(menu.firstElementChild).toHaveClass(
      "max-h-[17rem]",
      "overflow-y-auto",
      "overscroll-contain",
    );
  });

  it("keeps native checkbox semantics and closes on Escape", () => {
    render(
      <MultiCheckboxSelect
        ariaLabel="Auswahl"
        value={["1"]}
        options={OPTIONS}
        onChange={() => undefined}
        searchable
      />,
    );
    const trigger = screen.getByRole("button", { name: "Auswahl" });
    fireEvent.click(trigger);
    expect(screen.getByRole("dialog", { name: "Auswahl" })).toBeVisible();
    expect(screen.getByRole("checkbox", { name: "Bärengruppe" })).toBeChecked();

    fireEvent.keyDown(screen.getByRole("dialog", { name: "Auswahl" }), {
      key: "Escape",
    });

    expect(screen.queryByRole("dialog", { name: "Auswahl" })).toBeNull();
    expect(trigger).toHaveFocus();
  });

  it("closes on Escape while the trigger retains focus", () => {
    const onDocumentKeyDown = vi.fn();
    document.addEventListener("keydown", onDocumentKeyDown);

    try {
      render(
        <MultiCheckboxSelect
          ariaLabel="Auswahl"
          value={[]}
          options={OPTIONS}
          onChange={() => undefined}
        />,
      );
      const trigger = screen.getByRole("button", { name: "Auswahl" });
      trigger.focus();
      fireEvent.click(trigger);

      fireEvent.keyDown(trigger, { key: "Escape" });

      expect(screen.queryByRole("dialog", { name: "Auswahl" })).toBeNull();
      expect(trigger).toHaveFocus();
      expect(onDocumentKeyDown).not.toHaveBeenCalled();
    } finally {
      document.removeEventListener("keydown", onDocumentKeyDown);
    }
  });
});
