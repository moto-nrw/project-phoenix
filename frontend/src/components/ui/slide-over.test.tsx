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
      }: {
        children: React.ReactNode;
        direction?: string;
      }) => <div data-direction={direction}>{children}</div>,
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
});
