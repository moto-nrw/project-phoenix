import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { SchoolSearchResult } from "~/lib/school-search";

const {
  mockUseSession,
  mockPush,
  mockSearchSchool,
  mockShowLoadError,
  mockClearLoadError,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockPush: vi.fn(),
  mockSearchSchool: vi.fn(),
  mockShowLoadError: vi.fn(),
  mockClearLoadError: vi.fn(),
}));

vi.mock("next-auth/react", () => ({ useSession: () => mockUseSession() }));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, replace: vi.fn(), prefetch: vi.fn() }),
  usePathname: () => "/home",
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush, replace: vi.fn() }),
}));
vi.mock("~/lib/hooks/use-module-navigation", () => ({
  useModuleNavigation: () => ({ page: () => "open", section: () => "open" }),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useApiLoadError: () => ({
    error: null,
    show: mockShowLoadError,
    clear: mockClearLoadError,
  }),
}));
vi.mock("~/lib/school-search", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/school-search")>()),
  searchSchool: (...args: unknown[]) => mockSearchSchool(...args),
}));

import { SchoolSearchDialog } from "./school-search";

const EMPTY: SchoolSearchResult = {
  children: [],
  contacts: [],
  staff: [],
  rooms: [],
  groups: [],
};

function signIn(permissions: string[]) {
  mockUseSession.mockReturnValue({
    data: { user: { token: "t", roles: [], permissions } },
    status: "authenticated",
  });
}

async function typeQuery(text: string) {
  const user = userEvent.setup();
  await user.type(screen.getByRole("combobox"), text);
  // The search waits for a pause in typing before it asks the server.
  await act(async () => {
    await new Promise((resolve) => setTimeout(resolve, 260));
  });
  return user;
}

describe("SchoolSearchDialog", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchSchool.mockResolvedValue(EMPTY);
  });

  it("offers the pages the account may open before anything is typed", () => {
    signIn(["users:read"]);
    render(<SchoolSearchDialog isOpen onClose={vi.fn()} />);
    const pages = screen.getByRole("group", { name: "Seiten" });
    expect(
      within(pages).getByRole("option", { name: "Alle Kinder" }),
    ).toBeInTheDocument();
    expect(
      within(pages).queryByRole("option", { name: "Einstellungen" }),
    ).not.toBeInTheDocument();
    expect(mockSearchSchool).not.toHaveBeenCalled();
  });

  it("finds the family behind a phone number and opens the child's contacts", async () => {
    signIn(["users:read"]);
    const onClose = vi.fn();
    mockSearchSchool.mockResolvedValue({
      ...EMPTY,
      contacts: [
        {
          id: "21",
          firstName: "Sabine",
          lastName: "Sonnenberg",
          match: { field: "phone", text: "+49 170 1234567" },
          children: [
            {
              id: "11",
              firstName: "Mia",
              lastName: "Sonnenberg",
              schoolClass: "3a",
              relationshipType: "parent",
            },
          ],
        },
      ],
    });
    render(<SchoolSearchDialog isOpen onClose={onClose} />);

    const user = await typeQuery("  0170 1234567 ");
    await screen.findByRole("option", {
      name: "Sabine Sonnenberg, +49 170 1234567, Elternteil von Mia Sonnenberg (Klasse 3a)",
    });
    expect(mockSearchSchool).toHaveBeenLastCalledWith(
      "0170 1234567",
      expect.any(AbortSignal),
    );
    // The typed text never lands in a URL (ADR 0230).
    expect(document.querySelector('a[href*="0170"]')).toBeNull();

    await user.keyboard("{Enter}");
    expect(mockPush).toHaveBeenCalledWith(
      "/students/11?tab=erziehungsberechtigte",
    );
    expect(onClose).toHaveBeenCalled();
  });

  it("says why a phone number finds nothing without contact access", async () => {
    signIn(["students:read"]);
    render(<SchoolSearchDialog isOpen onClose={vi.fn()} />);
    await typeQuery("0170 1234567");
    expect(
      await screen.findByText(
        "Telefonnummern finden nur Konten, die Kontaktdaten sehen dürfen.",
      ),
    ).toBeInTheDocument();
  });

  it("names the registration question and answer of a child", async () => {
    signIn(["users:read"]);
    mockSearchSchool.mockResolvedValue({
      ...EMPTY,
      children: [
        {
          id: "11",
          firstName: "Mia",
          lastName: "Sonnenberg",
          schoolClass: "3a",
          groupName: "Füchse",
          match: {
            field: "enrollment_answer",
            label: "Seepferdchen vorhanden?",
            text: "Ja",
          },
        },
      ],
    });
    render(<SchoolSearchDialog isOpen onClose={vi.fn()} />);
    await typeQuery("seepferd");
    expect(
      await screen.findByRole("option", {
        name: "Mia Sonnenberg, Klasse 3a · Gruppe Füchse · Anmeldung: Seepferdchen vorhanden: Ja",
      }),
    ).toBeInTheDocument();
    expect(document.querySelector('a[href*="seepferd"]')).toBeNull();
  });
});
