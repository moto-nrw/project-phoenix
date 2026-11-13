import { act, fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { MultiSelect } from "./multi-select";

const OPTIONS = [
  { value: "1", label: "Bärengruppe" },
  { value: "2", label: "Sternengruppe" },
  { value: "3", label: "Sonnengruppe" },
  { value: "4", label: "Fußball-AG" },
];

function trigger() {
  return screen.getByRole("combobox", { name: "Auswahl" });
}

function openMenu() {
  fireEvent.click(trigger());
}

const NONE: string[] = [];

function Controlled({
  initial = NONE,
  onChange,
  ...props
}: Partial<React.ComponentProps<typeof MultiSelect>> & {
  readonly initial?: string[];
}) {
  const [value, setValue] = useState<string[]>(initial);
  return (
    <MultiSelect
      ariaLabel="Auswahl"
      options={OPTIONS}
      {...props}
      value={value}
      onChange={(next) => {
        setValue(next);
        onChange?.(next);
      }}
    />
  );
}

describe("MultiSelect", () => {
  it("exposes validation state and its error description on the trigger", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        invalid
        ariaDescribedBy="selection_error"
        value={[]}
        options={OPTIONS}
        onChange={() => undefined}
      />,
    );

    expect(trigger()).toHaveAttribute("aria-invalid", "true");
    expect(trigger()).toHaveAttribute("aria-describedby", "selection_error");
  });

  it("lists the options as checkable entries of a multi-selectable listbox", () => {
    render(<Controlled initial={["1"]} />);
    openMenu();

    expect(screen.getByRole("listbox")).toHaveAttribute(
      "aria-multiselectable",
      "true",
    );
    expect(screen.getByRole("option", { name: "Bärengruppe" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    expect(
      screen.getByRole("option", { name: "Sternengruppe" }),
    ).toHaveAttribute("aria-selected", "false");
  });

  it("shows a badge under the option label", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={[]}
        options={[
          {
            value: "a",
            label: "Fußball-AG",
            badge: "Zählt nicht als Betreuungstag",
          },
        ]}
        onChange={() => undefined}
      />,
    );
    openMenu();

    expect(
      screen.getByRole("option", { name: "Fußball-AG" }),
    ).toHaveTextContent("Zählt nicht als Betreuungstag");
  });

  it("summarises the selection with the field's own words", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={["1", "2"]}
        options={OPTIONS}
        onChange={() => undefined}
        summaryLabel={(count) => `${count} Gruppen`}
      />,
    );

    expect(trigger()).toHaveTextContent("2 Gruppen");
  });

  it("summarises multiple selections when no field-specific label is given", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={["1", "2"]}
        options={OPTIONS}
        onChange={() => undefined}
      />,
    );

    expect(trigger()).toHaveTextContent("2 ausgewählt");
  });

  it("can summarise every selection from the chosen options", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={["4"]}
        options={OPTIONS}
        onChange={() => undefined}
        summaryThreshold={0}
        summaryLabel={(_count, selected) =>
          selected.map((option) => option.label.slice(0, 1)).join(", ")
        }
      />,
    );

    expect(trigger()).toHaveTextContent("F");
  });

  it("ignores values that no longer have an option", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={["1", "entfernt"]}
        options={OPTIONS}
        onChange={() => undefined}
        summaryLabel={(count) => `${count} Gruppen`}
      />,
    );

    expect(trigger()).toHaveTextContent("Bärengruppe");
  });

  it("disables the field and says so when there is nothing to choose", () => {
    render(
      <MultiSelect
        ariaLabel="Auswahl"
        value={[]}
        options={[]}
        onChange={() => undefined}
        unavailableLabel="Keine Rollen vorhanden"
      />,
    );

    expect(trigger()).toBeDisabled();
    expect(trigger()).toHaveTextContent("Keine Rollen vorhanden");
  });

  describe("search and select all", () => {
    it("filters options by the search term", () => {
      render(<Controlled searchable />);
      openMenu();
      fireEvent.change(screen.getByRole("textbox", { name: "Suchen" }), {
        target: { value: "gruppe" },
      });

      expect(screen.getByRole("option", { name: "Bärengruppe" })).toBeTruthy();
      expect(
        screen.getByRole("option", { name: "Sternengruppe" }),
      ).toBeTruthy();
      expect(screen.queryByRole("option", { name: "Fußball-AG" })).toBeNull();
    });

    it("closes the panel when Tab leaves the select-all action", () => {
      render(<Controlled searchable selectAll />);
      openMenu();

      const selectAll = screen.getByRole("button", {
        name: "Alle auswählen",
      });
      // Anker: Das geöffnete Panel enthält die Aktion zum Auswählen aller Werte.
      expect(screen.getByRole("listbox")).toBeInTheDocument();

      fireEvent.keyDown(selectAll, { key: "Tab" });

      expect(trigger()).toHaveAttribute("aria-expanded", "false");
      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
      expect(trigger()).toHaveFocus();
    });

    it("keeps keyboard focus on an option when filtering changes its index", () => {
      const onChange = vi.fn();
      render(<Controlled searchable onChange={onChange} />);
      openMenu();

      const search = screen.getByRole("textbox", { name: "Suchen" });
      fireEvent.keyDown(search, { key: "ArrowDown" });
      fireEvent.keyDown(screen.getByRole("option", { name: "Bärengruppe" }), {
        key: "ArrowDown",
      });
      fireEvent.keyDown(screen.getByRole("option", { name: "Sternengruppe" }), {
        key: "ArrowDown",
      });
      fireEvent.keyDown(screen.getByRole("option", { name: "Sonnengruppe" }), {
        key: "ArrowDown",
      });

      fireEvent.change(search, { target: { value: "fußball" } });

      const focusedOption = screen.getByRole("option", {
        name: "Fußball-AG",
      });
      expect(focusedOption).toHaveFocus();
      expect(trigger()).toHaveAttribute(
        "aria-activedescendant",
        focusedOption.id,
      );

      fireEvent.keyDown(focusedOption, { key: "Enter" });
      expect(onChange).toHaveBeenCalledWith(["4"]);
    });

    it("shows an empty state for no matches", () => {
      render(<Controlled searchable />);
      openMenu();
      const search = screen.getByRole("textbox", { name: "Suchen" });
      fireEvent.change(search, {
        target: { value: "xyz" },
      });

      expect(screen.getByText("Keine Treffer.")).toBeInTheDocument();
      fireEvent.keyDown(search, { key: "ArrowDown" });
      fireEvent.keyDown(search, { key: "ArrowUp" });
      expect(trigger()).not.toHaveAttribute("aria-activedescendant");
    });

    it("selects all filtered options", () => {
      const onChange = vi.fn();
      render(
        <MultiSelect
          ariaLabel="Auswahl"
          value={["4"]}
          options={OPTIONS}
          onChange={onChange}
          searchable
          selectAll
        />,
      );
      openMenu();
      fireEvent.change(screen.getByRole("textbox", { name: "Suchen" }), {
        target: { value: "gruppe" },
      });
      fireEvent.click(
        screen.getByRole("button", { name: "Alle Treffer auswählen" }),
      );

      const next = onChange.mock.calls[0]?.[0] as string[];
      expect([...next].sort()).toEqual(["1", "2", "3", "4"]);
    });

    it("clears only the filtered options when all matches are selected", () => {
      const onChange = vi.fn();
      render(
        <MultiSelect
          ariaLabel="Auswahl"
          value={["1", "2", "3", "4"]}
          options={OPTIONS}
          onChange={onChange}
          searchable
          selectAll
        />,
      );
      openMenu();
      fireEvent.change(screen.getByRole("textbox", { name: "Suchen" }), {
        target: { value: "gruppe" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Treffer abwählen" }));

      expect(onChange).toHaveBeenCalledWith(["4"]);
    });

    it("offers select all without a search field too", () => {
      const onChange = vi.fn();
      render(
        <MultiSelect
          ariaLabel="Auswahl"
          value={[]}
          options={OPTIONS}
          onChange={onChange}
          selectAll
        />,
      );
      openMenu();
      fireEvent.click(screen.getByRole("button", { name: "Alle auswählen" }));

      expect(onChange).toHaveBeenCalledWith(["1", "2", "3", "4"]);
    });

    it("renders no search chrome without the flags", () => {
      render(<Controlled />);
      openMenu();

      expect(screen.queryByRole("textbox")).toBeNull();
      expect(screen.getByRole("listbox")).toBeInTheDocument();
      expect(
        screen.queryByRole("button", { name: "Alle auswählen" }),
      ).toBeNull();
    });
  });

  describe("keyboard", () => {
    it("opens with ArrowDown and moves between options with the arrow keys", () => {
      render(<Controlled />);
      trigger().focus();
      fireEvent.keyDown(trigger(), { key: "ArrowDown" });

      expect(screen.getByRole("option", { name: "Bärengruppe" })).toHaveFocus();
      fireEvent.keyDown(screen.getByRole("option", { name: "Bärengruppe" }), {
        key: "ArrowDown",
      });
      expect(
        screen.getByRole("option", { name: "Sternengruppe" }),
      ).toHaveFocus();
      fireEvent.keyDown(screen.getByRole("option", { name: "Sternengruppe" }), {
        key: "ArrowUp",
      });
      expect(screen.getByRole("option", { name: "Bärengruppe" })).toHaveFocus();
    });

    it("toggles the focused option with Enter and Space and stays open", () => {
      const onChange = vi.fn();
      render(<Controlled onChange={onChange} />);
      fireEvent.keyDown(trigger(), { key: "ArrowDown" });

      fireEvent.keyDown(screen.getByRole("option", { name: "Bärengruppe" }), {
        key: "Enter",
      });
      expect(onChange).toHaveBeenLastCalledWith(["1"]);
      expect(screen.getByRole("listbox")).toBeInTheDocument();

      fireEvent.keyDown(screen.getByRole("option", { name: "Bärengruppe" }), {
        key: "ArrowDown",
      });
      fireEvent.keyDown(screen.getByRole("option", { name: "Sternengruppe" }), {
        key: " ",
      });
      expect(onChange).toHaveBeenLastCalledWith(["1", "2"]);

      fireEvent.keyDown(screen.getByRole("option", { name: "Sternengruppe" }), {
        key: "Enter",
      });
      expect(onChange).toHaveBeenLastCalledWith(["1"]);
      expect(screen.getByRole("listbox")).toBeInTheDocument();
    });

    it("jumps to an option by typing its first letters", () => {
      render(<Controlled />);
      trigger().focus();
      fireEvent.keyDown(trigger(), { key: "s" });

      expect(
        screen.getByRole("option", { name: "Sternengruppe" }),
      ).toHaveFocus();
      fireEvent.keyDown(screen.getByRole("option", { name: "Sternengruppe" }), {
        key: "s",
      });
      expect(
        screen.getByRole("option", { name: "Sonnengruppe" }),
      ).toHaveFocus();
    });

    it("closes on Escape from an option and returns focus to the trigger", () => {
      render(<Controlled initial={["1"]} searchable />);
      openMenu();
      fireEvent.keyDown(screen.getByRole("textbox", { name: "Suchen" }), {
        key: "ArrowDown",
      });
      const focused = document.activeElement as HTMLElement;
      expect(focused).toHaveAttribute("role", "option");

      fireEvent.keyDown(focused, { key: "Escape" });

      expect(screen.queryByRole("listbox")).toBeNull();
      expect(trigger()).toHaveFocus();
    });

    it("closes on Escape while the trigger retains focus without reaching document listeners", () => {
      const onDocumentKeyDown = vi.fn();
      document.addEventListener("keydown", onDocumentKeyDown);

      try {
        render(<Controlled />);
        trigger().focus();
        fireEvent.click(trigger());
        act(() => trigger().focus());

        fireEvent.keyDown(trigger(), { key: "Escape" });

        expect(screen.queryByRole("listbox")).toBeNull();
        expect(trigger()).toHaveFocus();
        expect(onDocumentKeyDown).not.toHaveBeenCalled();
      } finally {
        document.removeEventListener("keydown", onDocumentKeyDown);
      }
    });
  });
});
