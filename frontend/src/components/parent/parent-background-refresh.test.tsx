import type { ReactNode } from "react";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ParentResourceTestProvider } from "~/test/parent-resource-provider";
import { useParentChildren } from "~/lib/hooks/use-parent-resources";
import { useSwrResourceLoadError } from "~/lib/hooks/use-swr-load-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ApiError } from "~/lib/api-error";
import * as api from "~/lib/parent-api";
import { ChildMasterDataView } from "./child-master-data";
import {
  ChildRequestsProvider,
  SharedParentFeaturesProvider,
} from "./child-requests-context";
import { ChildPage } from "./child/child-page";
import { ParentMessagesPage } from "./messages/parent-messages-page";
import { ParentRealtimeBridge } from "./parent-realtime-bridge";
vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("~/lib/hooks/use-sse", () => ({ useSSE: () => undefined }));
const params = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
  useSearchParams: () => params,
}));
const child: api.Child = {
  student_id: "42",
  tenant_id: "7",
  first_name: "Felix",
  last_name: "Schneider",
  school_name: "Schule",
  school_class: "1a",
  school_slug: "schule",
  status: "active",
};
const unavailable = new ApiError("offline", 503, {
  code: "general.unavailable",
});
beforeEach(async () => {
  await import("~/lib/error-presentation");
  vi.restoreAllMocks();
  vi.spyOn(api, "listMyChildren").mockResolvedValue([child]);
  vi.spyOn(api, "getChildFeatures").mockResolvedValue({
    sick_note_enabled: true,
    notes_enabled: true,
  } as api.ChildFeatures);
  vi.spyOn(api, "getChildToday").mockResolvedValue(api.UNKNOWN_CHILD_TODAY);
  vi.spyOn(api, "getChildCareSchedule").mockResolvedValue({
    weekdays: [],
    today_absent: false,
    can_request: false,
    request_capabilities: {
      pickup: false,
      arrival: false,
      departure_mode: false,
    },
  });
  vi.spyOn(api, "listSickDays").mockResolvedValue([]);
  vi.spyOn(api, "listExcusedRequests").mockResolvedValue([]);
  vi.spyOn(api, "listCareExceptions").mockResolvedValue([]);
  vi.spyOn(api, "listPickupChangeRequests").mockResolvedValue([]);
  vi.spyOn(api, "getChildMasterData").mockResolvedValue({
    pending_changes: [],
  } as unknown as api.ChildMasterData);
  vi.spyOn(api, "fetchChildParentConferences").mockResolvedValue([]);
  vi.spyOn(api, "getChildCareOfferings").mockResolvedValue({
    offerings: [],
    can_request: false,
  });
  vi.spyOn(api, "getChildCourses").mockResolvedValue({
    enabled: false,
    items: [],
    pending_submitted_by_self: false,
    other_request_pending: false,
  });
  vi.spyOn(api, "getRequestSharingOptions").mockResolvedValue({
    family_protected: false,
    recipients: [],
  });
  vi.spyOn(api, "listMessageThreads").mockResolvedValue([]);
  vi.spyOn(api, "listChildThreads").mockResolvedValue([]);
  vi.spyOn(api, "listGroupThreads").mockResolvedValue([]);
  vi.spyOn(api, "getChildConversation").mockResolvedValue({
    thread_id: "thread-42",
    student_id: "42",
    student_name: "Felix Schneider",
    school_name: "Schule",
    counterpart_name: "OGS Schule",
    messages: [],
  });
});
async function failChildrenRefresh() {
  vi.mocked(api.listMyChildren).mockRejectedValue(unavailable);
  await act(async () => {
    window.dispatchEvent(new CustomEvent("parent-child-status-refresh"));
  });
  await waitFor(() => expect(api.listMyChildren).toHaveBeenCalledTimes(2));
}
async function failResourceRefresh(source: "children" | "features" | "today") {
  if (source === "children") return failChildrenRefresh();
  const request =
    source === "features"
      ? vi.mocked(api.getChildFeatures)
      : vi.mocked(api.getChildToday);
  request.mockRejectedValue(unavailable);
  await act(async () => {
    window.dispatchEvent(new CustomEvent("parent-child-status-refresh"));
  });
  await waitFor(() => expect(request).toHaveBeenCalledTimes(2));
}
function mount(page: ReactNode) {
  return render(
    <>
      <ParentRealtimeBridge />
      {page}
    </>,
    { wrapper: ParentResourceTestProvider },
  );
}
describe("parent background refresh preserves mounted content", () => {
  it.each(["children", "features", "today"] as const)(
    "keeps the sickness draft after a %s 503",
    async (source) => {
      mount(<ChildPage studentId="42" />);
      fireEvent.click(
        await screen.findByRole("button", { name: /Felix abmelden/ }),
      );
      const dialog = await screen.findByRole("dialog");
      const draft = within(dialog).getByRole("textbox");
      fireEvent.change(draft, { target: { value: "Arzttermin am Vormittag" } });
      await failResourceRefresh(source);
      expect(screen.getByTestId("child-profile-icon")).toBeInTheDocument();
      expect(screen.getByRole("dialog")).toBe(dialog);
      expect(draft).toHaveValue("Arzttermin am Vormittag");
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    },
  );
  it.each(["children", "features"] as const)(
    "keeps the single-child chat draft after a %s 503",
    async (source) => {
      mount(<ParentMessagesPage />);
      const draft = await screen.findByRole("textbox", { name: "Nachricht" });
      fireEvent.change(draft, { target: { value: "Noch nicht gesendet" } });
      await failResourceRefresh(source);
      expect(screen.getByRole("textbox", { name: "Nachricht" })).toBe(draft);
      expect(draft).toHaveValue("Noch nicht gesendet");
    },
  );
});

it("keeps an unsaved master-data field after a feature background failure", async () => {
  vi.mocked(api.getChildFeatures).mockResolvedValue({
    master_data_edit_enabled: true,
  } as api.ChildFeatures);
  vi.mocked(api.getChildMasterData).mockResolvedValue({
    student_id: "42",
    first_name: "Felix",
    last_name: "Schneider",
    birthday: "2018-03-04",
    school_class: "1a",
    status: "active",
    health_info: "Allergie",
    pending_changes: [],
  } as unknown as api.ChildMasterData);
  mount(
    <SharedParentFeaturesProvider linkedChildren={[child]}>
      <ChildRequestsProvider>
        <ChildMasterDataView studentId="42" childName="Felix" />
      </ChildRequestsProvider>
    </SharedParentFeaturesProvider>,
  );
  const field = await screen.findByDisplayValue("Allergie");
  fireEvent.change(field, { target: { value: "Ungespeicherter Hinweis" } });
  await failResourceRefresh("features");
  expect(screen.getByDisplayValue("Ungespeicherter Hinweis")).toBe(field);
});

function SharedLoadErrorFixture() {
  const resource = useParentChildren();
  const { error, failure } = useSwrResourceLoadError([resource], "Kinder", () =>
    resource.mutate(),
  );
  if (failure) return <LoadErrorAlert error={error} />;
  return <output>{resource.data ? "loaded" : "loading"}</output>;
}

describe("shared SWR load error semantics", () => {
  it("shows the error card when the initial request has no data", async () => {
    vi.mocked(api.listMyChildren).mockRejectedValue(unavailable);
    mount(<SharedLoadErrorFixture />);
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
  it("treats a confirmed empty list as data during a failed revalidation", async () => {
    vi.mocked(api.listMyChildren).mockResolvedValue([]);
    mount(<SharedLoadErrorFixture />);
    const content = await screen.findByText("loaded");
    await failChildrenRefresh();
    expect(screen.getByText("loaded")).toBe(content);
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
