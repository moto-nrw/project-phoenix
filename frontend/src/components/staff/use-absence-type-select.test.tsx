import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

  it("offers to add a name that does not exist yet and selects it", async () => {
    const create = vi.fn().mockResolvedValue("custom:12");
    const onChange = vi.fn();
    typeOptions.current = { options: [...OPTIONS], create };
    render(<Select value="sick" onChange={onChange} />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "  Ferienzeit  " },
    });

    fireEvent.click(screen.getByRole("button", { name: /Ferienzeit.*hinzuf/ }));

    await waitFor(() => expect(create).toHaveBeenCalledWith("Ferienzeit"));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith("custom:12"));
  });

  it("does not offer to add a name that already exists, in any case", () => {
    typeOptions.current = { options: [...OPTIONS], create: vi.fn() };
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "REGENERATIONSTAG" },
    });

    expect(screen.queryByRole("button", { name: /hinzuf/ })).toBeNull();
  });

  it("hides every management affordance without the callbacks", () => {
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });

    expect(screen.queryByRole("button", { name: /hinzuf/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /umbenennen/i })).toBeNull();
  });

  it("never offers to rename or retire a fixed option", () => {
    typeOptions.current = {
      options: [...OPTIONS],
      rename: vi.fn(),
      setActive: vi.fn(),
    };
    render(<Select value="sick" />);
    open();

    expect(
      screen.getByRole("button", { name: "Regenerationstag umbenennen" }),
    ).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "Krank umbenennen" }),
    ).toBeNull();
    expect(
      screen.queryByRole("button", { name: "Urlaub deaktivieren" }),
    ).toBeNull();
  });

  it("renames an option in place", async () => {
    const rename = vi.fn().mockResolvedValue(undefined);
    typeOptions.current = { options: [...OPTIONS], rename };
    render(<Select value="sick" />);
    open();
    fireEvent.click(
      screen.getByRole("button", { name: "Regenerationstag umbenennen" }),
    );
    fireEvent.change(screen.getByRole("textbox", { name: /Name von/ }), {
      target: { value: "Regenerationstage" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Namen speichern" }));

    await waitFor(() =>
      expect(rename).toHaveBeenCalledWith("custom:7", "Regenerationstage"),
    );
  });

  it("keeps focus in the rename field instead of the search field", () => {
    typeOptions.current = { options: [...OPTIONS], rename: vi.fn() };
    render(<Select value="sick" />);
    open();
    fireEvent.click(
      screen.getByRole("button", { name: "Regenerationstag umbenennen" }),
    );

    expect(screen.getByRole("textbox", { name: /Name von/ })).toHaveFocus();
  });

  it("surfaces a failed add instead of closing silently", async () => {
    const create = vi
      .fn()
      .mockRejectedValue(new Error("Name ist bereits vergeben"));
    typeOptions.current = { options: [...OPTIONS], create };
    render(<Select value="sick" />);
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });
    fireEvent.click(screen.getByRole("button", { name: /hinzuf/ }));

    expect(await screen.findByText("Name ist bereits vergeben")).toBeTruthy();
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

  it("hides a retired option from someone who cannot restore it", () => {
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

  it("keeps a retired option available for reactivation but not for selection", () => {
    const onChange = vi.fn();
    typeOptions.current = {
      options: [
        ...OPTIONS,
        { value: "custom:9", label: "Sonderurlaub", inactive: true },
      ],
      setActive: vi.fn(),
    };
    render(<Select value="sick" onChange={onChange} />);
    open();

    const option = screen.getByRole("option", { name: /Sonderurlaub/ });
    expect(option).toBeDisabled();
    expect(option).toHaveAttribute("aria-disabled", "true");
    expect(
      screen.getByRole("button", { name: "Sonderurlaub wieder aktivieren" }),
    ).toBeTruthy();
    fireEvent.click(option);
    expect(onChange).not.toHaveBeenCalled();
  });
});
