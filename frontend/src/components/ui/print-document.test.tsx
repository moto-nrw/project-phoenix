import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { PrintButton, PrintDocument } from "./print-document";

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("PrintDocument", () => {
  it("keeps the screen copy in place and adds one print copy directly under body", () => {
    const { container } = render(
      <div className="shell">
        <PrintDocument>
          <p>Nachweis</p>
        </PrintDocument>
      </div>,
    );

    // Screen copy stays inside the page and is hidden only when printing.
    const screenCopy = container.querySelector(".shell > .print\\:hidden");
    expect(screenCopy).toHaveTextContent("Nachweis");

    // The print copy sits directly under <body>, outside every shell element,
    // and is invisible to assistive technology.
    const printCopies = document.querySelectorAll(
      "body > .moto-print-document",
    );
    expect(printCopies).toHaveLength(1);
    expect(printCopies[0]).toHaveAttribute("aria-hidden", "true");
    expect(printCopies[0]).toHaveTextContent("Nachweis");
  });

  it("removes the print copy with the page", () => {
    const { unmount } = render(
      <PrintDocument>
        <p>Nachweis</p>
      </PrintDocument>,
    );
    unmount();
    expect(document.querySelector(".moto-print-document")).toBeNull();
  });
});

describe("PrintButton", () => {
  it("opens the browser's print dialog", () => {
    // happy-dom has no window.print.
    const print = vi.fn();
    vi.stubGlobal("print", print);
    render(<PrintButton label="Drucken / als PDF speichern" />);

    fireEvent.click(
      screen.getByRole("button", { name: "Drucken / als PDF speichern" }),
    );
    expect(print).toHaveBeenCalledTimes(1);
  });
});
