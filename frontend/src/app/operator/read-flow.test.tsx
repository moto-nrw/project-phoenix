import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
import { useSWRAuth } from "~/lib/swr";
import { operatorProvisioningKeys } from "~/lib/swr/portal-keys";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  OrganizationSummary,
  SchoolSummary,
} from "~/lib/operator/provisioning-helpers";
import AccountsPage from "./accounts/page";
import DevicesPage from "./devices/page";
import PersonsPage from "./persons/page";
import TagsPage from "./unregistered-tags/page";
import SchoolPage from "./organizations/[slug]/schools/[schoolSlug]/page";

vi.unmock("swr");
const { state, api } = vi.hoisted(() => ({
  state: { query: "schoolId=10", replace: vi.fn() },
  api: {
    listOrganizations: vi.fn(),
    listSchools: vi.fn(),
    listSchoolAccounts: vi.fn(),
    listAllAccounts: vi.fn(),
    listOrganizationAccounts: vi.fn(),
    listSchoolDevices: vi.fn(),
    listAllDevices: vi.fn(),
    listOrganizationDevices: vi.fn(),
    listSchoolPersons: vi.fn(),
    listUnregisteredTagScans: vi.fn(),
    listOrganizationSummaries: vi.fn(),
    listSchoolSummaries: vi.fn(),
    listOrganizationSchools: vi.fn(),
    getSchoolPWAUsage: vi.fn(),
  },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-a", scope: "platform" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({ useTenantSlugSafe: () => null }));
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(state.query),
  usePathname: () => "/operator/accounts",
  useRouter: () => ({ replace: state.replace, push: vi.fn() }),
}));
vi.mock("~/lib/breadcrumb-context", () => ({ useSetBreadcrumb: vi.fn() }));
vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: api,
  revalidateTenantCache: vi.fn(),
}));
vi.mock("~/components/operator/accounts-table", () => ({
  AccountsTable: ({
    onManageTenantAccess,
  }: {
    onManageTenantAccess?: (account: object) => void;
  }) => (
    <button
      onClick={() =>
        onManageTenantAccess?.({
          accountId: "1",
          firstName: "Test",
          lastName: "Person",
          email: "person@example.test",
        })
      }
    >
      Zugang ändern
    </button>
  ),
}));
vi.mock("~/components/operator/account-tenant-access-modal", () => ({
  AccountTenantAccessModal: ({
    onUpdated,
  }: {
    onUpdated: () => Promise<void>;
  }) => <button onClick={() => void onUpdated()}>Zugang speichern</button>,
}));

const org: OrganizationSummary = {
  id: "1",
  name: "Träger A",
  slug: "org-a",
  active: true,
  testSystem: false,
  deletedAt: null,
  createdAt: "2026-09-09T10:00:00Z",
  updatedAt: "2026-09-09T10:00:00Z",
  schulenCount: 1,
  kontenCount: 1,
  geraeteCount: 0,
  personenCount: 0,
};
const school: SchoolSummary = {
  id: "10",
  organizationId: "1",
  organizationName: org.name,
  name: "Schule A",
  slug: "school-a",
  subdomain: "school-a",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: org.createdAt,
  updatedAt: org.updatedAt,
  address: "",
  city: "",
  zip: "",
  phone: "",
  email: "school@example.test",
  kontenCount: 1,
  geraeteCount: 0,
  personenCount: 0,
  childQuotaBundles: null,
  childQuotaBundleSize: 50,
  childQuotaCount: 1,
};
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
function renderPage(children: React.ReactNode) {
  const cache = new Map();
  const view = render(
    <SWRConfig value={{ provider: () => cache }}>
      <ToastProvider>
        <SWRScopeBoundary />
        {children}
      </ToastProvider>
    </SWRConfig>,
  );
  return { ...view, cache };
}

beforeEach(() => {
  vi.clearAllMocks();
  state.query = "schoolId=10";
  api.listOrganizations.mockResolvedValue([org]);
  api.listSchools.mockResolvedValue([{ ...school, organization: org }]);
  api.listSchoolAccounts.mockResolvedValue([]);
  api.listSchoolDevices.mockResolvedValue([]);
  api.listSchoolPersons.mockResolvedValue([]);
  api.listUnregisteredTagScans.mockResolvedValue([]);
  api.listOrganizationSummaries.mockResolvedValue([org]);
  api.listSchoolSummaries.mockResolvedValue([school]);
  api.getSchoolPWAUsage.mockResolvedValue({
    windowDays: 30,
    staff: { standaloneUsers: 0, eligibleUsers: 0 },
    parent: { standaloneUsers: 0, eligibleUsers: 0 },
  });
});

describe("operator reads with real SWR", () => {
  it.each([
    ["accounts", AccountsPage, api.listSchoolAccounts],
    ["devices", DevicesPage, api.listSchoolDevices],
    ["persons", PersonsPage, api.listSchoolPersons],
    ["RFID", TagsPage, api.listUnregisteredTagScans],
  ] as const)(
    "starts %s from the URL before school options arrive",
    async (_name, Page, read) => {
      api.listSchools.mockReturnValue(new Promise(() => undefined));
      renderPage(<Page />);
      await waitFor(() =>
        expect(read).toHaveBeenCalledWith(
          _name === "RFID"
            ? {
                organizationId: undefined,
                schoolId: "10",
                resolved: "unresolved",
              }
            : "10",
        ),
      );
    },
  );

  it("starts school and organization lookups together, then accounts and usage together", async () => {
    const organizations = deferred<OrganizationSummary[]>();
    api.listOrganizationSummaries.mockReturnValue(organizations.promise);
    const params = Promise.resolve({ slug: org.slug, schoolSlug: school.slug });
    await params;
    await act(async () => {
      renderPage(<SchoolPage params={params} />);
    });
    await waitFor(() =>
      expect(api.listSchoolSummaries).toHaveBeenCalledTimes(1),
    );
    await act(async () => {
      organizations.resolve([org]);
    });
    expect(await screen.findByText("Schule A")).toBeInTheDocument();
    expect(api.listSchoolAccounts).toHaveBeenCalledWith("10");
    expect(api.getSchoolPWAUsage).toHaveBeenCalledWith("10");
    // Anker: the matched school is fully resolved, not a permanent skeleton.
    expect(screen.getByText("Schule A")).toBeInTheDocument();
    expect(api.listOrganizationSchools).not.toHaveBeenCalled();
  });

  it("does not resolve a school slug under a different organization", async () => {
    api.listSchoolSummaries.mockResolvedValue([
      { ...school, organizationId: "2" },
    ]);
    const params = Promise.resolve({ slug: org.slug, schoolSlug: school.slug });
    await params;
    await act(async () => {
      renderPage(<SchoolPage params={params} />);
    });
    expect(
      await screen.findByText("Schule nicht gefunden."),
    ).toBeInTheDocument();
    // Anker: the wrong-organization route has resolved to its explicit not-found view.
    expect(screen.getByText("Schule nicht gefunden.")).toBeInTheDocument();
    expect(api.listSchoolAccounts).not.toHaveBeenCalled();
    expect(api.getSchoolPWAUsage).not.toHaveBeenCalled();
  });

  it("refreshes mounted detail and summary readers after a list write", async () => {
    let revision = 1;
    const detail = vi.fn(async () => `Detail ${revision}`);
    const summary = vi.fn(async () => `Übersicht ${revision}`);
    function DetailReaders() {
      const account = useSWRAuth(
        operatorProvisioningKeys.orgAccounts("1"),
        detail,
      );
      const count = useSWRAuth(
        operatorProvisioningKeys.organizationSummaries(),
        summary,
      );
      return (
        <p>
          {account.data} {count.data}
        </p>
      );
    }
    api.listSchoolAccounts.mockResolvedValue([{ accountId: "1" }]);
    renderPage(
      <>
        <AccountsPage />
        <DetailReaders />
      </>,
    );
    expect(await screen.findByText("Detail 1 Übersicht 1")).toBeInTheDocument();
    await screen.findByRole("button", { name: "Zugang ändern" });
    act(() => screen.getByRole("button", { name: "Zugang ändern" }).click());
    revision = 2;
    act(() => screen.getByRole("button", { name: "Zugang speichern" }).click());
    expect(await screen.findByText("Detail 2 Übersicht 2")).toBeInTheDocument();
  });
});
