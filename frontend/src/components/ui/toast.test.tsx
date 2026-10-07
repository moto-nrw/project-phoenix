import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { Toast } from "./toast";

const originalClipboard = Object.getOwnPropertyDescriptor(
  navigator,
  "clipboard",
);

afterEach(() => {
  if (originalClipboard) {
    Object.defineProperty(navigator, "clipboard", originalClipboard);
  } else {
    Reflect.deleteProperty(navigator, "clipboard");
  }
});

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
        copySucceededLabel="Vorgangskennung kopiert."
        copyFailedLabel="Kopieren nicht möglich."
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

  // Das X stand in der Aktionsreihe und brach in der schmalen Toast-Breite
  // allein in eine eigene Zeile um. Es sitzt jetzt oben rechts, getrennt von
  // Wiederholen und Vorgangskennung.
  it("keeps the close button in the top corner, apart from the actions", () => {
    render(
      <Toast
        type="error"
        message="Die Einstellung konnte nicht bearbeitet werden."
        accessibleLabel="Fehler: Die Einstellung konnte nicht bearbeitet werden."
        closeLabel="Schließen"
        onClose={vi.fn()}
        copySucceededLabel="Vorgangskennung kopiert."
        copyFailedLabel="Kopieren nicht möglich."
        copyRequestIdLabel="Vorgangskennung kopieren"
        requestId="req-1"
        requestIdLabel="Vorgangskennung: req-1"
        action={{
          label: "Wiederholen",
          accessibleLabel: "Wiederholen",
          onClick: vi.fn(),
        }}
        visible
        reducedMotion
        touchFriendly={false}
      />,
    );

    const close = screen.getByRole("button", { name: "Schließen" });
    const retry = screen.getByRole("button", { name: "Wiederholen" });
    expect(close).toHaveClass("absolute", "top-2.5", "right-2.5");
    expect(retry.parentElement).not.toContainElement(close);
    expect(screen.getByRole("alert")).toHaveClass("pr-12");
  });

  it("keeps a hidden toast mounted for its exit transition", () => {
    const { container } = render(
      <Toast
        type="info"
        message="Hinweis"
        accessibleLabel="Information: Hinweis"
        closeLabel="Schließen"
        onClose={() => undefined}
        copySucceededLabel="Vorgangskennung kopiert."
        copyFailedLabel="Kopieren nicht möglich."
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

  it("announces a successful copy", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
    render(
      <Toast
        type="error"
        message="Bitte versuchen Sie es später erneut."
        accessibleLabel="Fehler"
        closeLabel="Schließen"
        onClose={() => undefined}
        requestId="req-20"
        copyRequestIdLabel="Vorgangskennung kopieren"
        copySucceededLabel="Vorgangskennung kopiert."
        copyFailedLabel="Kopieren nicht möglich."
        visible
        reducedMotion
        touchFriendly={false}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    );
    expect(writeText).toHaveBeenCalledWith("req-20");
    expect(await screen.findByRole("status")).toHaveTextContent(
      "Vorgangskennung kopiert.",
    );
  });

  it.each(["missing", "rejected"])(
    "announces when copying is %s",
    async (clipboardState) => {
      Object.defineProperty(navigator, "clipboard", {
        configurable: true,
        value:
          clipboardState === "missing"
            ? undefined
            : { writeText: vi.fn().mockRejectedValue(new Error("Denied")) },
      });
      render(
        <Toast
          type="error"
          message="Bitte versuchen Sie es später erneut."
          accessibleLabel="Fehler"
          closeLabel="Schließen"
          onClose={() => undefined}
          requestId="req-20"
          copyRequestIdLabel="Vorgangskennung kopieren"
          copySucceededLabel="Vorgangskennung kopiert."
          copyFailedLabel="Kopieren nicht möglich."
          visible
          reducedMotion
          touchFriendly={false}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
      );
      expect(await screen.findByRole("status")).toHaveTextContent(
        "Kopieren nicht möglich.",
      );
    },
  );
});
