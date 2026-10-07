import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PayrollPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mutateMock, setSettingValueMock } = vi.hoisted(() => ({
  mutateMock: vi.fn(),
  setSettingValueMock: vi.fn(),
}));

vi.mock("swr", () => ({
  default: () => ({
    data: {
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
    },
    error: undefined,
    mutate: mutateMock,
  }),
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
    mutateMock.mockResolvedValue(undefined);
  });

  it("serializes overlapping saves for the same setting key", async () => {
    const firstSave = deferred<string | null>();
    setSettingValueMock
      .mockImplementationOnce(() => firstSave.promise)
      .mockResolvedValueOnce(null);

    render(<PayrollPage />);

    const input = screen.getByRole("textbox", {
      name: "Lohnartnummer Regelarbeit",
    });
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

    const input = screen.getByRole("textbox", {
      name: "Lohnartnummer Regelarbeit",
    });
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
