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
