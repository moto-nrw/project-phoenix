import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import PayrollPage from "./page";

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

vi.mock("~/lib/settings-api", () => ({
  setSettingValue: setSettingValueMock,
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
});
