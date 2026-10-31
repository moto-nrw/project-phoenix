import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { ParentResourceTestProvider } from "~/test/parent-resource-provider";
import * as api from "~/lib/parent-api";
import { ApiError } from "~/lib/api-error";
import { useChildCare } from "./child-care";
import { useChildMasterData } from "./child-master-data";
import {
  ChildRequestsProvider,
  SharedParentFeaturesProvider,
} from "./child-requests-context";
vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));

const children = [{ student_id: "42", tenant_id: "7" }] as api.Child[];
const features = {
  sick_note_enabled: true,
  master_data_edit_enabled: true,
} as api.ChildFeatures;
function wrapper({ children: content }: Readonly<{ children: ReactNode }>) {
  return (
    <ParentResourceTestProvider>
      <SharedParentFeaturesProvider linkedChildren={children}>
        <ChildRequestsProvider>{content}</ChildRequestsProvider>
      </SharedParentFeaturesProvider>
    </ParentResourceTestProvider>
  );
}
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(api, "getChildFeatures").mockResolvedValue(features);
  vi.spyOn(api, "getChildMasterData").mockResolvedValue(
    {} as api.ChildMasterData,
  );
  vi.spyOn(api, "getChildCareSchedule").mockResolvedValue({
    weekdays: [],
    today_absent: false,
  } as unknown as api.ChildCareSchedule);
  vi.spyOn(api, "listSickDays").mockResolvedValue([]);
  vi.spyOn(api, "listExcusedRequests").mockResolvedValue([]);
  vi.spyOn(api, "listCareExceptions").mockResolvedValue([]);
  vi.spyOn(api, "listPickupChangeRequests").mockResolvedValue([]);
});
function mount() {
  return renderHook(
    () => ({ care: useChildCare("42"), master: useChildMasterData("42") }),
    { wrapper },
  );
}

describe("shared parent relationship features", () => {
  it("shares the confirmed flags between care and master-data sections", async () => {
    const { result } = mount();
    await waitFor(() =>
      expect(result.current.master.features).toEqual(features),
    );
    expect(result.current.care.features).toEqual(features);
    expect(api.getChildFeatures).toHaveBeenCalledTimes(1);
  });
  it("retries shared flags from care and clears the master-data error after recovery", async () => {
    vi.mocked(api.getChildFeatures).mockRejectedValue(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.master.error).not.toBeNull());
    expect(result.current.care.features.sick_note_enabled).toBe(false);
    vi.mocked(api.getChildFeatures).mockResolvedValue(features);
    await act(async () => {
      result.current.care.refresh();
    });
    await waitFor(() =>
      expect(result.current.master.features).toEqual(features),
    );
    expect(result.current.master.error).toBeNull();
    expect(result.current.care.features).toEqual(features);
  });
});
