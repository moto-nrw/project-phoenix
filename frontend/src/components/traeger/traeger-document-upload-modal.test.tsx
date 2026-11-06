import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { TraegerDocumentUploadModal } from "./traeger-document-upload-modal";

vi.mock("~/lib/traeger-documents-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-documents-api")>()),
  uploadTraegerDocument: vi.fn(),
}));

// A refused file names its reason under the drop zone and a screen reader
// hears it as soon as it appears: the kit's Dropzone owns the file input
// (#4692 review, #5599).
it("announces the reason a file is refused", async () => {
  render(
    <ToastProvider>
      <TraegerDocumentUploadModal
        isOpen
        onClose={vi.fn()}
        onUploaded={vi.fn()}
      />
    </ToastProvider>,
  );

  fireEvent.change(screen.getByLabelText("Dokument auswählen"), {
    target: {
      files: [new File(["<html>"], "liste.html", { type: "text/html" })],
    },
  });

  const reason = await screen.findByText(
    "Erlaubt sind PDF, Word, Excel, PowerPoint und Bilder (PNG, JPEG).",
  );
  expect(reason).toHaveAttribute("role", "alert");
  expect(reason.id).toBe("traeger-document-file-error");
});

it("takes a file from the drop zone and offers another one", async () => {
  render(
    <ToastProvider>
      <TraegerDocumentUploadModal
        isOpen
        onClose={vi.fn()}
        onUploaded={vi.fn()}
      />
    </ToastProvider>,
  );

  fireEvent.change(screen.getByLabelText("Dokument auswählen"), {
    target: {
      files: [
        new File(["%PDF"], "hausordnung_2026.pdf", { type: "application/pdf" }),
      ],
    },
  });

  expect(await screen.findByText("hausordnung_2026.pdf")).toBeInTheDocument();
  expect(screen.getByLabelText(/Titel/)).toHaveValue("hausordnung 2026");
  fireEvent.click(screen.getByRole("button", { name: "Andere Datei" }));
  expect(screen.getByLabelText("Dokument auswählen")).toBeInTheDocument();
});
