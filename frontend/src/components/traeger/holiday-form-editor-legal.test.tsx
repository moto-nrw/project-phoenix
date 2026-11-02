import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { FormLegalBlock } from "~/lib/enrollment-form-schema-api";
import type { HolidayFormField } from "~/lib/holiday-form-field";

const { save, upload, remove, schoolUpload } = vi.hoisted(() => ({
  save: vi.fn(),
  upload: vi.fn(),
  remove: vi.fn(),
  schoolUpload: vi.fn(),
}));
vi.mock("~/lib/traeger-holiday-form-api", async (original) => ({
  ...(await original<object>()),
  saveHolidayForm: (...args: unknown[]) => save(...args),
  uploadHolidayDocument: (...args: unknown[]) => upload(...args),
  deleteHolidayDocument: (...args: unknown[]) => remove(...args),
}));
vi.mock("~/lib/enrollment-form-schema-api", async (original) => ({
  ...(await original<object>()),
  uploadEnrollmentLegalDocument: schoolUpload,
}));
import { HolidayFormEditor } from "./holiday-form-editor";

const form = {
  version: 1,
  fields: [],
  legalBlocks: [],
  template: [],
  createdAt: "2026-09-01T10:00:00Z",
};

describe("Holiday form legal blocks", () => {
  beforeEach(() => vi.clearAllMocks());

  // Invariants: pending uploads block save and discard. Once settled,
  // discarding resets the saved form and removes every uploaded draft.
  it("blocks discarding until an in-flight PDF is settled, then discards its draft", async () => {
    let finishUpload!: (id: string) => void;
    upload.mockReturnValueOnce(
      new Promise<string>((resolve) => {
        finishUpload = resolve;
      }),
    );
    remove.mockResolvedValue(undefined);
    const savedBlock: FormLegalBlock = {
      key: "brief",
      kind: "consent",
      source: "custom",
      enabled: true,
      required: false,
      title: "Ferienbrief",
      label: "Brief gelesen",
      text: "Der gespeicherte Wortlaut.",
      sort_order: 0,
      attachments: [],
    };
    render(
      <ToastProvider>
        <HolidayFormEditor
          periodId="5"
          form={{ ...form, legalBlocks: [savedBlock] }}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    const discard = screen.getByRole("button", {
      name: "Änderungen verwerfen",
    });
    expect(discard).toBeEnabled();
    fireEvent.change(screen.getByLabelText("PDF-Dateien auswählen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "entwurf.pdf", { type: "application/pdf" }),
        ],
      },
    });
    expect(screen.getByText("PDFs werden hochgeladen…")).toBeInTheDocument();
    expect(discard).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    ).toBeDisabled();
    await act(async () => finishUpload("90"));
    expect(screen.getByDisplayValue("entwurf")).toBeInTheDocument();
    expect(discard).toBeEnabled();
    fireEvent.click(discard);
    await waitFor(() =>
      expect(remove).toHaveBeenCalledWith("5", "90", { keepalive: true }),
    );
    expect(screen.getByLabelText("Rechtstext / Erklärung")).toHaveValue(
      "Der gespeicherte Wortlaut.",
    );
    expect(screen.getByText("Noch keine PDFs angehängt.")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    ).toBeDisabled();
  });

  it("saves two required PDF notices and a voluntary text consent through the Träger upload adapter", async () => {
    upload.mockResolvedValueOnce("81").mockResolvedValueOnce("82");
    save.mockImplementation(
      (_period: string, fields: HolidayFormField[], blocks: FormLegalBlock[]) =>
        Promise.resolve({ ...form, version: 2, fields, legalBlocks: blocks }),
    );
    const onSaved = vi.fn();
    render(
      <ToastProvider>
        <HolidayFormEditor periodId="5" form={form} onSaved={onSaved} />
      </ToastProvider>,
    );
    for (let i = 0; i < 2; i++) {
      fireEvent.click(
        screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
      );
      fireEvent.change(screen.getAllByLabelText("Titel")[i]!, {
        target: { value: `Hinweis ${i + 1}` },
      });
      fireEvent.click(
        screen.getAllByRole("checkbox", { name: "Muss bestätigt werden" })[i]!,
      );
      const file = new File(["%PDF-1.4"], `hinweis-${i + 1}.pdf`, {
        type: "application/pdf",
      });
      fireEvent.change(screen.getAllByLabelText("PDF-Dateien auswählen")[i]!, {
        target: { files: [file] },
      });
      await waitFor(() => expect(upload).toHaveBeenCalledTimes(i + 1));
      await waitFor(() =>
        expect(screen.getAllByDisplayValue(`hinweis-${i + 1}`)).toHaveLength(1),
      );
    }
    fireEvent.click(
      screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
    );
    fireEvent.change(screen.getAllByLabelText("Titel")[2]!, {
      target: { value: "Fotos" },
    });
    fireEvent.change(screen.getAllByLabelText("Rechtstext / Erklärung")[2]!, {
      target: { value: "Die Fotos bleiben in der OGS." },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Als neue Version speichern" }),
    );
    await waitFor(() => expect(onSaved).toHaveBeenCalledTimes(1));
    expect(save).toHaveBeenCalledWith(
      "5",
      [],
      expect.arrayContaining([
        expect.objectContaining({
          title: "Hinweis 1",
          kind: "terms",
          required: true,
          attachments: [{ label: "hinweis-1", document_url: "81" }],
        }),
        expect.objectContaining({
          title: "Hinweis 2",
          kind: "terms",
          required: true,
          attachments: [{ label: "hinweis-2", document_url: "82" }],
        }),
        expect.objectContaining({
          title: "Fotos",
          kind: "consent",
          required: false,
          text: "Die Fotos bleiben in der OGS.",
        }),
      ]),
    );
    // Anker: the complete form was saved with both Träger uploads.
    expect(upload).toHaveBeenNthCalledWith(1, "5", expect.any(File));
    expect(upload).toHaveBeenNthCalledWith(2, "5", expect.any(File));
    expect(schoolUpload).not.toHaveBeenCalled();
  });

  it("allows discarding again when an upload fails", async () => {
    let failUpload!: (error: Error) => void;
    upload.mockReturnValueOnce(
      new Promise<string>((_resolve, reject) => {
        failUpload = reject;
      }),
    );
    render(
      <ToastProvider>
        <HolidayFormEditor periodId="5" form={form} onSaved={vi.fn()} />
      </ToastProvider>,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText("PDF-Dateien auswählen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "fehler.pdf", { type: "application/pdf" }),
        ],
      },
    });
    const discard = screen.getByRole("button", {
      name: "Änderungen verwerfen",
    });
    expect(discard).toBeDisabled();
    await act(async () => failUpload(new Error("Upload fehlgeschlagen")));
    expect(discard).toBeEnabled();
  });
});
