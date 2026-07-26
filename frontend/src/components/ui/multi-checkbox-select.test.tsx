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
});
