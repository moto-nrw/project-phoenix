import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { SuggestInput } from "./suggest-input";

const POSITIONS = ["Pädagogische Fachkraft", "OGS-Büro", "Leitung"] as const;

function Probe({
  highlightFirst = false,
  onKeyDown,
  onSuggestionSelect,
  maxSuggestions,
}: Readonly<{
  highlightFirst?: boolean;
  onKeyDown?: () => void;
  onSuggestionSelect?: (value: string) => void;
  maxSuggestions?: number;
}>) {
  const [value, setValue] = useState("");
  return (
    <SuggestInput
      label="Position"
      name="position"
      value={value}
      onValueChange={setValue}
      suggestions={POSITIONS}
      listLabel="Vorschläge für die Position"
      highlightFirst={highlightFirst}
      maxSuggestions={maxSuggestions}
      onKeyDown={onKeyDown}
      onSuggestionSelect={onSuggestionSelect}
    />
  );
}

function field() {
  return screen.getByRole("combobox", { name: "Position" });
}

describe("SuggestInput", () => {
  it("offers all suggestions on focus and links the list to the field", () => {
    render(<Probe />);
    expect(field()).toHaveAttribute("aria-expanded", "false");

    fireEvent.focus(field());

    const list = screen.getByRole("listbox", {
      name: "Vorschläge für die Position",
    });
    expect(field()).toHaveAttribute("aria-expanded", "true");
    expect(field()).toHaveAttribute("aria-controls", list.id);
    expect(screen.getAllByRole("option")).toHaveLength(3);
  });

  it("filters ignoring case and umlauts and keeps free text", () => {
    render(<Probe />);
    fireEvent.change(field(), { target: { value: "padag" } });

    expect(screen.getAllByRole("option").map((o) => o.textContent)).toEqual([
      "Pädagogische Fachkraft",
    ]);

    fireEvent.change(field(), { target: { value: "Hausmeister" } });
    expect(field()).toHaveValue("Hausmeister");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
    expect(field()).toHaveAttribute("aria-expanded", "false");
  });

  it("moves with the arrow keys, wraps and takes the entry on Enter", () => {
    const onSuggestionSelect = vi.fn();
    render(<Probe onSuggestionSelect={onSuggestionSelect} />);
    fireEvent.focus(field());
    expect(field()).not.toHaveAttribute("aria-activedescendant");

    fireEvent.keyDown(field(), { key: "ArrowDown" });
    fireEvent.keyDown(field(), { key: "ArrowDown" });
    const options = screen.getAllByRole("option");
    expect(field()).toHaveAttribute("aria-activedescendant", options[1]?.id);
    expect(options[1]).toHaveAttribute("aria-selected", "true");

    fireEvent.keyDown(field(), { key: "ArrowUp" });
    fireEvent.keyDown(field(), { key: "ArrowUp" });
    expect(field()).toHaveAttribute("aria-activedescendant", options[2]?.id);

    fireEvent.keyDown(field(), { key: "Home" });
    expect(field()).toHaveAttribute("aria-activedescendant", options[0]?.id);
    fireEvent.keyDown(field(), { key: "End" });
    expect(field()).toHaveAttribute("aria-activedescendant", options[2]?.id);

    fireEvent.keyDown(field(), { key: "Enter" });
    expect(field()).toHaveValue("Leitung");
    expect(onSuggestionSelect).toHaveBeenCalledWith("Leitung");
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("leaves Enter to the caller while no entry is marked", () => {
    const onKeyDown = vi.fn();
    render(<Probe onKeyDown={onKeyDown} />);
    fireEvent.change(field(), { target: { value: "Lei" } });

    fireEvent.keyDown(field(), { key: "Enter" });

    expect(field()).toHaveValue("Lei");
    expect(onKeyDown).toHaveBeenCalledTimes(1);
  });

  it("takes the first match on Enter with highlightFirst", () => {
    const onKeyDown = vi.fn();
    render(<Probe highlightFirst onKeyDown={onKeyDown} />);
    fireEvent.change(field(), { target: { value: "büro" } });

    fireEvent.keyDown(field(), { key: "Enter" });

    expect(field()).toHaveValue("OGS-Büro");
    expect(onKeyDown).not.toHaveBeenCalled();
  });

  it("closes only the list on Escape and keeps it from the document", () => {
    const documentEscape = vi.fn();
    document.addEventListener("keydown", documentEscape);
    try {
      render(<Probe />);
      fireEvent.focus(field());

      fireEvent.keyDown(field(), { key: "Escape" });

      expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
      expect(documentEscape).not.toHaveBeenCalled();

      // Closed: the next Escape belongs to the surrounding dialog again.
      fireEvent.keyDown(field(), { key: "Escape" });
      expect(documentEscape).toHaveBeenCalledTimes(1);
    } finally {
      document.removeEventListener("keydown", documentEscape);
    }
  });

  it("opens with ArrowDown after Escape", () => {
    render(<Probe />);
    fireEvent.focus(field());
    fireEvent.keyDown(field(), { key: "Escape" });

    fireEvent.keyDown(field(), { key: "ArrowDown" });

    const options = screen.getAllByRole("option");
    expect(field()).toHaveAttribute("aria-activedescendant", options[0]?.id);
  });

  it("takes a tapped entry on pointer-down without moving the focus", () => {
    render(<Probe />);
    field().focus();
    fireEvent.focus(field());

    const option = screen.getByRole("option", { name: "OGS-Büro" });
    const pointerDown = fireEvent.pointerDown(option);

    expect(pointerDown).toBe(false);
    expect(field()).toHaveValue("OGS-Büro");
    expect(field()).toHaveFocus();
    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("closes the list when the field loses focus", () => {
    render(<Probe />);
    fireEvent.focus(field());

    fireEvent.blur(field());

    expect(screen.queryByRole("listbox")).not.toBeInTheDocument();
  });

  it("limits the matches with maxSuggestions", () => {
    render(<Probe maxSuggestions={2} />);
    fireEvent.focus(field());

    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("gives every entry a finger-sized target on touch screens", () => {
    render(<Probe />);
    fireEvent.focus(field());

    for (const option of screen.getAllByRole("option")) {
      expect(option.className).toContain("pointer-coarse:min-h-11");
    }
  });

  it("portals into a surrounding dialog's focus scope", () => {
    render(
      <div data-modal-focus-scope="true" data-testid="scope">
        <SuggestInput
          aria-label="Klasse"
          value=""
          onValueChange={vi.fn()}
          suggestions={["1a", "1b"]}
          listLabel="Vorschläge für die Klasse"
        />
      </div>,
    );
    fireEvent.focus(screen.getByRole("combobox", { name: "Klasse" }));

    expect(screen.getByTestId("scope")).toContainElement(
      screen.getByRole("listbox"),
    );
  });
});
