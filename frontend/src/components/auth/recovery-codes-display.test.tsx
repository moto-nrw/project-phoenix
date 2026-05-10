import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { RecoveryCodesDisplay } from "./recovery-codes-display";

const codes = [
  "abcd-efgh-ijkl-mnop",
  "1234-5678-9abc-def0",
  "qrst-uvwx-yzab-cdef",
];

describe("RecoveryCodesDisplay", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders the title and all codes", () => {
    render(<RecoveryCodesDisplay codes={codes} onConfirm={vi.fn()} />);
    expect(
      screen.getByRole("heading", { name: "Wiederherstellungscodes" }),
    ).toBeInTheDocument();
    for (const code of codes) {
      expect(screen.getByText(code)).toBeInTheDocument();
    }
  });

  it("disables the confirm button until the checkbox is ticked", () => {
    const onConfirm = vi.fn();
    render(<RecoveryCodesDisplay codes={codes} onConfirm={onConfirm} />);

    const button = screen.getByRole("button", {
      name: "Weiter zum Dashboard",
    });
    expect(button).toBeDisabled();

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    expect(button).not.toBeDisabled();

    fireEvent.click(button);
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  it("copies codes to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });

    render(<RecoveryCodesDisplay codes={codes} onConfirm={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "In Zwischenablage kopieren" }),
    );

    await waitFor(() => {
      expect(writeText).toHaveBeenCalledWith(codes.join("\n"));
    });
    await waitFor(() => {
      expect(screen.getByText("Kopiert!")).toBeInTheDocument();
    });
  });

  it("triggers a download with the recovery codes file", () => {
    const createObjectURL = vi.fn(() => "blob:mock");
    const revokeObjectURL = vi.fn();
    Object.assign(URL, { createObjectURL, revokeObjectURL });

    const clickSpy = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    render(<RecoveryCodesDisplay codes={codes} onConfirm={vi.fn()} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Als .txt herunterladen" }),
    );

    expect(createObjectURL).toHaveBeenCalled();
    expect(clickSpy).toHaveBeenCalled();
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:mock");
    expect(screen.getByText("Heruntergeladen ✓")).toBeInTheDocument();

    clickSpy.mockRestore();
  });
});
