import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { listMyChildren } from "~/lib/parent-api";
import { ChildPage } from "./child-page";

const pushMock = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: pushMock, replace: vi.fn(), prefetch: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  getChildToday: vi.fn().mockResolvedValue({ at_ogs: null, state: "unknown" }),
  UNKNOWN_CHILD_TODAY: { at_ogs: null, state: "unknown" },
}));

// Die schweren Abschnitte laden eigene Daten; hier zaehlt nur, DASS sie in der
// richtigen Reihenfolge stehen.
vi.mock("~/components/parent/child/booked-care-section", () => ({
  BookedCareSection: () => <div data-testid="section-care">Betreuung</div>,
}));
vi.mock("~/components/parent/child-master-data", () => ({
  ChildMasterDataView: ({ childName }: { childName: string }) => (
    <div data-testid="section-data">Daten von {childName}</div>
  ),
}));
vi.mock("~/components/parent/guardians-panel", () => ({
  default: () => <div data-testid="section-people">Eltern</div>,
}));
vi.mock("~/components/parent/child-care", () => ({
  useChildCare: () => ({
    loading: false,
    features: {
      sick_note_enabled: true,
      pickup_change_enabled: true,
      notes_enabled: true,
      related_accounts_invite_enabled: false,
      related_accounts_remove_enabled: false,
      excused_requires_approval: false,
    },
    careExceptions: [],
    careExceptionsLoaded: true,
    todayPickup: { kind: "time", time: "15:00", changed: false },
    sickDays: [],
    excusedRequests: [],
    reportSick: vi.fn(),
    withdrawExcused: vi.fn(),
    saveCareException: vi.fn(),
    removeCareException: vi.fn(),
  }),
  SickNoteModal: () => <div data-testid="sick-modal" />,
  PickupTimeModal: () => <div data-testid="pickup-modal" />,
}));

const mockedChildren = vi.mocked(listMyChildren);

const felix = {
  student_id: "42",
  tenant_id: "7",
  first_name: "Felix",
  last_name: "Schneider",
  school_class: "1a",
  status: "active",
  school_name: "Schule am Berg",
  school_slug: "berg",
} as unknown as Awaited<ReturnType<typeof listMyChildren>>[number];

const mia = { ...felix, student_id: "43", first_name: "Mia" };

function renderPage(studentId?: string) {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ChildPage studentId={studentId} />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedChildren.mockResolvedValue([felix]);
});

describe("ChildPage", () => {
  it("zeigt bei einem Kind weder Liste noch Umschalter", async () => {
    renderPage();
    expect(await screen.findByText("Felix Schneider")).toBeInTheDocument();
    expect(screen.queryByRole("tablist")).not.toBeInTheDocument();
  });

  it("zeigt bei mehreren Kindern einen Umschalter und wechselt den Inhalt", async () => {
    mockedChildren.mockResolvedValue([felix, mia]);
    renderPage("42");
    expect(await screen.findByRole("tablist")).toBeInTheDocument();
    expect(screen.getByTestId("section-data")).toHaveTextContent(
      "Daten von Felix Schneider",
    );

    fireEvent.click(screen.getByRole("tab", { name: "Mia Schneider" }));
    expect(pushMock).toHaveBeenCalledWith("/parents/children/43");
  });

  it("stellt die vier Abschnitte in der festgelegten Reihenfolge", async () => {
    renderPage();
    await screen.findByText("Felix Schneider");
    const headings = screen
      .getAllByRole("heading", { level: 2 })
      .map((node) => node.textContent);
    expect(headings[0]).toBe("Heute");

    const sections = [
      screen.getByText("Heute"),
      screen.getByTestId("section-care"),
      screen.getByTestId("section-data"),
      screen.getByTestId("section-people"),
    ];
    for (let i = 1; i < sections.length; i++) {
      expect(
        sections[i - 1]!.compareDocumentPosition(sections[i]!) &
          Node.DOCUMENT_POSITION_FOLLOWING,
      ).toBeTruthy();
    }
  });

  it("nennt die geplante Abholung im Klartext", async () => {
    renderPage();
    expect(
      await screen.findByText("Abholung heute um 15:00 Uhr"),
    ).toBeInTheDocument();
  });

  it("kennt weder Betreuungszeiten noch AGs", async () => {
    renderPage();
    await screen.findByText("Felix Schneider");
    expect(screen.queryByText(/Betreuungszeiten/)).not.toBeInTheDocument();
    expect(screen.queryByText(/AGs/)).not.toBeInTheDocument();
  });

  it("bietet keine Eingabefelder im Kinderbereich ausserhalb der Datenabschnitte", async () => {
    renderPage();
    await screen.findByText("Felix Schneider");
    await waitFor(() =>
      expect(document.querySelectorAll("input")).toHaveLength(0),
    );
  });
});
