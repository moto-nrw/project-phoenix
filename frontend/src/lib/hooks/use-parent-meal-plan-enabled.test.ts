import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ParentResourceTestProvider } from "~/test/parent-resource-provider";
import { useInvalidateMatching } from "~/lib/swr";
import { parentKeys } from "~/lib/swr/parent-keys";
import {
  listMyChildren,
  getChildFeatures,
  listAnnouncements,
  type Child,
} from "~/lib/parent-api";
import { useParentMealPlanEnabled } from "./use-parent-meal-plan-enabled";

vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("~/lib/parent-api", () => ({
  listMyChildren: vi.fn(),
  getChildFeatures: vi.fn(),
  listAnnouncements: vi.fn(),
}));
const children = [
  { tenant_id: "1", student_id: "10" },
  { tenant_id: "1", student_id: "11" },
  { tenant_id: "2", student_id: "20" },
] as Child[];
beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(listMyChildren).mockResolvedValue(children);
  vi.mocked(getChildFeatures).mockResolvedValue({
    meal_plan_enabled: false,
  } as Awaited<ReturnType<typeof getChildFeatures>>);
  vi.mocked(listAnnouncements).mockResolvedValue([]);
});
function mount(enabled = true) {
  return renderHook(
    () => ({
      enabled: useParentMealPlanEnabled(enabled),
      refresh: useInvalidateMatching(),
    }),
    { wrapper: ParentResourceTestProvider },
  );
}
describe("useParentMealPlanEnabled with real SWR", () => {
  it("does not fetch in another portal or disabled mode", () => {
    const { result } = mount(false);
    expect(result.current.enabled).toBe(false);
    expect(listMyChildren).not.toHaveBeenCalled();
    expect(getChildFeatures).not.toHaveBeenCalled();
  });
  it("checks every relationship, including siblings at the same school", async () => {
    vi.mocked(getChildFeatures).mockImplementation(
      async (id) =>
        ({ meal_plan_enabled: id === "11" }) as Awaited<
          ReturnType<typeof getChildFeatures>
        >,
    );
    const { result } = mount();
    await waitFor(() => expect(result.current.enabled).toBe(true));
    expect(getChildFeatures).toHaveBeenCalledTimes(3);
    expect(getChildFeatures).toHaveBeenCalledWith("10");
    expect(getChildFeatures).toHaveBeenCalledWith("11");
    expect(getChildFeatures).toHaveBeenCalledWith("20");
    expect(listMyChildren).toHaveBeenCalledTimes(1);
  });
  it("stays hidden until availability is confirmed", async () => {
    const { result } = mount();
    await waitFor(() => expect(getChildFeatures).toHaveBeenCalledTimes(3));
    expect(result.current.enabled).toBe(false);
  });
  it("does not cache a failed lookup as false and recovers on revalidation", async () => {
    vi.mocked(getChildFeatures).mockRejectedValue(new Error("offline"));
    const { result } = mount();
    await waitFor(() => expect(getChildFeatures).toHaveBeenCalledTimes(3));
    expect(result.current.enabled).toBe(false);
    vi.mocked(getChildFeatures).mockResolvedValue({
      meal_plan_enabled: true,
    } as Awaited<ReturnType<typeof getChildFeatures>>);
    await act(async () => {
      await result.current.refresh(parentKeys.isResource);
    });
    await waitFor(() => expect(result.current.enabled).toBe(true));
  });
  it("has no discoverable feature without children or feed", async () => {
    vi.mocked(listMyChildren).mockResolvedValue([]);
    const { result } = mount();
    await waitFor(() => expect(listMyChildren).toHaveBeenCalledOnce());
    expect(result.current.enabled).toBe(false);
    expect(getChildFeatures).not.toHaveBeenCalled();
  });
});
