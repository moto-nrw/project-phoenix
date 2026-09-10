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
});
