import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  StaffLinkPreview,
  StaffOverview,
  StaffRecordDetail,
} from "~/lib/traeger-staff-api";
import { DESKTOP_WIDTH, setViewportWidth } from "~/test/helpers/viewport";

const { api, mockPush } = vi.hoisted(() => ({
  api: {
    fetchStaffOverview: vi.fn(),
    previewStaffLink: vi.fn(),
    linkStaff: vi.fn(),
  },
  mockPush: vi.fn(),
}));

vi.mock("~/lib/traeger-staff-api", async (original) => ({
  ...(await original<typeof import("~/lib/traeger-staff-api")>()),
  ...api,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn() }),
  usePathname: () => "/personal",
  useSearchParams: () => new URLSearchParams(),
}));

import TraegerStaffRecordsPage from "./page";

afterEach(() => setViewportWidth(DESKTOP_WIDTH));

const mira = (schoolId: string, staffId: string, lastName: string) => ({
  schoolId,
  schoolName: schoolId === "1" ? "OGS Nord" : "OGS Süd",
  staffId,
  firstName: "Mira",
  lastName,
  birthday: "",
  personnelNumber: schoolId === "1" ? "70001" : null,
  hasAccount: true,
});

const OVERVIEW: StaffOverview = {
  release: { representation: "agreement", releasedAt: "2026-11-13T09:00:00Z" },
  schools: [
    { id: "1", name: "OGS Nord" },
    { id: "2", name: "OGS Süd" },
  ],
  records: [],
  suggestions: [
    {
      reason: "same_account",
      recordId: null,
      members: [mira("1", "11", "Sommer"), mira("2", "22", "Sommer-Berg")],
    },
    {
      reason: "single",
      recordId: null,
      members: [
        {
          ...mira("2", "33", "Keller"),
          firstName: "Jonas",
          personnelNumber: null,
        },
      ],
    },
  ],
};

const PREVIEW: StaffLinkPreview = {
  record: null,
  members: [mira("1", "11", "Sommer"), mira("2", "22", "Sommer-Berg")],
  fields: [
    {
      field: "first_name",
      conflict: false,
      options: [{ source: "1", schoolName: "OGS Nord", value: "Mira" }],
    },
    {
      field: "last_name",
      conflict: true,
      options: [
        { source: "1", schoolName: "OGS Nord", value: "Sommer" },
        { source: "2", schoolName: "OGS Süd", value: "Sommer-Berg" },
      ],
    },
    {
      field: "iban",
      conflict: false,
      options: [{ source: "2", schoolName: "OGS Süd", value: "•••• 3000" }],
    },
  ],
};

const RECORD = {
  id: "5",
  firstName: "Mira",
  lastName: "Sommer-Berg",
} as StaffRecordDetail;

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerStaffRecordsPage />
    </ToastProvider>,
  );
}

describe("TraegerStaffRecordsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    api.fetchStaffOverview.mockResolvedValue(OVERVIEW);
    api.previewStaffLink.mockResolvedValue(PREVIEW);
    api.linkStaff.mockResolvedValue(RECORD);
  });

  it("explains the release where the Träger is not released yet", async () => {
    api.fetchStaffOverview.mockResolvedValue({
      ...OVERVIEW,
      release: null,
      suggestions: [],
    });
    renderPage();
    expect(
      await screen.findByText(
        "Personal beim Träger ist noch nicht freigeschaltet.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: /Vorschläge/ })).toBeNull();
  });

  it("opens a proposal, asks for the differing value and links the record", async () => {
    renderPage();
    const proposal = await screen.findByText(
      "Gleiches Konto",
      {},
      { timeout: 3000 },
    );
    expect(screen.getByText("Vorschläge zum Verknüpfen")).toBeInTheDocument();
    const row = proposal.closest("tr");
    expect(row).not.toBeNull();
    fireEvent.click(row!);

    const dialog = await screen.findByRole("dialog", {
      name: "Personalakte anlegen",
    });
    expect(api.previewStaffLink).toHaveBeenCalledWith({
      recordId: null,
      members: [
        { schoolId: "1", staffId: "11" },
        { schoolId: "2", staffId: "22" },
      ],
      choices: {},
    });
    expect(
      await within(dialog).findByText("Welcher Wert gilt?"),
    ).toBeInTheDocument();
    expect(within(dialog).getByText("•••• 3000")).toBeInTheDocument();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Akte anlegen" }),
    );
    expect(
      await within(dialog).findByText(
        "Bitte wählen Sie bei jedem Feld, welcher Wert gilt.",
      ),
    ).toBeInTheDocument();
    expect(api.linkStaff).not.toHaveBeenCalled();

    fireEvent.click(within(dialog).getByRole("radio", { name: /Sommer-Berg/ }));
    fireEvent.click(
      within(dialog).getByRole("button", { name: "Akte anlegen" }),
    );
    await vi.waitFor(() =>
      expect(api.linkStaff).toHaveBeenCalledWith({
        recordId: null,
        members: [
          { schoolId: "1", staffId: "11" },
          { schoolId: "2", staffId: "22" },
        ],
        choices: { last_name: "2" },
      }),
    );
    await vi.waitFor(() =>
      expect(mockPush).toHaveBeenCalledWith("/personal/5"),
    );
  });

  it("lists the people without a record", async () => {
    renderPage();
    fireEvent.click(await screen.findByRole("tab", { name: /Ohne Akte/ }));
    expect(await screen.findByText("Jonas Keller")).toBeInTheDocument();
  });
});
