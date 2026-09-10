import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { UseAbsenceTypeOptionsResult } from "./use-absence-type-options";

const typeOptions = vi.hoisted(() => ({
  current: { options: [] } as UseAbsenceTypeOptionsResult,
}));

vi.mock("./use-absence-type-options", () => ({
  useAbsenceTypeOptions: () => typeOptions.current,
}));

import { useAbsenceTypeSelect } from "./use-absence-type-select";
import { ListboxDropdown } from "~/components/ui/listbox-dropdown";
import type { AbsenceTypeOption } from "./use-absence-type-options";

const OPTIONS: AbsenceTypeOption[] = [
  { value: "sick", label: "Krank", fixed: true },
  { value: "vacation", label: "Urlaub", fixed: true },
  { value: "custom:7", label: "Regenerationstag" },
];

function Select({
  value,
  onChange = vi.fn(),
}: {
  readonly value: string;
  readonly onChange?: (next: string) => void;
}) {
  const props = useAbsenceTypeSelect({ value, onChange, canManage: true });
  return <ListboxDropdown {...props} ariaLabel="Art der Abwesenheit" />;
}

function open(): void {
  fireEvent.click(screen.getByRole("combobox"));
}

beforeEach(() => {
  typeOptions.current = { options: [...OPTIONS] };
});

// Seit #3114 ist das Feld eine reine Auswahl: Anlegen, Umbenennen und
// Abschalten liegen unter „Datenverwaltung → Abwesenheitsarten".
describe("useAbsenceTypeSelect", () => {
  it("shows the selected option's label on the trigger", () => {
    render(<Select value="custom:7" />);
    expect(screen.getByRole("combobox")).toHaveTextContent("Regenerationstag");
  });

  it("filters the list by the typed text, ignoring case", () => {
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "regen" },
    });

    expect(
      screen.getByRole("option", { name: /Regenerationstag/ }),
    ).toBeTruthy();
    expect(screen.queryByRole("option", { name: "Krank" })).toBeNull();
  });

  it("says so when nothing matches", () => {
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });

    expect(screen.getByText("Kein Treffer.")).toBeTruthy();
  });

  it("moves option focus with the arrow, home, and end keys", () => {
    render(<Select value="sick" />);
    open();

    const search = screen.getByRole("textbox");
    fireEvent.keyDown(search, { key: "ArrowDown" });
    expect(screen.getByRole("option", { name: "Urlaub" })).toHaveFocus();
    fireEvent.keyDown(screen.getByRole("option", { name: "Urlaub" }), {
      key: "End",
    });
    expect(
      screen.getByRole("option", { name: /Regenerationstag/ }),
    ).toHaveFocus();
    fireEvent.keyDown(
      screen.getByRole("option", { name: /Regenerationstag/ }),
      { key: "Home" },
    );
    expect(screen.getByRole("option", { name: "Krank" })).toHaveFocus();
  });

  it("carries no management affordance at all", () => {
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });

    expect(screen.queryByRole("button", { name: /hinzuf/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /umbenennen/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /deaktivieren/i })).toBeNull();
  });

  it("keeps a retired option visible while it is the current value", () => {
    typeOptions.current = {
      options: [
        ...OPTIONS,
        { value: "custom:9", label: "Sonderurlaub", inactive: true },
      ],
    };
    render(<Select value="custom:9" />);
    open();

    expect(screen.getByRole("option", { name: /Sonderurlaub/ })).toBeTruthy();
  });

  it("hides a retired option that is not the current value", () => {
    typeOptions.current = {
      options: [
        ...OPTIONS,
        { value: "custom:9", label: "Sonderurlaub", inactive: true },
      ],
    };
    render(<Select value="sick" />);
    open();

    expect(screen.queryByRole("option", { name: /Sonderurlaub/ })).toBeNull();
  });

  it("selects an option", () => {
    const onChange = vi.fn();
    render(<Select value="sick" onChange={onChange} />);
    open();
    fireEvent.click(screen.getByRole("option", { name: /Regenerationstag/ }));

    expect(onChange).toHaveBeenCalledWith("custom:7");
  });
});
