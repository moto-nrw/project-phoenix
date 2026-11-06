import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CopyField } from "./copy-field";

const writeText = vi.fn<(text: string) => Promise<void>>();

describe("CopyField", () => {
  beforeEach(() => {
    writeText.mockReset();
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: { writeText },
    });
  });

  it("shows the whole value and copies it", async () => {
    writeText.mockResolvedValue();
    render(<CopyField value="geheimer-schluessel" monospace />);

    expect(screen.getByText("geheimer-schluessel")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Kopieren" }));

    await screen.findByRole("button", { name: "Kopiert" });
    expect(writeText).toHaveBeenCalledExactlyOnceWith("geheimer-schluessel");
  });

  it("names the object when a label is given", () => {
    render(
      <CopyField value="https://example.test" copyLabel="Link kopieren" />,
    );

    expect(
      screen.getByRole("button", { name: "Link kopieren" }),
    ).toBeInTheDocument();
  });

  it("reports a refused clipboard and does not claim success", async () => {
    writeText.mockRejectedValue(new Error("denied"));
    const onCopyFailed = vi.fn();
    render(<CopyField value="wert" onCopyFailed={onCopyFailed} />);

    fireEvent.click(screen.getByRole("button", { name: "Kopieren" }));

    await waitFor(() => expect(onCopyFailed).toHaveBeenCalledTimes(1));
    expect(
      screen.getByRole("button", { name: "Kopieren" }),
    ).toBeInTheDocument();
    expect(screen.getByText("wert")).toBeInTheDocument();
  });
});
