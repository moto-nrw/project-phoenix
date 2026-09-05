import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const { mockGetSession } = vi.hoisted(() => ({
  mockGetSession: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  getSession: mockGetSession,
}));

import { StudentHistorieTab } from "./student-historie-tab";

function mockFetchResponse(
  body: unknown,
  options: { ok?: boolean; status?: number; text?: string } = {},
): Response {
  return {
    ok: options.ok ?? true,
    status: options.status ?? 200,
    json: () => Promise.resolve(body),
    text: () => Promise.resolve(options.text ?? ""),
  } as unknown as Response;
}

describe("StudentHistorieTab", () => {
  const fetchSpy = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    globalThis.fetch = fetchSpy as unknown as typeof fetch;
    mockGetSession.mockResolvedValue({ user: { token: "t" } });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("shows loading indicator initially", () => {
    fetchSpy.mockImplementation(() => new Promise(() => undefined));
    render(<StudentHistorieTab studentId="1" />);

    expect(
      screen.getByText("Lade Anwesenheits-Historie..."),
    ).toBeInTheDocument();
  });

  it("shows disabled card when response is 403 feature_disabled", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse(null, {
        ok: false,
        status: 403,
        text: '{"error":"feature_disabled"}',
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Historie ist deaktiviert")).toBeInTheDocument(),
    );
  });

  it("shows forbidden card on other 403 reasons", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse(null, { ok: false, status: 403, text: "nope" }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "Du hast keine Berechtigung, diese Historie zu sehen.",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("shows error banner on other non-ok responses", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse(null, {
        ok: false,
        status: 500,
        text: "internal boom",
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText(/Request failed \(500\)/)).toBeInTheDocument(),
    );
  });

  it("renders empty state when no days have attendance", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse({
        data: {
          student_id: "1",
          days: [
            {
              date: "2024-01-15",
              attendance: null,
              room_detail_available: false,
              visits: [],
            },
          ],
          range: { start: "2024-01-01", end: "2024-01-15" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(
        screen.getByText("Keine Einträge im sichtbaren Zeitraum"),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText(/Letzte 30 Tage/)).toBeInTheDocument();
  });

  it("renders days with attendance and visits", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse({
        data: {
          student_id: "1",
          days: [
            {
              date: "2024-01-15",
              attendance: {
                check_in_time: "2024-01-15T08:00:00Z",
                check_out_time: "2024-01-15T15:30:00Z",
                duration_minutes: 450,
              },
              room_detail_available: true,
              visits: [
                {
                  room_id: 1,
                  room_name: "Klassenraum 3a",
                  entry_time: "2024-01-15T08:00:00Z",
                  exit_time: "2024-01-15T10:00:00Z",
                  duration_minutes: 120,
                },
                {
                  room_name: "",
                  entry_time: "2024-01-15T10:00:00Z",
                  exit_time: null,
                },
              ],
            },
          ],
          range: { start: "2024-01-15", end: "2024-01-15" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Klassenraum 3a")).toBeInTheDocument(),
    );
    expect(screen.getByText("Raum unbekannt")).toBeInTheDocument();
    expect(screen.getByText(/Anwesenheits-Historie/i)).toBeInTheDocument();
  });

  it("shows 'offen' when no check_out_time and duration 0", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse({
        data: {
          student_id: "1",
          days: [
            {
              date: "2024-01-15",
              attendance: {
                check_in_time: "2024-01-15T08:00:00Z",
                check_out_time: null,
                duration_minutes: 0,
              },
              room_detail_available: false,
              visits: [],
            },
          ],
          range: { start: "2024-01-15", end: "2024-01-15" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      }),
    );

    const { container } = render(<StudentHistorieTab studentId="1" />);

    await waitFor(() => expect(container.textContent).toContain("offen"));
  });

  it("handles unwrapped (no .data) payloads", async () => {
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse({
        student_id: "1",
        days: [],
        range: { start: "2024-01-01", end: "2024-01-15" },
        clamped: false,
        caps: { attendance_days: 30, room_detail_days: 7 },
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() =>
      expect(
        screen.getByText("Keine Einträge im sichtbaren Zeitraum"),
      ).toBeInTheDocument(),
    );
  });

  it("omits Authorization header when no session token", async () => {
    mockGetSession.mockResolvedValueOnce(null);
    fetchSpy.mockResolvedValueOnce(
      mockFetchResponse({
        data: {
          student_id: "1",
          days: [],
          range: { start: "2024-01-01", end: "2024-01-15" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      }),
    );

    render(<StudentHistorieTab studentId="1" />);

    await waitFor(() => expect(fetchSpy).toHaveBeenCalled());
    const headers = (fetchSpy.mock.calls[0]![1] as RequestInit)
      .headers as Record<string, string>;
    expect(headers.Authorization).toBeUndefined();
  });
});
