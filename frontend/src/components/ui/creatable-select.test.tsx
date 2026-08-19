import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  CreatableSelect,
  type CreatableSelectOption,
} from "./creatable-select";

const OPTIONS: CreatableSelectOption[] = [
  { value: "sick", label: "Krank", fixed: true },
  { value: "vacation", label: "Urlaub", fixed: true },
  { value: "custom:7", label: "Regenerationstag" },
];

function open(): void {
  fireEvent.click(screen.getByRole("combobox"));
}

describe("CreatableSelect", () => {
  it("shows the selected option's label on the trigger", () => {
    render(
      <CreatableSelect
        value="custom:7"
        options={OPTIONS}
        onChange={vi.fn()}
        ariaLabel="Art der Abwesenheit"
      />,
    );
    expect(screen.getByRole("combobox")).toHaveTextContent("Regenerationstag");
  });

  it("filters the list by the typed text, ignoring case", () => {
    render(
      <CreatableSelect value="sick" options={OPTIONS} onChange={vi.fn()} />,
    );
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "regen" },
    });

    expect(
      screen.getByRole("button", { name: /Regenerationstag/ }),
    ).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Krank" })).toBeNull();
  });

  it("offers to add a name that does not exist yet and selects it", async () => {
    const onCreate = vi.fn().mockResolvedValue("custom:12");
    const onChange = vi.fn();
    render(
      <CreatableSelect
        value="sick"
        options={OPTIONS}
        onChange={onChange}
        onCreate={onCreate}
      />,
    );
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "  Ferienzeit  " },
    });

    fireEvent.click(screen.getByRole("button", { name: /Ferienzeit.*hinzuf/ }));

    await waitFor(() => expect(onCreate).toHaveBeenCalledWith("Ferienzeit"));
    await waitFor(() => expect(onChange).toHaveBeenCalledWith("custom:12"));
  });

  it("does not offer to add a name that already exists, in any case", () => {
    render(
      <CreatableSelect
        value="sick"
        options={OPTIONS}
        onChange={vi.fn()}
        onCreate={vi.fn()}
      />,
    );
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "REGENERATIONSTAG" },
    });

    expect(screen.queryByRole("button", { name: /hinzuf/ })).toBeNull();
  });

  it("hides every management affordance without the callbacks", () => {
    render(
      <CreatableSelect value="sick" options={OPTIONS} onChange={vi.fn()} />,
    );
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });

    expect(screen.queryByRole("button", { name: /hinzuf/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /umbenennen/i })).toBeNull();
  });

  it("never offers to rename or retire a fixed option", () => {
    render(
      <CreatableSelect
        value="sick"
        options={OPTIONS}
        onChange={vi.fn()}
        onRename={vi.fn()}
        onSetActive={vi.fn()}
      />,
    );
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
    const onRename = vi.fn().mockResolvedValue(undefined);
    render(
      <CreatableSelect
        value="sick"
        options={OPTIONS}
        onChange={vi.fn()}
        onRename={onRename}
      />,
    );
    open();
    fireEvent.click(
      screen.getByRole("button", { name: "Regenerationstag umbenennen" }),
    );
    fireEvent.change(screen.getByRole("textbox", { name: /Name von/ }), {
      target: { value: "Regenerationstage" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Namen speichern" }));

    await waitFor(() =>
      expect(onRename).toHaveBeenCalledWith("custom:7", "Regenerationstage"),
    );
  });

  it("surfaces a failed add instead of closing silently", async () => {
    const onCreate = vi
      .fn()
      .mockRejectedValue(new Error("Name ist bereits vergeben"));
    render(
      <CreatableSelect
        value="sick"
        options={OPTIONS}
        onChange={vi.fn()}
        onCreate={onCreate}
      />,
    );
    open();
    fireEvent.change(screen.getByRole("textbox"), {
      target: { value: "Ferienzeit" },
    });
    fireEvent.click(screen.getByRole("button", { name: /hinzuf/ }));

    expect(await screen.findByText("Name ist bereits vergeben")).toBeTruthy();
  });

  it("keeps a retired option visible while it is the current value", () => {
    const withRetired: CreatableSelectOption[] = [
      ...OPTIONS,
      { value: "custom:9", label: "Sonderurlaub", inactive: true },
    ];
    render(
      <CreatableSelect
        value="custom:9"
        options={withRetired}
        onChange={vi.fn()}
      />,
    );
    open();

    expect(screen.getByRole("button", { name: /Sonderurlaub/ })).toBeTruthy();
  });

  it("hides a retired option from someone who cannot restore it", () => {
    const withRetired: CreatableSelectOption[] = [
      ...OPTIONS,
      { value: "custom:9", label: "Sonderurlaub", inactive: true },
    ];
    render(
      <CreatableSelect value="sick" options={withRetired} onChange={vi.fn()} />,
    );
    open();

    expect(screen.queryByRole("button", { name: /Sonderurlaub/ })).toBeNull();
  });
});
