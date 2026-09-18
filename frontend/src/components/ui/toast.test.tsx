import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { Toast } from "./toast";

describe("Toast", () => {
  it("uses the shared alert surface and kit controls", () => {
    const onAction = vi.fn();
    const onClose = vi.fn();

    render(
      <Toast
        type="success"
        message="Gespeichert"
        accessibleLabel="Erfolgreich: Gespeichert"
        closeLabel="Schließen"
        onClose={onClose}
        action={{
          label: "Rückgängig",
          accessibleLabel: "Tippen zum Rückgängig",
          onClick: onAction,
        }}
        visible
        reducedMotion
        touchFriendly
      />,
    );

    const status = screen.getByRole("status", {
      name: "Erfolgreich: Gespeichert",
    });
    expect(status).toHaveClass("bg-moto-green-soft");
    expect(status).toHaveClass("rounded-xl");
    expect(status).toHaveAttribute("aria-live", "polite");
    expect(status).toHaveAttribute("aria-atomic", "true");
    expect(
      screen.getByRole("button", { name: "Tippen zum Rückgängig" }),
    ).toHaveClass("h-11");
    expect(screen.getByRole("button", { name: "Schließen" })).toHaveClass(
      "h-11",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Tippen zum Rückgängig" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Schließen" }));

    expect(onAction).toHaveBeenCalledOnce();
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("keeps a hidden toast mounted for its exit transition", () => {
    const { container } = render(
      <Toast
        type="info"
        message="Hinweis"
        accessibleLabel="Information: Hinweis"
        closeLabel="Schließen"
        onClose={() => undefined}
        visible={false}
        reducedMotion={false}
        touchFriendly={false}
      />,
    );

    expect(container.firstElementChild).toHaveClass(
      "translate-y-2",
      "opacity-0",
      "duration-300",
    );
  });
});
