import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./session-cache", () => ({
  sessionFetch: vi.fn(),
}));

import { sessionFetch } from "./session-cache";
import { staffPayrollNumberService } from "./staff-api";

const mockedSessionFetch = vi.mocked(sessionFetch);

function jsonResponse(data: unknown): Response {
  return {
    ok: true,
    json: () => Promise.resolve({ data }),
  } as Response;
}

function errorResponse(code: string, message = "backend error"): Response {
  return {
    ok: false,
    text: () => Promise.resolve(JSON.stringify({ code, error: message })),
  } as Response;
}

describe("staffPayrollNumberService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches the current personnel number", async () => {
    mockedSessionFetch.mockResolvedValue(
      jsonResponse({ personnel_number: "90001" }),
    );

    await expect(staffPayrollNumberService.get("42")).resolves.toBe("90001");
    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/staff/42/payroll-number",
    );
  });

  it("updates the personnel number and note", async () => {
    mockedSessionFetch.mockResolvedValue(
      jsonResponse({ personnel_number: "90002" }),
    );

    await expect(
      staffPayrollNumberService.update("42", "90002", "Korrektur"),
    ).resolves.toBe("90002");
    expect(mockedSessionFetch).toHaveBeenCalledWith(
      "/api/staff/42/payroll-number",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          personnel_number: "90002",
          note: "Korrektur",
        }),
      },
    );
  });

  it("reports a failed read", async () => {
    mockedSessionFetch.mockResolvedValue({
      ok: false,
      statusText: "Forbidden",
    } as Response);

    await expect(staffPayrollNumberService.get("42")).rejects.toThrow(
      "Failed to fetch payroll number: Forbidden",
    );
  });

  it.each([
    [
      "personnel_number_taken",
      "Diese Personalnummer ist in dieser Schule bereits vergeben.",
    ],
    [
      "personnel_number_invalid",
      "Ungültige Personalnummer: nur Ziffern, höchstens 9 Stellen.",
    ],
    ["other", "backend error"],
  ])("maps update error %s", async (code, expectedMessage) => {
    mockedSessionFetch.mockResolvedValue(errorResponse(code));

    await expect(
      staffPayrollNumberService.update("42", "90002", ""),
    ).rejects.toThrow(expectedMessage);
  });
});
