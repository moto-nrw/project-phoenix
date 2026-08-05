import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  exportStaffBirthdays,
  fetchBirthdayOptOut,
  fetchBirthdayOverviewClient,
  mapBirthdayOverview,
  updateBirthdayOptOut,
} from "./birthdays-api";
import { fetchWithAuth } from "./fetch-with-auth";

vi.mock("./fetch-with-auth", () => ({
  fetchWithAuth: vi.fn(),
}));

vi.mock("./analytics", () => ({
  trackEvent: vi.fn(),
}));

const originalFetch = globalThis.fetch;
const originalCreateObjectURL = URL.createObjectURL;
const originalRevokeObjectURL = URL.revokeObjectURL;

beforeEach(() => {
  vi.clearAllMocks();
  URL.createObjectURL = vi.fn(() => "blob:birthday-url");
  URL.revokeObjectURL = vi.fn();
});

afterEach(() => {
  globalThis.fetch = originalFetch;
  URL.createObjectURL = originalCreateObjectURL;
  URL.revokeObjectURL = originalRevokeObjectURL;
  vi.restoreAllMocks();
});

describe("mapBirthdayOverview", () => {
  it("maps the backend payload into the camelCase shape the card renders", () => {
    const overview = mapBirthdayOverview({
      enabled: true,
      include_staff: true,
      today: "2026-08-03",
      celebrations: [
        {
          kind: "student",
          id: 12,
          name: "Lina Adler",
          group_name: "Delfine",
          school_class: "1a",
          date: "2026-08-03",
          age: 8,
          is_today: true,
        },
        {
          kind: "staff",
          id: 7,
          name: "Anna Berg",
          date: "2026-08-01",
          is_today: false,
        },
      ],
    });

    expect(overview.enabled).toBe(true);
    expect(overview.includeStaff).toBe(true);
    expect(overview.today).toBe("2026-08-03");
    expect(overview.celebrations).toEqual([
      {
        kind: "student",
        id: "12",
        name: "Lina Adler",
        groupName: "Delfine",
        schoolClass: "1a",
        date: "2026-08-03",
        age: 8,
        isToday: true,
      },
      {
        kind: "staff",
        id: "7",
        name: "Anna Berg",
        groupName: undefined,
        schoolClass: undefined,
        date: "2026-08-01",
        age: undefined,
        isToday: false,
      },
    ]);
  });

  // Go marshals an empty slice as null; the card must render an empty list
  // rather than crash on it.
  it("treats a null celebration list as empty", () => {
    const overview = mapBirthdayOverview({
      enabled: true,
      include_staff: false,
      today: "2026-08-05",
      celebrations: null,
    });

    expect(overview.celebrations).toEqual([]);
  });

  it("keeps a disabled display disabled", () => {
    const overview = mapBirthdayOverview({
      enabled: false,
      include_staff: false,
      today: "2026-08-05",
      celebrations: null,
    });

    expect(overview.enabled).toBe(false);
  });
});

describe("fetchBirthdayOverviewClient", () => {
  it("calls the BFF route and returns the mapped overview", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          data: {
            enabled: true,
            include_staff: false,
            today: "2026-08-05",
            celebrations: [
              {
                kind: "student",
                id: 3,
                name: "Mika Klein",
                date: "2026-08-05",
                age: 9,
                is_today: true,
              },
            ],
          },
        }),
    } as Response);

    const overview = await fetchBirthdayOverviewClient();

    expect(fetchWithAuth).toHaveBeenCalledWith("/api/birthdays");
    expect(overview.celebrations[0]).toMatchObject({
      id: "3",
      name: "Mika Klein",
      isToday: true,
    });
  });

  it("throws with the status so the dashboard can drop the card", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response);

    await expect(fetchBirthdayOverviewClient()).rejects.toThrow(
      "Birthday fetch failed: 500",
    );
  });
});

describe("fetchBirthdayOptOut", () => {
  it("returns the stored preference", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { opt_out: true } }),
    } as Response);

    await expect(fetchBirthdayOptOut()).resolves.toBe(true);
    expect(fetchWithAuth).toHaveBeenCalledWith("/api/birthdays/opt-out");
  });

  // An account without a staff record answers 404; the profile card uses the
  // rejection to hide itself rather than showing a setting nobody can apply.
  it("throws when the account has no preference to read", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: false,
      status: 404,
    } as Response);

    await expect(fetchBirthdayOptOut()).rejects.toThrow(
      "Birthday opt-out fetch failed: 404",
    );
  });
});

describe("updateBirthdayOptOut", () => {
  it("sends the preference and returns what the backend stored", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ data: { opt_out: true } }),
    } as Response);

    await expect(updateBirthdayOptOut(true)).resolves.toBe(true);
    expect(fetchWithAuth).toHaveBeenCalledWith("/api/birthdays/opt-out", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ opt_out: true }),
    });
  });

  it("throws on a failed write so the switch can roll back", async () => {
    vi.mocked(fetchWithAuth).mockResolvedValue({
      ok: false,
      status: 500,
    } as Response);

    await expect(updateBirthdayOptOut(false)).rejects.toThrow(
      "Birthday opt-out update failed: 500",
    );
  });
});

describe("exportStaffBirthdays", () => {
  function stubLink() {
    const link = {
      click: vi.fn(),
      remove: vi.fn(),
      download: "",
      href: "",
    } as unknown as HTMLAnchorElement;
    vi.spyOn(document, "createElement").mockReturnValue(link);
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    return link;
  }

  it("posts the request and downloads the returned document", async () => {
    const link = stubLink();
    globalThis.fetch = vi.fn(
      async () =>
        new Response(new Blob(["pdf"]), {
          status: 200,
          headers: {
            "content-disposition":
              'attachment; filename="geburtstagsliste-personal.pdf"',
          },
        }),
    );

    await exportStaffBirthdays({ format: "pdf", months: ["08"] });

    expect(globalThis.fetch).toHaveBeenCalledWith(
      "/api/birthdays/staff-export",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ format: "pdf", months: ["08"] }),
      },
    );
    expect(link.download).toBe("geburtstagsliste-personal.pdf");
    expect(link.click).toHaveBeenCalled();
    expect(link.remove).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:birthday-url");
  });

  it("falls back to a local filename when the response carries no disposition", async () => {
    const link = stubLink();
    globalThis.fetch = vi.fn(
      async () => new Response(new Blob(["sheet"]), { status: 200 }),
    );

    await exportStaffBirthdays({ format: "xlsx" });

    expect(link.download).toBe("geburtstagsliste-personal.xlsx");
  });

  it("falls back to a local filename when the disposition names no file", async () => {
    const link = stubLink();
    globalThis.fetch = vi.fn(
      async () =>
        new Response(new Blob(["doc"]), {
          status: 200,
          headers: { "content-disposition": "attachment" },
        }),
    );

    await exportStaffBirthdays({ format: "docx" });

    expect(link.download).toBe("geburtstagsliste-personal.docx");
  });

  // The proxy forwards the backend's human-readable message; the toast must
  // show that rather than the JSON envelope.
  it("surfaces the backend error message", async () => {
    globalThis.fetch = vi.fn(
      async () =>
        new Response(JSON.stringify({ error: "Ungültiger Geburtsmonat" }), {
          status: 400,
        }),
    );

    await expect(exportStaffBirthdays({ format: "pdf" })).rejects.toThrow(
      "Ungültiger Geburtsmonat",
    );
  });

  it("forwards a plain-text error body unchanged", async () => {
    globalThis.fetch = vi.fn(
      async () => new Response("Unauthorized", { status: 401 }),
    );

    await expect(exportStaffBirthdays({ format: "pdf" })).rejects.toThrow(
      "Unauthorized",
    );
  });

  it("falls back to a generic message on an empty error body", async () => {
    globalThis.fetch = vi.fn(async () => new Response("", { status: 500 }));

    await expect(exportStaffBirthdays({ format: "pdf" })).rejects.toThrow(
      "Export fehlgeschlagen",
    );
  });

  it("keeps the generic message when the error payload has no error field", async () => {
    globalThis.fetch = vi.fn(
      async () =>
        new Response(JSON.stringify({ detail: "nope" }), { status: 500 }),
    );

    await expect(exportStaffBirthdays({ format: "pdf" })).rejects.toThrow(
      '{"detail":"nope"}',
    );
  });
});
