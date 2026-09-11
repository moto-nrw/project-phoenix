import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

import {
  transferExportViaSFTP,
  transferFailureMessage,
} from "./sftp-export-api";

vi.mock("./session-cache", () => ({ sessionFetch: vi.fn() }));

const { sessionFetch } = await import("./session-cache");
const fetchMock = vi.mocked(sessionFetch);

function jsonResponse(data: unknown) {
  return {
    ok: true,
    status: 200,
    statusText: "OK",
    json: () => Promise.resolve({ data }),
  } as unknown as Response;
}

// Die Auswahl muss im BODY reisen. In der ersten Fassung stand sie in der
// Query, die die POST-Proxy-Route nicht weiterreicht — jede Übertragung kam
// ohne Parameter an und endete in einem 400.
describe("transferExportViaSFTP", () => {
  beforeEach(() => {
    fetchMock.mockReset();
    fetchMock.mockResolvedValue(
      jsonResponse({
        transferred: true,
        filename: "zeitkonten-2026-06.csv",
        byte_size: 42,
      }),
    );
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("sendet die Auswahl als JSON-Body, nicht als Query", async () => {
    await transferExportViaSFTP({
      year: 2026,
      month: 6,
      format: "datev_lodas",
    });

    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0]!;
    expect(url).toBe("/api/staff/time-tracking/export/sftp");
    expect(url).not.toContain("?");
    expect(init?.method).toBe("POST");
    expect(JSON.parse(String(init?.body))).toEqual({
      year: 2026,
      month: 6,
      format: "datev_lodas",
    });
  });

  it("lässt den Monat weg, wenn das ganze Jahr übertragen wird", async () => {
    await transferExportViaSFTP({
      year: 2026,
      month: 6,
      format: "csv",
      wholeYear: true,
      granularity: "month",
      timeFormat: "decimal",
    });

    const [, init] = fetchMock.mock.calls[0]!;
    expect(JSON.parse(String(init?.body))).toEqual({
      year: 2026,
      format: "csv",
      granularity: "month",
      time_format: "decimal",
    });
  });

  it("bildet das Ergebnis auf camelCase ab", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        transferred: false,
        filename: "zeitkonten-2026-06.csv",
        byte_size: 0,
        target_host: "dateien.beispiel.de",
        target_directory: "/upload/lohn",
        reason: "host_key_mismatch",
      }),
    );

    const outcome = await transferExportViaSFTP({
      year: 2026,
      month: 6,
      format: "csv",
    });

    expect(outcome.transferred).toBe(false);
    expect(outcome.targetHost).toBe("dateien.beispiel.de");
    expect(outcome.targetDirectory).toBe("/upload/lohn");
    expect(outcome.reason).toBe("host_key_mismatch");
  });
});

describe("transferFailureMessage", () => {
  it("nennt zu jedem Grund einen verständlichen Satz", () => {
    for (const reason of [
      "not_configured",
      "address_denied",
      "host_key_mismatch",
      "authentication_rejected",
      "connection_failed",
      "upload_failed",
      "file_too_large",
      "internal_error",
    ]) {
      const message = transferFailureMessage(reason);
      expect(message.length).toBeGreaterThan(10);
      // Kein Grund-Code und kein Fachjargon im Text für die Schule.
      expect(message).not.toContain(reason);
      expect(message).not.toMatch(/SFTP|SSH|Server|Socket/);
    }
  });

  it("fällt bei unbekanntem Grund auf eine allgemeine Meldung zurück", () => {
    expect(transferFailureMessage("etwas-neues")).toBe(
      transferFailureMessage("internal_error"),
    );
    expect(transferFailureMessage(undefined)).toBe(
      transferFailureMessage("internal_error"),
    );
  });
});
