import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { AttendanceExportDialog } from "./attendance-export-dialog";

const { openBlobForPrint } = vi.hoisted(() => ({
  openBlobForPrint: vi.fn(),
}));

vi.mock("vaul", () => import("~/test/mocks/vaul"));
vi.mock("~/lib/file-download", () => ({
  downloadBlob: vi.fn(),
  filenameFromDisposition: () => null,
  openBlobForPrint,
}));

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.restoreAllMocks();
});

describe("AttendanceExportDialog", () => {
  it("limits the native date fields to the visible days", () => {
    render(
      <ToastProvider>
        <AttendanceExportDialog
          isOpen
          onClose={vi.fn()}
          studentId="42"
          studentName="Mia Muster"
          today="2026-09-09"
          visibleDays={30}
        />
      </ToastProvider>,
    );

    const from = screen.getByLabelText<HTMLInputElement>("Von");
    const to = screen.getByLabelText<HTMLInputElement>("Bis");

    expect(from).toHaveAttribute("type", "date");
    expect(from).toHaveValue("2026-08-11");
    expect(from).toHaveAttribute("min", "2026-08-11");
    expect(from).toHaveAttribute("max", "2026-09-09");
    expect(to).toHaveAttribute("min", "2026-08-11");
    expect(to).toHaveAttribute("max", "2026-09-09");

    fireEvent.change(from, { target: { value: "2026-08-20" } });

    expect(to).toHaveAttribute("min", "2026-08-20");
  });

  it("prints the PDF in the tab opened before the export request", async () => {
    const target = {} as Window;
    vi.spyOn(globalThis, "open").mockReturnValue(target);
    globalThis.fetch = vi.fn(async () => new Response(new Blob(["pdf"])));

    render(
      <ToastProvider>
        <AttendanceExportDialog
          isOpen
          onClose={vi.fn()}
          studentId="42"
          studentName="Mia Muster"
          today="2026-09-09"
          visibleDays={30}
        />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    await waitFor(() =>
      expect(openBlobForPrint).toHaveBeenCalledWith(expect.any(Blob), target),
    );
  });
});
