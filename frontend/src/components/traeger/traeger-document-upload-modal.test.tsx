import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { TraegerDocumentUploadModal } from "./traeger-document-upload-modal";

vi.mock("~/lib/traeger-documents-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/traeger-documents-api")>()),
  uploadTraegerDocument: vi.fn(),
}));

// A refused file names its reason at the file field itself, so a screen
// reader on the focused field hears why (#4692 review).
it("links the reason a file is refused to the file field", async () => {
  render(
    <ToastProvider>
      <TraegerDocumentUploadModal
        isOpen
        onClose={vi.fn()}
        onUploaded={vi.fn()}
      />
    </ToastProvider>,
  );
  const input = document.querySelector<HTMLInputElement>('input[type="file"]');
  if (!input) throw new Error("file input missing");

  fireEvent.change(input, {
    target: {
      files: [new File(["<html>"], "liste.html", { type: "text/html" })],
    },
  });

  const reason = await screen.findByText(
    "Erlaubt sind PDF, Word, Excel, PowerPoint und Bilder (PNG, JPEG).",
  );
  expect(reason.id).not.toBe("");
  expect(input.getAttribute("aria-describedby")?.split(" ")).toContain(
    reason.id,
  );
  expect(input).toHaveAttribute("aria-invalid", "true");
});
