import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DeclarationStatus } from "~/lib/parent-announcements-api";
import {
  DeclarationStatusPanel,
  sortDeclarationChildren,
} from "./declaration-status-panel";

const { statusMock, remindMock, downloadMock } = vi.hoisted(() => ({
  statusMock: vi.fn(),
  remindMock: vi.fn(),
  downloadMock: vi.fn(),
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/parent-announcements-api")>();
  return {
    ...actual,
    fetchDeclarationStatus: statusMock,
    remindUnanswered: remindMock,
    downloadDeclarationExport: downloadMock,
  };
});

const HASH = "a1b2c3d4e5f6".padEnd(64, "0");

function status(overrides: Partial<DeclarationStatus> = {}): DeclarationStatus {
  const version = {
    id: "12",
    version_no: 2,
    title: "Ausflug in den Zoo",
    body: "Wir fahren am Freitag in den Zoo.",
    content_hash: HASH,
    published_at: "2026-09-01T08:00:00Z",
    integrity_ok: true,
    attachments: [
      {
        filename: "Ausflug.pdf",
        content_type: "application/pdf",
        size_bytes: 12_345,
        sha256: "f".repeat(64),
      },
    ],
  };
  return {
    kind: "consent",
    signers: "all",
    revocable: true,
    requires_password: false,
    deadline: null,
    current_version: version,
    versions: [version, { ...version, id: "11", version_no: 1 }],
    summary: {
      children_total: 4,
      agreed: 1,
      declined: 0,
      revoked: 0,
      partial: 1,
      open: 1,
      no_signer: 1,
      expired: 0,
    },
    children: [
      {
        student_id: "5",
        first_name: "Mia",
        last_name: "Muster",
        school_class: "2a",
        state: "agreed",
        signers: [
          {
            account_id: "24",
            first_name: "Klaus",
            last_name: "Schneider",
            action: "agreed",
            submitted_at: "2026-09-02T06:15:00Z",
          },
        ],
      },
      {
        student_id: "6",
        first_name: "Ben",
        last_name: "Yilmaz",
        school_class: "2a",
        state: "partial",
        signers: [],
      },
      {
        student_id: "7",
        first_name: "Lea",
        last_name: "Kaya",
        school_class: "3b",
        state: "open",
        signers: [],
      },
      {
        student_id: "8",
        first_name: "Tim",
        last_name: "Braun",
        school_class: "3b",
        state: "no_signer",
        signers: [],
      },
    ],
    submissions: [
      {
        id: "77",
        student_id: "5",
        student_first_name: "Mia",
        student_last_name: "Muster",
        signer_name: "Klaus Schneider",
        guardian_role: "primary_guardian",
        action: "agreed",
        method: "simple_electronic",
        password_confirmed: true,
        version_no: 2,
        content_hash: HASH,
        record_hash: "b".repeat(64),
        submitted_at: "2026-09-02T06:15:00Z",
        integrity_ok: false,
      },
    ],
    integrity_ok: false,
    ...overrides,
  };
}

describe("DeclarationStatusPanel (#3430)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    statusMock.mockResolvedValue(status());
    remindMock.mockResolvedValue(3);
    downloadMock.mockResolvedValue(undefined);
  });

  it("counts per child and names who acted", async () => {
    render(<DeclarationStatusPanel announcementId="42" canAct />);

    expect(await screen.findByText("Kinder mit Antwort")).toBeInTheDocument();
    // 1 of 3 children with somebody who can act; Tim has nobody.
    expect(screen.getByText("/ 3")).toBeInTheDocument();
    expect(
      screen.getByText("Für 2 Kinder fehlt noch eine Antwort."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Für 1 Kind kann niemand antworten/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Klaus Schneider: Zugestimmt am 02.09.2026, 08:15 Uhr"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Teilweise beantwortet").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText("Niemand kann antworten")).toBeInTheDocument();
    expect(screen.getAllByText("Offen").length).toBeGreaterThan(0);
  });

  it("filters to the children a reminder reaches", async () => {
    render(<DeclarationStatusPanel announcementId="42" canAct />);
    await screen.findByText("Kinder mit Antwort");

    fireEvent.click(screen.getByRole("button", { name: "Nur offene" }));
    expect(screen.queryByText("Mia Muster")).not.toBeInTheDocument();
    expect(screen.queryByText("Tim Braun")).not.toBeInTheDocument();
    expect(screen.getByText("Ben Yilmaz")).toBeInTheDocument();
    expect(screen.getByText("Lea Kaya")).toBeInTheDocument();
  });

  it("shows the version, older versions and no checksum at all", async () => {
    render(<DeclarationStatusPanel announcementId="42" canAct />);
    await screen.findByText("Kinder mit Antwort");

    expect(
      screen.getByText(/Fassung 2, veröffentlicht am/),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Ausflug.pdf", { exact: false }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Frühere Fassung: 1/)).toBeInTheDocument();

    const text = document.body.textContent ?? "";
    for (const hash of [HASH, "f".repeat(64), "b".repeat(64)]) {
      expect(text).not.toContain(hash);
    }
    expect(text).not.toContain("a1b2c3d4e5f6");
    expect(text).not.toMatch(/SHA-256|Prüfsumme/);
  });

  it("says in one sentence that everything is unchanged", async () => {
    statusMock.mockResolvedValue(
      status({
        integrity_ok: true,
        submissions: status().submissions.map((entry) => ({
          ...entry,
          integrity_ok: true,
        })),
      }),
    );
    render(<DeclarationStatusPanel announcementId="42" canAct />);
    await screen.findByText("Kinder mit Antwort");

    expect(
      screen.getByText(
        "Text und Antworten sind seit der Veröffentlichung unverändert.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Nachträglich verändert"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/Achtung/)).not.toBeInTheDocument();
  });

  it("warns and marks the changed entry when the check fails", async () => {
    render(<DeclarationStatusPanel announcementId="42" canAct />);
    await screen.findByText("Kinder mit Antwort");

    expect(
      screen.getByText(
        "Achtung: Mindestens ein gespeicherter Eintrag wurde nachträglich verändert. Bitte wenden Sie sich an den moto-Support.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(
        "Text und Antworten sind seit der Veröffentlichung unverändert.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Klaus Schneider")).toBeInTheDocument();
    expect(
      screen.getByText(/Fassung 2 · Hauptberechtigt · mit Passwort/),
    ).toBeInTheDocument();
    expect(screen.getByText("Nachträglich verändert")).toBeInTheDocument();
  });

  it("reminds the open ones and downloads the report and the history", async () => {
    render(<DeclarationStatusPanel announcementId="42" canAct />);
    await screen.findByText("Kinder mit Antwort");

    fireEvent.click(screen.getByRole("button", { name: /Offene erinnern/ }));
    expect(
      await screen.findByText("3 Personen wurden erinnert."),
    ).toBeInTheDocument();
    expect(remindMock).toHaveBeenCalledWith("42");

    expect(
      screen.queryByRole("link", { name: /Bericht drucken/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Bericht als PDF/ }));
    await waitFor(() => expect(downloadMock).toHaveBeenCalledWith("42", "pdf"));
    fireEvent.click(screen.getByRole("button", { name: /Verlauf als CSV/ }));
    await waitFor(() => expect(downloadMock).toHaveBeenCalledWith("42", "csv"));
  });

  it("offers no reminder for a withdrawn Einverständnis and says when an export fails", async () => {
    downloadMock.mockRejectedValue(new Error("boom"));
    render(<DeclarationStatusPanel announcementId="42" canAct={false} />);
    await screen.findByText("Kinder mit Antwort");

    expect(
      screen.queryByRole("button", { name: /Offene erinnern/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Verlauf als CSV/ }));
    expect(
      await screen.findByText(
        "Die Datei konnte nicht erstellt werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
  });
});

describe("child order (#3430)", () => {
  function kid(
    first_name: string,
    last_name: string,
    state: DeclarationStatus["children"][number]["state"],
  ): DeclarationStatus["children"][number] {
    return {
      student_id: `${first_name}-${last_name}`,
      first_name,
      last_name,
      school_class: "1a",
      state,
      signers: [],
    };
  }

  it("puts answers first, then waiting children, nobody-can-submit last, each by name", () => {
    const sorted = sortDeclarationChildren([
      kid("Anton", "Adler", "no_signer"),
      kid("Berta", "Braun", "open"),
      kid("Carl", "Zeller", "agreed"),
      kid("Dora", "Albrecht", "expired"),
      kid("Emil", "Becker", "partial"),
      kid("Frida", "Arndt", "declined"),
      kid("Gustav", "Abel", "no_signer"),
    ]).map((child) => child.last_name);

    expect(sorted).toEqual([
      "Arndt",
      "Becker",
      "Zeller",
      "Albrecht",
      "Braun",
      "Abel",
      "Adler",
    ]);
  });

  it("renders the panel list in that order", async () => {
    // Reversed input, so the order below comes from the sort.
    statusMock.mockResolvedValue(
      status({ children: [...status().children].reverse() }),
    );
    const { container } = render(
      <DeclarationStatusPanel announcementId="42" canAct />,
    );
    await screen.findByText("Kinder mit Antwort");

    const names = Array.from(
      container.querySelectorAll("section ul > li > div > p:first-child"),
    ).map((node) => node.textContent ?? "");
    const order = ["Mia Muster", "Ben Yilmaz", "Lea Kaya", "Tim Braun"].map(
      (name) => names.findIndex((text) => text.startsWith(name)),
    );
    expect(order.every((index) => index >= 0)).toBe(true);
    expect(order).toEqual([...order].sort((a, b) => a - b));
  });
});
