import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { StudentDokumenteTab } from "./dokumente-tab";

const upload = vi.fn();
const mutate = vi.fn(async () => undefined);
const swrResult: { data: unknown; isLoading: boolean; error: unknown } = {
  data: undefined,
  isLoading: false,
  error: undefined,
};

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ ...swrResult, mutate }),
}));

vi.mock("~/lib/student-documents-api", () => ({
  studentDocumentsService: {
    upload: (...args: unknown[]) => upload(...args) as unknown,
    delete: vi.fn(),
    downloadUrl: () => "/download",
  },
}));

function renderTab() {
  return render(
    <ToastProvider>
      <StudentDokumenteTab studentId="7" />
    </ToastProvider>,
  );
}

function chooseFile(file: File) {
  const input = document.querySelector<HTMLInputElement>('input[type="file"]')!;
  fireEvent.change(input, { target: { files: [file] } });
}

describe("StudentDokumenteTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrResult.error = undefined;
    swrResult.data = {
      documents: [],
      visibleCategories: [{ value: "attest", label: "Attest" }],
    };
  });

  it("shows a failed load in place with a retry", async () => {
    swrResult.data = undefined;
    swrResult.error = new ApiError("down", 503, {
      code: "general.unavailable",
    });
    renderTab();

    expect(
      await screen.findByText(
        "Die Liste der Dokumente ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalledOnce();
  });

  it("reports a refused upload as a toast that names the document", async () => {
    upload.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    renderTab();

    chooseFile(new File(["x"], "Attest.pdf", { type: "application/pdf" }));

    expect(
      await screen.findByText(
        "Für das Dokument fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      ),
    ).toBeInTheDocument();
  });

  it("stops a file over 10 MB before sending it", async () => {
    renderTab();
    const big = new File(["x"], "Gross.pdf", { type: "application/pdf" });
    Object.defineProperty(big, "size", { value: 11 * 1024 * 1024 });

    chooseFile(big);

    expect(
      await screen.findByText(
        "Die Datei ist größer als 10 MB. Bitte wählen Sie eine kleinere Datei.",
      ),
    ).toBeInTheDocument();
    expect(upload).not.toHaveBeenCalled();
  });
});
