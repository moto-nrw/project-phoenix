import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { DeclarationProofPage } from "./declaration-proof-page";
import * as parentApi from "~/lib/parent-api";
import { ParentApiError, type ParentDeclarationProof } from "~/lib/parent-api";

const searchParams = vi.hoisted(() => new URLSearchParams("student=5"));

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  usePathname: () => "/news/42/nachweis",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

const proof: ParentDeclarationProof = {
  title: "Ausflug in den Zoo",
  school_name: "OGS Am Berg",
  child_name: "Mia Muster",
  kind: "consent",
  method_label: "Einfache elektronische Erklärung im angemeldeten Eltern-Konto",
  generated_at: "2026-09-09T10:00:00Z",
  integrity_ok: true,
  versions: [
    {
      id: "12",
      version_no: 2,
      title: "Ausflug in den Zoo",
      body: "Wir fahren am Freitag in den Zoo.",
      content_hash: "a".repeat(64),
      published_at: "2026-09-01T08:00:00Z",
      attachments: [
        {
          id: "19",
          filename: "Ausflug.pdf",
          content_type: "application/pdf",
          size_bytes: 2048,
          sha256: "b".repeat(64),
        },
      ],
    },
  ],
  submissions: [
    {
      id: "77",
      action: "agreed",
      submitted_at: "2026-09-02T06:15:00Z",
      signer_name: "Klaus Schneider",
      guardian_role: "primary_guardian",
      version_no: 2,
      password_confirmed: true,
      method: "simple_electronic",
      content_hash: "a".repeat(64),
      record_hash: "c".repeat(64),
    },
  ],
};

beforeEach(() => {
  searchParams.set("student", "5");
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("Nachweis eines Einverständnisses im Eltern-Portal (#3430)", () => {
  it("shows the full text, own history, the integrity sentence and no checksums", async () => {
    const load = vi
      .spyOn(parentApi, "fetchDeclarationProof")
      .mockResolvedValue(proof);

    render(<DeclarationProofPage announcementId="42" />);

    expect(
      await screen.findByRole("button", { name: "Als PDF herunterladen" }),
    ).toBeInTheDocument();
    expect(load).toHaveBeenCalledWith("42", "5");
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Nachweis Ihrer Antwort",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Mia Muster")).toBeInTheDocument();
    expect(
      screen.getByText("Wir fahren am Freitag in den Zoo."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Zugestimmt am 02.09.2026, 08:15 Uhr"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Antwort von Klaus Schneider (Hauptberechtigt)"),
    ).toBeInTheDocument();
    expect(screen.getByText(/Mit Passwort bestätigt/)).toBeInTheDocument();
    expect(screen.getByText(/Ausflug\.pdf/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Ausflug.pdf" })).toHaveAttribute(
      "href",
      "/api/parent/me/news/42/attachments/19/download?student_id=5&inline=1",
    );
    expect(
      screen.getByText(
        "Text und Antworten sind seit der Veröffentlichung unverändert.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Dieser Nachweis belegt eine einfache elektronische Erklärung. Er ersetzt keine gesetzlich vorgeschriebene Unterschrift auf Papier.",
      ),
    ).toBeInTheDocument();

    // No checksum and no print view anywhere.
    const text = document.body.textContent ?? "";
    for (const hash of ["a", "b", "c"].map((c) => c.repeat(64))) {
      expect(text).not.toContain(hash);
    }
    expect(text).not.toMatch(/SHA-256|Prüfsumme/);
    expect(document.querySelector(".moto-print-document")).toBeNull();
    expect(
      screen.queryByRole("button", { name: /Drucken/ }),
    ).not.toBeInTheDocument();
  });

  it("warns when a stored entry was changed afterwards", async () => {
    vi.spyOn(parentApi, "fetchDeclarationProof").mockResolvedValue({
      ...proof,
      integrity_ok: false,
    });

    render(<DeclarationProofPage announcementId="42" />);

    expect(
      await screen.findByText(
        "Achtung: Ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an die OGS.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Text und Antworten sind seit der Veröffentlichung unverändert.",
      ),
    ).not.toBeInTheDocument();
  });

  it("downloads the PDF through the portal and says so when it fails", async () => {
    vi.spyOn(parentApi, "fetchDeclarationProof").mockResolvedValue(proof);
    const download = vi
      .spyOn(parentApi, "downloadDeclarationProofPdf")
      .mockRejectedValueOnce(new ParentApiError("boom", 500))
      .mockResolvedValueOnce(undefined);

    render(<DeclarationProofPage announcementId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Als PDF herunterladen" }),
    );
    await waitFor(() => expect(download).toHaveBeenCalledWith("42", "5"));
    expect(
      await screen.findByText(
        "Das PDF konnte nicht erstellt werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Als PDF herunterladen" }),
    );
    await waitFor(() => expect(download).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByText(/Das PDF konnte nicht erstellt werden/),
      ).not.toBeInTheDocument(),
    );
  });

  it("explains a missing proof instead of an error", async () => {
    vi.spyOn(parentApi, "fetchDeclarationProof").mockRejectedValue(
      new ParentApiError("not found", 404, "not_found"),
    );

    render(<DeclarationProofPage announcementId="42" />);

    expect(
      await screen.findByText(
        "Für dieses Kind gibt es noch keinen Nachweis. Er entsteht, sobald Sie auf ein Einverständnis geantwortet haben.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Als PDF herunterladen" }),
    ).not.toBeInTheDocument();
  });

  it("offers a retry when loading fails", async () => {
    const load = vi
      .spyOn(parentApi, "fetchDeclarationProof")
      .mockRejectedValueOnce(new ParentApiError("boom", 500))
      .mockResolvedValueOnce(proof);

    render(<DeclarationProofPage announcementId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Erneut versuchen" }),
    );
    await waitFor(() => expect(load).toHaveBeenCalledTimes(2));
    expect(
      await screen.findByRole("button", { name: "Als PDF herunterladen" }),
    ).toBeInTheDocument();
  });

  it("asks for nothing when the child is missing from the address", () => {
    searchParams.delete("student");
    const load = vi.spyOn(parentApi, "fetchDeclarationProof");

    render(<DeclarationProofPage announcementId="42" />);

    expect(load).not.toHaveBeenCalled();
    expect(
      screen.getByText(/Für dieses Kind gibt es noch keinen Nachweis/),
    ).toBeInTheDocument();
  });
});
