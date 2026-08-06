import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DokumenteTab } from "./dokumente-tab";
import type { StaffDocumentList } from "~/lib/staff-documents-api";

const mutate = vi.hoisted(() => vi.fn());
const useSWRAuth = vi.hoisted(() => vi.fn());
const uploadMock = vi.hoisted(() => vi.fn());
const deleteMock = vi.hoisted(() => vi.fn());

vi.mock("~/lib/swr", () => ({
  useSWRAuth,
}));

vi.mock("~/lib/staff-documents-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/staff-documents-api")>();
  return {
    ...actual,
    staffDocumentsService: {
      list: vi.fn(),
      upload: uploadMock,
      delete: deleteMock,
      downloadUrl: (staffId: string, documentId: string) =>
        `/api/staff/${staffId}/documents/${documentId}/download`,
    },
  };
});

const sampleData: StaffDocumentList = {
  documents: [
    {
      id: "1",
      staffId: "42",
      category: "zeugnis",
      filename: "erste-hilfe.pdf",
      sizeBytes: 2048,
      contentType: "application/pdf",
      uploadedAt: "2026-07-01T10:00:00Z",
      retainUntil: null,
      reviewDue: null,
    },
    {
      id: "2",
      staffId: "42",
      category: "bewerbung",
      filename: "lebenslauf.pdf",
      sizeBytes: 4096,
      contentType: "application/pdf",
      uploadedAt: "2025-12-01T10:00:00Z",
      retainUntil: null,
      reviewDue: "2026-06-01",
    },
  ],
  visibleCategories: ["arbeitsvertrag", "zeugnis", "bewerbung", "sonstiges"],
};

function mockSWR(overrides: {
  data?: StaffDocumentList;
  error?: Error;
  isLoading?: boolean;
}) {
  useSWRAuth.mockImplementation(() => ({
    data: overrides.data,
    error: overrides.error,
    isLoading: overrides.isLoading ?? false,
    isValidating: false,
    mutate,
  }));
}

describe("DokumenteTab", () => {
  beforeEach(() => {
    mutate.mockReset();
    useSWRAuth.mockReset();
    uploadMock.mockReset();
    deleteMock.mockReset();
  });

  it("zeigt Dokumente mit Kategorie, Größe und Fristen-Hinweis", () => {
    mockSWR({ data: sampleData });
    render(<DokumenteTab staffId="42" />);

    expect(screen.getByText("erste-hilfe.pdf")).toBeInTheDocument();
    // Das Label erscheint mehrfach (Filter-Pill, Kategorie-Auswahl, Badge) —
    // entscheidend ist, dass es gerendert wird.
    expect(
      screen.getAllByText("Zeugnis / Qualifikation").length,
    ).toBeGreaterThan(0);
    expect(screen.getByText("2 KB")).toBeInTheDocument();
    // Bewerbungsunterlagen älter als 6 Monate tragen den überfälligen
    // Prüfhinweis (reviewDue 2026-06-01 liegt vor dem heutigen Datum).
    expect(
      screen.getByText(/Prüfung überfällig seit 01\.06\.2026/),
    ).toBeInTheDocument();
  });

  it("zeigt bei einem Ladefehler eine Fehlermeldung", () => {
    mockSWR({ error: new Error("request failed") });
    render(<DokumenteTab staffId="42" />);

    expect(
      screen.getByText("Dokumente konnten nicht geladen werden."),
    ).toBeInTheDocument();
  });

  it("zeigt den Leerzustand ohne Dokumente", () => {
    mockSWR({
      data: { documents: [], visibleCategories: ["sonstiges"] },
    });
    render(<DokumenteTab staffId="42" />);

    expect(screen.getByText("Noch keine Dokumente")).toBeInTheDocument();
  });

  it("filtert die Liste über die Kategorie-Pills", () => {
    mockSWR({ data: sampleData });
    render(<DokumenteTab staffId="42" />);

    fireEvent.click(
      screen.getByRole("button", { name: "Bewerbungsunterlagen" }),
    );
    expect(screen.queryByText("erste-hilfe.pdf")).not.toBeInTheDocument();
    expect(screen.getByText("lebenslauf.pdf")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Alle" }));
    expect(screen.getByText("erste-hilfe.pdf")).toBeInTheDocument();
  });

  it("lädt eine gewählte Datei mit der eingestellten Kategorie hoch", async () => {
    mockSWR({ data: sampleData });
    uploadMock.mockResolvedValue(undefined);
    render(<DokumenteTab staffId="42" />);

    const file = new File(["%PDF-1.4"], "vertrag.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("Dokument auswählen"), {
      target: { files: [file] },
    });

    await waitFor(() => {
      expect(uploadMock).toHaveBeenCalledWith("42", file, "arbeitsvertrag");
    });
    expect(mutate).toHaveBeenCalled();
  });

  it("zeigt eine zu große Datei als Fehler ohne Upload-Versuch", async () => {
    mockSWR({ data: sampleData });
    render(<DokumenteTab staffId="42" />);

    const big = new File([new ArrayBuffer(10 * 1024 * 1024 + 1)], "gross.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("Dokument auswählen"), {
      target: { files: [big] },
    });

    expect(
      await screen.findByText("Die Datei ist größer als 10 MB."),
    ).toBeInTheDocument();
    expect(uploadMock).not.toHaveBeenCalled();
  });

  it("löscht ein Dokument nach Bestätigung im Modal", async () => {
    mockSWR({ data: sampleData });
    deleteMock.mockResolvedValue(undefined);
    render(<DokumenteTab staffId="42" />);

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für erste-hilfe.pdf" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

    expect(screen.getByText("Dokument löschen")).toBeInTheDocument();

    // Zwei-Schritt-Gate: erst "Ja, löschen", dann die endgültige Bestätigung.
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    await waitFor(() => {
      expect(deleteMock).toHaveBeenCalledWith("42", "1");
    });
    expect(mutate).toHaveBeenCalled();
  });
});
