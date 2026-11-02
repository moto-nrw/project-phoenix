import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PayrollPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mutateMock, setSettingValueMock, swrData, swrKeys } = vi.hoisted(
  () => ({
    mutateMock: vi.fn(),
    setSettingValueMock: vi.fn(),
    // Per SWR key: the page loads the status, the latest dispatch and, for
    // the SFTP way, whether the counterpart is ready.
    swrData: {} as Record<string, { data?: unknown; error?: unknown }>,
    swrKeys: [] as Array<string | null>,
  }),
);

function payrollStatus(dispatch: Record<string, unknown> = {}) {
  return {
    categories: [
      {
        id: "regular",
        label: "Regelarbeit",
        number: "",
        unit: "",
        unitRequired: false,
        settingKey: "payroll.lohnart_regelarbeit",
        unitSettingKey: null,
      },
    ],
    beraternummer: "",
    mandantennummer: "",
    lodasHeaderComplete: false,
    configuredCategories: 0,
    totalCategories: 1,
    staffTotal: 1,
    staffWithoutPersonnelNumber: 1,
    dispatch: {
      auto: false,
      channel: "sftp",
      format: "csv",
      email: "",
      ...dispatch,
    },
  };
}

// The page reads through useSWRAuth with factory keys; the wrapper adds the
// session's school and account (tested in ~/lib/swr).
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    swrKeys.push(key);
    return {
      data: key ? swrData[key]?.data : undefined,
      error: key ? swrData[key]?.error : undefined,
      mutate: mutateMock,
    };
  },
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => ({
    isReady: true,
    isLoading: false,
  }),
}));

vi.mock("~/lib/payroll-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/payroll-api")>()),
  savePayrollSetting: setSettingValueMock,
}));

function resetSwr(dispatch: Record<string, unknown> = {}) {
  for (const key of Object.keys(swrData)) delete swrData[key];
  swrData["payroll-status"] = { data: payrollStatus(dispatch) };
  swrData["payroll-dispatch"] = { data: null };
  swrData["payroll-sftp-status"] = {
    data: { enabled: true, ready: true, missingSettings: [] },
  };
}

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise;
  });
  return { promise, resolve };
}

describe("PayrollPage autosave", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrKeys.length = 0;
    mutateMock.mockResolvedValue(undefined);
    resetSwr();
  });

  it("serializes overlapping saves for the same setting key", async () => {
    const firstSave = deferred<string | null>();
    setSettingValueMock
      .mockImplementationOnce(() => firstSave.promise)
      .mockResolvedValueOnce(null);

    render(<PayrollPage />);

    // Die Handyansicht trägt dieselben Felder ein zweites Mal (#4975).
    const input = within(screen.getByTestId("data-table-table")).getByRole(
      "textbox",
      { name: "Lohnartnummer Regelarbeit" },
    );
    fireEvent.change(input, { target: { value: "1001" } });
    fireEvent.blur(input);

    await waitFor(() => {
      expect(setSettingValueMock).toHaveBeenCalledTimes(1);
    });

    fireEvent.change(input, { target: { value: "1002" } });
    fireEvent.blur(input);

    expect(setSettingValueMock).toHaveBeenCalledTimes(1);

    await act(async () => {
      firstSave.resolve(null);
      await firstSave.promise;
    });

    await waitFor(() => {
      expect(setSettingValueMock).toHaveBeenCalledTimes(2);
    });
    expect(setSettingValueMock).toHaveBeenNthCalledWith(
      1,
      "payroll.lohnart_regelarbeit",
      "1001",
    );
    expect(setSettingValueMock).toHaveBeenNthCalledWith(
      2,
      "payroll.lohnart_regelarbeit",
      "1002",
    );
    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalledTimes(2);
    });
  });

  it("zeigt einen Speicherfehler über den Karten und wiederholt genau diesen Wert", async () => {
    setSettingValueMock
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(undefined);

    render(<PayrollPage />);

    // Die Handyansicht trägt dieselben Felder ein zweites Mal (#4975).
    const input = within(screen.getByTestId("data-table-table")).getByRole(
      "textbox",
      { name: "Lohnartnummer Regelarbeit" },
    );
    fireEvent.change(input, { target: { value: "1001" } });
    fireEvent.blur(input);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Einstellung"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(setSettingValueMock).toHaveBeenCalledTimes(2);
    });
    expect(setSettingValueMock).toHaveBeenLastCalledWith(
      "payroll.lohnart_regelarbeit",
      "1001",
    );
  });
});

describe("PayrollPage Versand an das Lohnbüro (#4805)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrKeys.length = 0;
    mutateMock.mockResolvedValue(undefined);
    // Like the backend: a saved Lohnbüro value comes back with the next load.
    setSettingValueMock.mockImplementation((key: string, value: unknown) => {
      const field = key.replace("payroll.dispatch_", "");
      const status = swrData["payroll-status"]?.data as ReturnType<
        typeof payrollStatus
      >;
      swrData["payroll-status"] = {
        data: { ...status, dispatch: { ...status.dispatch, [field]: value } },
      };
      return Promise.resolve(undefined);
    });
    resetSwr();
  });

  it("reads payroll data through the session-scoped wrapper", () => {
    render(<PayrollPage />);

    expect(swrKeys).toContain("payroll-status");
    expect(swrKeys).toContain("payroll-dispatch");
    expect(swrKeys).toContain("payroll-sftp-status");
  });

  it("saves the switch as a boolean", async () => {
    render(<PayrollPage />);

    const toggle = screen.getByRole("switch", {
      name: "Automatisch nach dem Monatsabschluss senden",
    });
    expect(toggle).toHaveAttribute("aria-checked", "false");
    fireEvent.click(toggle);

    await waitFor(() => {
      expect(setSettingValueMock).toHaveBeenCalledWith(
        "payroll.dispatch_auto",
        true,
      );
    });
  });

  it("shows the e-mail address only for the e-mail way and saves it on blur", async () => {
    render(<PayrollPage />);

    // Anker: der SFTP-Weg ist gewählt und erklärt.
    expect(
      screen.getByText(
        "moto legt die Datei auf der Gegenstelle ab, die in den Einstellungen bei Zeitkonten übertragen steht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("textbox", { name: "E-Mail-Adresse des Lohnbüros" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "E-Mail" }));

    const input = await screen.findByRole("textbox", {
      name: "E-Mail-Adresse des Lohnbüros",
    });
    expect(setSettingValueMock).toHaveBeenCalledWith(
      "payroll.dispatch_channel",
      "email",
    );
    expect(
      screen.getByText(
        "Das Lohnbüro bekommt einen Link, keinen Anhang. Der Link gilt 14 Tage und funktioniert nur einmal.",
      ),
    ).toBeInTheDocument();

    fireEvent.change(input, { target: { value: "lohn@" } });
    fireEvent.blur(input);
    expect(
      screen.getByText("Bitte geben Sie eine vollständige E-Mail-Adresse ein."),
    ).toBeInTheDocument();
    expect(setSettingValueMock).not.toHaveBeenCalledWith(
      "payroll.dispatch_email",
      expect.anything(),
    );

    fireEvent.change(input, { target: { value: " lohn@example.de " } });
    fireEvent.blur(input);
    await waitFor(() => {
      expect(setSettingValueMock).toHaveBeenCalledWith(
        "payroll.dispatch_email",
        "lohn@example.de",
      );
    });
  });

  it("warns when the e-mail way is switched on without an address", () => {
    resetSwr({ auto: true, channel: "email" });

    render(<PayrollPage />);

    expect(
      screen.getByText(
        "Tragen Sie die E-Mail-Adresse des Lohnbüros ein. Ohne sie sendet moto nichts.",
      ),
    ).toBeInTheDocument();
  });

  it("points to the settings while the SFTP counterpart is not ready", () => {
    swrData["payroll-sftp-status"] = {
      data: { enabled: false, ready: false, missingSettings: [] },
    };

    render(<PayrollPage />);

    expect(
      screen.getByText(
        "Die Übertragung per SFTP ist ausgeschaltet. Sie steht in den Einstellungen unter Kalender und Export bei Zeitkonten übertragen. Bis dahin sendet moto nichts.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zu den Einstellungen" }),
    ).toHaveAttribute("href", "/settings?tab=system&highlight=sftp.enabled");
  });

  it("drops the SFTP hint without an error when the status is forbidden", () => {
    swrData["payroll-sftp-status"] = {
      error: new ApiError("forbidden", 403, { code: "general.permission" }),
    };

    render(<PayrollPage />);

    // Anker: die Karte ist geladen, der SFTP-Weg gewählt.
    expect(
      screen.getByText(
        "moto legt die Datei auf der Gegenstelle ab, die in den Einstellungen bei Zeitkonten übertragen steht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zu den Einstellungen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("warns about missing DATEV details for a DATEV format", () => {
    resetSwr({ format: "datev_lug" });

    render(<PayrollPage />);

    expect(
      screen.getByText(
        "Für DATEV fehlen noch Angaben. Was fehlt, steht oben unter Vollständigkeit. Fehlt beim Monatsabschluss etwas, sendet moto keine Datei.",
      ),
    ).toBeInTheDocument();
  });

  it("shows a blocked latest dispatch with its reason", () => {
    swrData["payroll-dispatch"] = {
      data: {
        year: 2026,
        month: 8,
        correction: true,
        channel: "sftp",
        format: "datev_lodas",
        recipient: null,
        state: "blocked",
        reason: "datev_incomplete",
        attempts: 0,
        filename: null,
        createdAt: "2026-09-01T08:00:00Z",
        finishedAt: "2026-09-01T08:00:05Z",
        downloadExpiresAt: null,
        downloadedAt: null,
      },
    };

    render(<PayrollPage />);

    expect(screen.getByText("Letzter Versand")).toBeInTheDocument();
    expect(screen.getByText("August 2026")).toBeInTheDocument();
    expect(screen.getByText("Ja")).toBeInTheDocument();
    expect(screen.getByText("Nicht gesendet")).toBeInTheDocument();
    expect(screen.getByText("DATEV LODAS")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die DATEV-Angaben sind nicht vollständig. Zum Beispiel fehlt eine Personalnummer oder die Nummer einer Lohnart.",
      ),
    ).toBeInTheDocument();
  });

  it("shows when the e-mailed link expires and whether it was used", () => {
    swrData["payroll-dispatch"] = {
      data: {
        year: 2026,
        month: 8,
        correction: false,
        channel: "email",
        format: "xlsx",
        recipient: "lohn@example.de",
        state: "sent",
        reason: null,
        attempts: 1,
        filename: "zeitkonten_2026-08.xlsx",
        createdAt: "2026-09-01T08:00:00Z",
        finishedAt: "2026-09-01T08:01:00Z",
        downloadExpiresAt: "2026-09-15T08:00:00Z",
        downloadedAt: "2026-09-02T07:30:00Z",
      },
    };

    render(<PayrollPage />);

    expect(screen.getByText("E-Mail an lohn@example.de")).toBeInTheDocument();
    expect(screen.getByText("E-Mail zugestellt")).toBeInTheDocument();
    expect(screen.getByText("15.09.2026, 10:00")).toBeInTheDocument();
    expect(screen.getByText("02.09.2026, 09:30")).toBeInTheDocument();
  });

  it("says that nothing was sent yet", () => {
    resetSwr({ auto: true });

    render(<PayrollPage />);

    expect(
      screen.getByText(
        "Noch nichts gesendet. Der erste Versand startet nach dem nächsten Monatsabschluss.",
      ),
    ).toBeInTheDocument();
  });
});
