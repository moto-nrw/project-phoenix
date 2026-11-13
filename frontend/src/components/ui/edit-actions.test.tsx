import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { EditActions } from "./edit-actions";

describe("EditActions", () => {
  it("renders Abbrechen before Speichern and wires both", () => {
    const onCancel = vi.fn();
    const onSave = vi.fn();
    render(<EditActions onCancel={onCancel} onSave={onSave} />);

    const buttons = screen.getAllByRole("button");
    expect(buttons.map((button) => button.textContent)).toEqual([
      "Abbrechen",
      "Speichern",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledTimes(1);
  });

  it("locks both buttons while saving and shows the saving label", () => {
    render(<EditActions onCancel={vi.fn()} onSave={vi.fn()} saving />);

    expect(screen.getByRole("button", { name: "Speichert…" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
  });

  it("keeps Abbrechen usable when only the draft is invalid", () => {
    render(<EditActions onCancel={vi.fn()} onSave={vi.fn()} disabled />);

    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeEnabled();
  });

  it("submits the surrounding form when asked to", () => {
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <EditActions onCancel={vi.fn()} saveType="submit" />
      </form>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("submits the surrounding form without an onSave callback", () => {
    const onSubmit = vi.fn((event: React.FormEvent) => event.preventDefault());
    render(
      <form onSubmit={onSubmit}>
        <EditActions onCancel={vi.fn()} />
      </form>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("offers a second way to save between Abbrechen and Speichern", () => {
    const onSave = vi.fn();
    const alsoSave = vi.fn();
    const { rerender } = render(
      <EditActions
        onCancel={vi.fn()}
        onSave={onSave}
        alsoSave={{
          label: "Speichern und Team informieren",
          onClick: alsoSave,
          tooltip: "Speichert und informiert.",
        }}
      />,
    );
    const buttons = screen.getAllByRole("button");
    const position = (name: string) =>
      buttons.indexOf(screen.getByRole("button", { name }));
    expect(position("Abbrechen")).toBe(0);
    expect(position("Speichern und Team informieren")).toBe(1);
    expect(position("Speichern")).toBe(2);
    fireEvent.click(
      screen.getByRole("button", { name: "Speichern und Team informieren" }),
    );
    expect(alsoSave).toHaveBeenCalledTimes(1);
    expect(onSave).not.toHaveBeenCalled();

    rerender(
      <EditActions
        onCancel={vi.fn()}
        onSave={onSave}
        alsoSave={{
          label: "Speichern und Team informieren",
          onClick: alsoSave,
          tooltip: "Speichert und informiert.",
          disabledReason: "Erst etwas anderes wählen.",
        }}
      />,
    );
    const blocked = screen.getByRole("button", {
      name: "Speichern und Team informieren",
    });
    expect(blocked).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(blocked);
    expect(alsoSave).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });
});
