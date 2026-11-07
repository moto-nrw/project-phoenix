import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovableChip } from "./removable-chip";

describe("RemovableChip", () => {
  it("removes through a button that names the entry", () => {
    const onRemove = vi.fn();
    render(
      <RemovableChip
        label="Klasse 1a"
        onRemove={onRemove}
        removeLabel="Klasse 1a entfernen"
      />,
    );

    expect(screen.getByText("Klasse 1a")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Klasse 1a entfernen" }),
    );
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("offers no button without onRemove", () => {
    render(<RemovableChip label="Klasse 1a" />);

    // Anker: der Eintrag steht da, nur ohne Kreuz.
    expect(screen.getByText("Klasse 1a")).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("does not remove while disabled", () => {
    const onRemove = vi.fn();
    render(
      <RemovableChip
        label="Klasse 1a"
        onRemove={onRemove}
        removeLabel="Klasse 1a entfernen"
        disabled
      />,
    );

    const button = screen.getByRole("button", { name: "Klasse 1a entfernen" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onRemove).not.toHaveBeenCalled();
  });

  it("shows icon and detail next to the text", () => {
    render(
      <RemovableChip
        label="Impfpass.pdf"
        detail="1,2 MB"
        icon={<span data-testid="file-icon" />}
      />,
    );

    expect(screen.getByTestId("file-icon")).toBeInTheDocument();
    expect(screen.getByText("1,2 MB")).toBeInTheDocument();
  });
});
