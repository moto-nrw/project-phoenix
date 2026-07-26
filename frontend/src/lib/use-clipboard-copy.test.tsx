import "@testing-library/jest-dom/vitest";
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useClipboardCopy } from "./use-clipboard-copy";

function Probe({ text }: { text: string }) {
  const { copied, copy } = useClipboardCopy("ProbeComponent", 1500);
  return (
    <button type="button" data-testid="probe" onClick={() => void copy(text)}>
      {copied ? "Kopiert" : "Kopieren"}
    </button>
  );
}

describe("useClipboardCopy", () => {
  const writeText = vi.fn(() => Promise.resolve());

  beforeEach(() => {
    vi.useFakeTimers();
    writeText.mockReset();
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("writes the text and toggles copied for the reset window", async () => {
    writeText.mockResolvedValueOnce(undefined);
    render(<Probe text="hello" />);
    const button = screen.getByTestId("probe");

    expect(button).toHaveTextContent("Kopieren");

    await act(async () => {
      button.click();
    });

    expect(writeText).toHaveBeenCalledWith("hello");
    expect(button).toHaveTextContent("Kopiert");

    act(() => {
      vi.advanceTimersByTime(1500);
    });
    expect(button).toHaveTextContent("Kopieren");
  });

  it("logs a warning and stays uncopied when the clipboard write fails", async () => {
    writeText.mockRejectedValueOnce(new Error("denied"));
    const consoleWarn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<Probe text="secret" />);

    await act(async () => {
      screen.getByTestId("probe").click();
    });

    expect(screen.getByTestId("probe")).toHaveTextContent("Kopieren");
    expect(consoleWarn).toHaveBeenCalledWith("clipboard_copy_failed", {
      error: "denied",
    });
    consoleWarn.mockRestore();
  });

  it("stringifies non-Error rejection reasons before logging", async () => {
    writeText.mockRejectedValueOnce("permission-denied");
    const consoleWarn = vi.spyOn(console, "warn").mockImplementation(() => {});
    render(<Probe text="payload" />);

    await act(async () => {
      screen.getByTestId("probe").click();
    });

    expect(screen.getByTestId("probe")).toHaveTextContent("Kopieren");
    expect(consoleWarn).toHaveBeenCalledWith("clipboard_copy_failed", {
      error: "permission-denied",
    });
    consoleWarn.mockRestore();
  });
});
