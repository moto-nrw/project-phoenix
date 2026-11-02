import { act, renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ParentResourceTestProvider } from "~/test/parent-resource-provider";
import { PARENT_ACCESS_REFUSED_EVENT } from "~/lib/parent-access-refusal";
import { bindSessionAccount } from "~/lib/session-account";
import {
  useParentChildren,
  useParentFeatures,
  useParentResourceEvents,
  useParentToday,
} from "~/lib/hooks/use-parent-resources";
vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
const apiSignals = vi.fn();
beforeEach(() => {
  apiSignals.mockClear();
  window.addEventListener(PARENT_ACCESS_REFUSED_EVENT, apiSignals);
});
afterEach(() => {
  window.removeEventListener(PARENT_ACCESS_REFUSED_EVENT, apiSignals);
  bindSessionAccount(null);
  vi.restoreAllMocks();
});
it.each(["features", "today"] as const)(
  "handles one real %s refusal once across the API event and SWR error paths",
  async (resource) => {
    bindSessionAccount("parent-test");
    let refused = false;
    const request = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async (input) => {
        const url = String(input);
        if (url.endsWith("/children"))
          return Response.json({
            data: refused ? [] : [{ student_id: "42", tenant_id: "7" }],
          });
        if (refused && url.endsWith(`/${resource}`))
          return Response.json(
            { code: "care.child_not_linked" },
            { status: 403 },
          );
        if (url.endsWith("/features"))
          return Response.json({ data: { sick_note_enabled: true } });
        if (url.endsWith("/today"))
          return Response.json({ data: { at_ogs: null, state: "unknown" } });
        throw new Error(`Unexpected transport request ${url}`);
      });
    const broadcast = vi.spyOn(BroadcastChannel.prototype, "postMessage");
    const { result } = renderHook(
      () => {
        useParentResourceEvents();
        const children = useParentChildren();
        return {
          children,
          features: useParentFeatures(children.data),
          today: useParentToday(children.data),
        };
      },
      { wrapper: ParentResourceTestProvider },
    );
    await waitFor(() =>
      expect(result.current.features.data?.["42"]?.sick_note_enabled).toBe(
        true,
      ),
    );
    await waitFor(() =>
      expect(result.current.today.data?.["42"]?.state).toBe("unknown"),
    );
    refused = true;
    await act(async () => {
      await result.current[resource].mutate();
    });
    await waitFor(() => expect(result.current.children.data).toEqual([]));
    expect(apiSignals).toHaveBeenCalledOnce();
    expect(
      broadcast.mock.calls.filter(
        ([value]) => value?.code === "care.child_not_linked",
      ),
    ).toHaveLength(1);
    expect(
      request.mock.calls.filter(([url]) => String(url).endsWith("/children")),
    ).toHaveLength(2);
  },
);
