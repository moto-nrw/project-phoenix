import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  acknowledgeStaffNotice,
  createStaffNoticesReadApi,
  deleteStaffNotice,
  describeAudience,
  fetchNoticeAcknowledgements,
} from "./staff-notices-api";

describe("staff-notices-api", () => {
  let fetchMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it("accepts empty successful deletion and acknowledgement responses", async () => {
    fetchMock
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await expect(deleteStaffNotice("42")).resolves.toBeUndefined();
    await expect(acknowledgeStaffNotice("42")).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(1, "/api/staff-notices/42", {
      method: "DELETE",
    });
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/staff-notices/42/acknowledge",
      expect.objectContaining({ method: "POST" }),
    );
  });

  // moto schule liest dieselben Funktionen über seinen eigenen Pfad (#2208).
  it("bindet die Lesefunktionen an den übergebenen Portal-Pfad", async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ data: [{ id: "1" }] }), { status: 200 }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const api = createStaffNoticesReadApi("/api/school/staff-notices");
    await expect(api.fetchTodaysNotices()).resolves.toEqual([{ id: "1" }]);
    await expect(api.acknowledgeStaffNotice("1")).resolves.toBeUndefined();

    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/school/staff-notices/today",
      undefined,
    );
    expect(fetchMock).toHaveBeenNthCalledWith(
      2,
      "/api/school/staff-notices/1/acknowledge",
      expect.objectContaining({ method: "POST" }),
    );
  });

  // Der Schul-Proxy reicht die Backend-Hülle `{ data: [...] }` ungeöffnet in
  // seiner eigenen Hülle weiter; die Liste muss trotzdem ankommen.
  it("packt die doppelte Hülle des Schul-Proxys aus", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { data: [{ id: "9" }] } }), {
        status: 200,
      }),
    );
    const api = createStaffNoticesReadApi("/api/school/staff-notices");
    await expect(api.fetchTodaysNotices()).resolves.toEqual([{ id: "9" }]);
  });

  it("liefert die Bestätigungsliste und meldet Fehler auf Deutsch", async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            data: [
              { account_id: "7", name: "Anna Beispiel", acknowledged_at: "x" },
            ],
          }),
          { status: 200 },
        ),
      )
      .mockResolvedValueOnce(new Response("nope", { status: 403 }));

    await expect(fetchNoticeAcknowledgements("42")).resolves.toEqual([
      { account_id: "7", name: "Anna Beispiel", acknowledged_at: "x" },
    ]);
    expect(fetchMock).toHaveBeenNthCalledWith(
      1,
      "/api/staff-notices/42/acknowledgements",
      undefined,
    );

    await expect(fetchNoticeAcknowledgements("42")).rejects.toThrow(
      "Die Bestätigungen konnten nicht geladen werden",
    );
  });

  it("beschreibt die Zielgruppe in einem Satzteil", () => {
    expect(describeAudience("all")).toBe("Für alle");
    expect(describeAudience("staff")).toBe("Nur für die Betreuung");
    expect(describeAudience("lehrkraft")).toBe("Nur für Lehrkräfte");
  });
});
