import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { useExportRun } from "~/hooks/use-export-run";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { ExportDialog, type ExportFileFormat } from "./export-dialog";

// Der Dialog ist ein SlideOver; Vaul rendert in jsdom nichts.
vi.mock("vaul", () => import("~/test/mocks/vaul"));

interface HarnessProps {
  readonly onDownload: (
    format: ExportFileFormat,
    title: string,
  ) => Promise<void>;
  readonly onPrint?: (target: Window) => Promise<void>;
  readonly formats?: readonly ExportFileFormat[];
  readonly appliedFilters?: readonly string[];
  readonly blocked?: boolean;
  readonly onClose?: () => void;
}

const ALL_FORMATS: readonly ExportFileFormat[] = ["pdf", "docx", "xlsx"];
const noop = () => undefined;

function Harness({
  onDownload,
  onPrint,
  formats = ALL_FORMATS,
  appliedFilters,
  blocked,
  onClose = noop,
}: HarnessProps) {
  const [title, setTitle] = useState("Liste");
  const run = useExportRun({
    isOpen: true,
    onDownload: (format: ExportFileFormat) => onDownload(format, title),
    onPrint,
    errorObject: "die Liste",
    onDone: onClose,
    logName: "test",
  });
  return (
    <ExportDialog
      isOpen
      onClose={onClose}
      title="Liste exportieren"
      appliedFilters={appliedFilters}
      formats={formats}
      run={run}
      blocked={blocked}
    >
      <input
        aria-label="Titel"
        value={title}
        onChange={(event) => setTitle(event.target.value)}
      />
    </ExportDialog>
  );
}

function renderHarness(props: HarnessProps) {
  render(
    <ToastProvider>
      <Harness {...props} />
    </ToastProvider>,
  );
}

afterEach(() => {
  vi.restoreAllMocks();
});

describe("ExportDialog", () => {
  it("downloads in the chosen format and closes", async () => {
    const onDownload = vi.fn().mockResolvedValue(undefined);
    const onClose = vi.fn();
    renderHarness({ onDownload, onClose });

    // Formatnamen nach Regel 3, nicht die Dateiendungen.
    fireEvent.click(screen.getByRole("button", { name: "Word" }));
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    await waitFor(() =>
      expect(onDownload).toHaveBeenCalledWith("docx", "Liste"),
    );
    expect(
      await screen.findByText("Die Datei ist heruntergeladen."),
    ).toBeInTheDocument();
    expect(onClose).toHaveBeenCalled();
  });

  it("shows no format choice when there is only one format", () => {
    renderHarness({ onDownload: vi.fn(), formats: ["pdf"] });

    // Anker: der Dialog ist offen.
    expect(
      screen.getByRole("heading", { name: "Liste exportieren" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Format")).not.toBeInTheDocument();
  });

  it("lists the filters it took over from the page", () => {
    renderHarness({
      onDownload: vi.fn(),
      appliedFilters: ["Klasse 3a", '"Mia"'],
    });

    expect(screen.getByText("Filter der Seite")).toBeInTheDocument();
    expect(screen.getByText("Klasse 3a")).toBeInTheDocument();
    expect(screen.getByText('"Mia"')).toBeInTheDocument();
  });

  it("waits while the options cannot produce a file", () => {
    renderHarness({
      onDownload: vi.fn(),
      onPrint: vi.fn(),
      blocked: true,
    });

    expect(
      screen.getByRole("button", { name: "Herunterladen" }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Drucken" })).toBeDisabled();
  });

  it("opens the print tab inside the click and hands it over", async () => {
    const tab = { closed: false, close: vi.fn() } as unknown as Window;
    const open = vi.spyOn(globalThis, "open").mockReturnValue(tab);
    const onPrint = vi.fn().mockResolvedValue(undefined);
    const onDownload = vi.fn();
    renderHarness({ onDownload, onPrint });

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    expect(open).toHaveBeenCalledWith("", "_blank");
    await waitFor(() => expect(onPrint).toHaveBeenCalledWith(tab));
    expect(onDownload).not.toHaveBeenCalled();
  });

  it("explains a blocked print window in the dialog", async () => {
    vi.spyOn(globalThis, "open").mockReturnValue(null);
    const onPrint = vi.fn();
    renderHarness({ onDownload: vi.fn(), onPrint });

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    expect(
      await screen.findByText(
        "Das Druckfenster ging nicht auf. Bitte erlauben Sie neue Fenster für moto.",
      ),
    ).toBeInTheDocument();
    expect(onPrint).not.toHaveBeenCalled();
  });

  it("keeps a failed print in the dialog and closes its tab", async () => {
    const tab = { closed: false, close: vi.fn() } as unknown as Window;
    vi.spyOn(globalThis, "open").mockReturnValue(tab);
    const onPrint = vi
      .fn()
      .mockRejectedValue(
        new ApiError("boom", 500, { code: "general.server", instance: "r1" }),
      );
    const onClose = vi.fn();
    renderHarness({ onDownload: vi.fn(), onPrint, onClose });

    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    const message = catalogText("general.server", "die Liste");
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(tab.close).toHaveBeenCalledOnce();
  });

  it("keeps a failure in the dialog and retries with the current options", async () => {
    const onDownload = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server", instance: "r1" }),
      )
      .mockResolvedValueOnce(undefined);
    const onClose = vi.fn();
    renderHarness({ onDownload, onClose });

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    const message = catalogText("general.server", "die Liste");
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toContainElement(
      screen.getByText(message),
    );
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Neue Liste" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Word" }));
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(onDownload).toHaveBeenCalledTimes(2));
    expect(onDownload).toHaveBeenLastCalledWith("docx", "Neue Liste");
    await waitFor(() => expect(onClose).toHaveBeenCalled());
  });
});
