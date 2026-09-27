import "@testing-library/jest-dom/vitest";
import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  Announcement,
  DeclarationStatus,
} from "~/lib/parent-announcements-api";
import DeclarationReportPage from "./page";

const { swr } = vi.hoisted(() => ({
  swr: {
    announcement: undefined as Announcement | undefined,
    status: undefined as DeclarationStatus | undefined,
    error: null as Error | null,
    statusKey: null as string | null,
  },
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "42" }),
  useSearchParams: () => new URLSearchParams(),
  usePathname: () => "/parent-announcements/42/nachweis",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key === "parent-announcement-42") {
      return {
        data: swr.announcement,
        isLoading: false,
        error: swr.error,
        mutate: vi.fn(),
      };
    }
    swr.statusKey = key;
    return {
      data: key ? swr.status : undefined,
      isLoading: false,
      error: null,
      mutate: vi.fn(),
    };
  },
}));

const announcement: Announcement = {
  id: "42",
  title: "Ausflug in den Zoo",
  body: "Wir fahren am Freitag in den Zoo.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: false,
  status: "published",
  published_at: "2026-09-01T08:00:00Z",
  active: true,
  created_at: "2026-08-30T08:00:00Z",
  updated_at: "2026-09-01T08:00:00Z",
  targets: [{ target_type: "school_all" }],
  response_type: "none",
  options: [],
  delivery_mode: "declaration",
  email_audience: "portal_only",
  declaration_kind: "consent",
  declaration_signers: "any",
  declaration_revocable: true,
  declaration_requires_password: true,
};

const HASH = "c".repeat(64);

const status: DeclarationStatus = {
  kind: "consent",
  signers: "any",
  revocable: true,
  requires_password: true,
  deadline: "2026-10-15T21:59:59Z",
  current_version: null,
  versions: [
    {
      id: "12",
      version_no: 2,
      title: "Ausflug in den Zoo",
      body: "Wir fahren am Freitag in den Zoo.\nAbfahrt 8 Uhr.",
      content_hash: HASH,
      published_at: "2026-09-01T08:00:00Z",
      attachments: [
        {
          filename: "Ausflug.pdf",
          content_type: "application/pdf",
          size_bytes: 2048,
          sha256: "d".repeat(64),
        },
      ],
      integrity_ok: true,
    },
    {
      id: "11",
      version_no: 1,
      title: "Ausflug",
      body: "Alter Text.",
      content_hash: "e".repeat(64),
      published_at: "2026-08-31T08:00:00Z",
      attachments: [],
      integrity_ok: false,
    },
  ],
  summary: {
    children_total: 1,
    agreed: 1,
    declined: 0,
    acknowledged: 0,
    revoked: 0,
    partial: 0,
    open: 0,
    no_signer: 0,
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
      record_hash: "f".repeat(64),
      submitted_at: "2026-09-02T06:15:00Z",
      integrity_ok: true,
    },
  ],
  integrity_ok: false,
};

function printCopy(): HTMLElement {
  const copy = document.querySelector<HTMLElement>(
    "body > .moto-print-document",
  );
  expect(copy).not.toBeNull();
  return copy!;
}

describe("Nachweisbericht einer Erklärung (#3430)", () => {
  beforeEach(() => {
    swr.announcement = announcement;
    swr.status = status;
    swr.error = null;
    swr.statusKey = null;
  });

  it("prints settings, every version in full, the state per child and the history", async () => {
    render(<DeclarationReportPage />);

    expect(
      await screen.findByRole("button", {
        name: "Drucken / als PDF speichern",
      }),
    ).toBeInTheDocument();

    const doc = within(printCopy());
    expect(
      doc.getByText("Nachweisbericht: Ausflug in den Zoo"),
    ).toBeInTheDocument();
    expect(doc.getByText("Zustimmen oder ablehnen")).toBeInTheDocument();
    expect(doc.getByText("Wird abgefragt")).toBeInTheDocument();
    expect(doc.getByText("Bis 15.10.2026")).toBeInTheDocument();
    expect(
      doc.getByText(/Wir fahren am Freitag in den Zoo.\s*Abfahrt 8 Uhr./),
    ).toBeInTheDocument();
    expect(doc.getByText("Alter Text.")).toBeInTheDocument();
    expect(doc.getAllByText(HASH).length).toBeGreaterThan(0);
    expect(doc.getByText("d".repeat(64))).toBeInTheDocument();
    expect(doc.getByText("f".repeat(64))).toBeInTheDocument();
    expect(
      doc.getByText("Klaus Schneider: Zugestimmt am 02.09.2026, 08:15 Uhr"),
    ).toBeInTheDocument();
    expect(doc.getByText("Passwort bestätigt")).toBeInTheDocument();
    expect(doc.getByText("Prüfung in Ordnung")).toBeInTheDocument();
    // Both integrity flags reach the paper.
    expect(
      doc.getByText(/Mindestens ein Eintrag passt nicht mehr/),
    ).toBeInTheDocument();
    expect(
      doc.getByText(/Der gespeicherte Text passt nicht mehr/),
    ).toBeInTheDocument();
    expect(
      doc.getByText(/Er ersetzt keine gesetzlich vorgeschriebene Unterschrift/),
    ).toBeInTheDocument();
  });

  it("does not load a status for an announcement that is no Erklärung", () => {
    swr.announcement = { ...announcement, delivery_mode: "letter" };
    render(<DeclarationReportPage />);

    expect(swr.statusKey).toBeNull();
    expect(
      screen.getByText("Zu dieser Mitteilung gibt es keinen Nachweisbericht."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Drucken / als PDF speichern" }),
    ).not.toBeInTheDocument();
  });
});
