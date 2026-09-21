import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  type HelpTableOfContentsItem,
  useHelpTableOfContents,
} from "./help-table-of-contents";

const ITEMS: readonly HelpTableOfContentsItem[] = [
  { id: "first" },
  { id: "second", children: [{ id: "second-detail" }] },
] as const;

function Harness() {
  const { activeSectionId, scrollToSection } = useHelpTableOfContents(ITEMS);
  return (
    <>
      <section id="first" data-top="-40" />
      <section id="second" data-top="320" />
      <section id="second-detail" data-top="640" />
      <nav>
        {ITEMS.flatMap((item) => [item, ...(item.children ?? [])]).map(
          (item) => (
            <a
              key={item.id}
              href={`#${item.id}`}
              aria-current={
                activeSectionId === item.id ? "location" : undefined
              }
              onClick={(event) => scrollToSection(event, item.id)}
            >
              {item.id}
            </a>
          ),
        )}
      </nav>
    </>
  );
}

describe("useHelpTableOfContents", () => {
  const scrollIntoView = vi.fn();

  beforeEach(() => {
    vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(
      function getBoundingClientRect(this: HTMLElement) {
        const top = Number(this.dataset.top ?? 0);
        return {
          x: 0,
          y: top,
          top,
          right: 0,
          bottom: top,
          left: 0,
          width: 0,
          height: 0,
          toJSON: () => ({}),
        };
      },
    );
    Object.defineProperty(Element.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });
    vi.stubGlobal("matchMedia", vi.fn().mockReturnValue({ matches: false }));
  });

  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    scrollIntoView.mockClear();
  });

  it("follows the section that reaches the reading position", async () => {
    render(<Harness />);

    await waitFor(() =>
      expect(screen.getByText("first")).toHaveAttribute(
        "aria-current",
        "location",
      ),
    );

    document.getElementById("second")!.dataset.top = "120";
    fireEvent.scroll(window);

    await waitFor(() =>
      expect(screen.getByText("second")).toHaveAttribute(
        "aria-current",
        "location",
      ),
    );
  });

  it("smoothly scrolls to a clicked section", () => {
    render(<Harness />);

    fireEvent.click(screen.getByText("second-detail"));

    expect(scrollIntoView).toHaveBeenCalledWith({
      behavior: "smooth",
      block: "start",
    });
    expect(window.location.hash).toBe("#second-detail");
  });
});
