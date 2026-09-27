import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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

describe("Nachweis einer Erklärung im Eltern-Portal (#3430)", () => {
  it("shows the full text, checksums, own history and the legal note, ready to print", async () => {
    const load = vi
      .spyOn(parentApi, "fetchDeclarationProof")
      .mockResolvedValue(proof);

    render(<DeclarationProofPage announcementId="42" />);

    expect(
      await screen.findByRole("button", {
        name: "Drucken / als PDF speichern",
      }),
    ).toBeInTheDocument();
    expect(load).toHaveBeenCalledWith("42", "5");
    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Nachweis Ihrer Erklärung",
      }),
    ).toBeInTheDocument();

    const copy = await waitFor(() => {
      const found = document.querySelector<HTMLElement>(
        "body > .moto-print-document",
      );
      expect(found).not.toBeNull();
      return found;
    });
    const doc = within(copy!);
    expect(doc.getByText("Nachweis Ihrer Erklärung")).toBeInTheDocument();
    expect(doc.getByText("Mia Muster")).toBeInTheDocument();
    expect(
      doc.getByText("Wir fahren am Freitag in den Zoo."),
    ).toBeInTheDocument();
    expect(
      doc.getByText("Zugestimmt am 02.09.2026, 08:15 Uhr"),
    ).toBeInTheDocument();
    expect(
      doc.getByText("Antwort von Klaus Schneider (Hauptberechtigt)"),
    ).toBeInTheDocument();
    expect(doc.getByText(/Mit Passwort bestätigt/)).toBeInTheDocument();
    expect(doc.getByText("c".repeat(64))).toBeInTheDocument();
    expect(doc.getByText(`SHA-256: ${"b".repeat(64)}`)).toBeInTheDocument();
    expect(
      doc.getByText(
        "Dieser Nachweis belegt eine einfache elektronische Erklärung. Er ersetzt keine gesetzlich vorgeschriebene Unterschrift auf Papier.",
      ),
    ).toBeInTheDocument();
  });

  it("explains a missing proof instead of an error", async () => {
    vi.spyOn(parentApi, "fetchDeclarationProof").mockRejectedValue(
      new ParentApiError("not found", 404, "not_found"),
    );

    render(<DeclarationProofPage announcementId="42" />);

    expect(
      await screen.findByText(
        "Für dieses Kind gibt es noch keinen Nachweis. Er entsteht, sobald Sie auf eine Erklärung geantwortet haben.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Drucken / als PDF speichern" }),
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
      await screen.findByRole("button", {
        name: "Drucken / als PDF speichern",
      }),
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
