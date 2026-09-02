import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("vaul", async () => {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        direction,
        onOpenChange,
      }: {
        children: React.ReactNode;
        direction?: string;
        onOpenChange?: (open: boolean) => void;
      }) => (
        <div data-direction={direction}>
          {/* Vaul schließt selbst bei Escape und Klick auf den Hintergrund und
              meldet das über onOpenChange. Das echte Vaul lässt sich in jsdom
              nicht fahren, prüfbar ist aber das, was uns gehört: dass der
              Rückruf beim Aufrufer ankommt. */}
          <button
            type="button"
            data-testid="vaul-dismiss"
            onClick={() => onOpenChange?.(false)}
          />
          {children}
        </div>
      ),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

import { SlideOver, SlideOverContent } from "./slide-over";

function StatefulContent() {
  const [value, setValue] = useState("");
  return (
    <input
      aria-label="Entwurf"
      value={value}
      onChange={(event) => setValue(event.target.value)}
    />
  );
}

describe("SlideOver", () => {
  let matches = false;
  let changeListener: (() => void) | undefined;

  beforeEach(() => {
    matches = false;
    changeListener = undefined;
    vi.stubGlobal(
      "matchMedia",
      vi.fn(() => ({
        get matches() {
          return matches;
        },
        addEventListener: (_event: string, listener: () => void) => {
          changeListener = listener;
        },
        removeEventListener: vi.fn(),
      })),
    );
  });

  it("uses the mobile direction for a panel already open at initial render", async () => {
    matches = true;

    render(
      <SlideOver defaultOpen>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    await waitFor(() => {
      expect(
        screen.getByText("Inhalt").closest("[data-direction]"),
      ).toHaveAttribute("data-direction", "bottom");
    });
  });

  it("dims and blurs the page with the shared overlay backdrop", () => {
    const { container } = render(
      <SlideOver open>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    const overlay = container.querySelector(".fixed.inset-0");
    expect(overlay).not.toBeNull();
    // Same tint as Modal, FormModal and Drawer — no slate-tinted or lighter
    // backdrop that would dim the page in a different gray (#2932).
    expect(overlay).toHaveClass("bg-black/40", "backdrop-blur-sm");
    expect(overlay?.className).not.toContain("bg-slate-900");
  });

  it("keeps unsaved child state when the viewport changes while open", () => {
    const { rerender } = render(
      <SlideOver open>
        <SlideOverContent>
          <StatefulContent />
        </SlideOverContent>
      </SlideOver>,
    );

    fireEvent.change(screen.getByLabelText("Entwurf"), {
      target: { value: "Ungespeichert" },
    });

    act(() => {
      matches = true;
      changeListener?.();
    });

    expect(screen.getByLabelText("Entwurf")).toHaveValue("Ungespeichert");
    expect(
      screen.getByLabelText("Entwurf").closest("[data-direction]"),
    ).toHaveAttribute("data-direction", "right");

    rerender(
      <SlideOver open={false}>
        <SlideOverContent>
          <StatefulContent />
        </SlideOverContent>
      </SlideOver>,
    );

    return waitFor(() => {
      expect(
        screen.getByLabelText("Entwurf").closest("[data-direction]"),
      ).toHaveAttribute("data-direction", "bottom");
    });
  });

  it("meldet ein Schließen durch Vaul (Escape, Klick auf den Hintergrund) an den Aufrufer", () => {
    // Beim Wechsel von Modal auf Panel ist diese Zusicherung je Verbrauchsstelle
    // entfallen, weil sie dort nicht mehr prüfbar war. Sie gehört ohnehin hierher:
    // einmal für alle Panels statt einmal je Aufrufer.
    const onOpenChange = vi.fn();

    render(
      <SlideOver open onOpenChange={onOpenChange}>
        <SlideOverContent>Inhalt</SlideOverContent>
      </SlideOver>,
    );

    fireEvent.click(screen.getByTestId("vaul-dismiss"));

    expect(onOpenChange).toHaveBeenCalledWith(false);
  });
});
