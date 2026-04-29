/**
 * Shared fixtures and SWR mock factory used by the per-page operator tests.
 *
 * Per-page pages each consume a small subset of SWR keys — the helper mirrors
 * the original provisioning/page.test.tsx setup so tests can remain
 * byte-similar to their original form.
 */

// SWR mutate functions are passed in as vi.fn() from per-page tests. The
// tests only care that they're callable — they don't inspect return values
// or argument types — so keep the signatures deliberately loose here.
type MutateFn = (...args: unknown[]) => unknown;

export const mockOrg = {
  id: "1",
  name: "Test Org",
  slug: "test-org",
  active: true,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  schulenCount: 1,
  kontenCount: 2,
  geraeteCount: 3,
  personenCount: 4,
};

export const mockSchool = {
  id: "10",
  organizationId: "1",
  name: "Test School",
  slug: "test-school",
  subdomain: "test-school",
  address: "Main St 1",
  city: "Berlin",
  zip: "10115",
  phone: "",
  email: "",
  active: true,
  hidden: false,
  deletedAt: null,
  createdAt: "2025-01-01T00:00:00Z",
  updatedAt: "2025-01-01T00:00:00Z",
  organizationName: "Test Org",
  kontenCount: 2,
  geraeteCount: 3,
  personenCount: 4,
  organization: { ...mockOrg },
};

export interface SetupSWROptions {
  useSWRMock: { mockImplementation: (fn: (key: unknown) => unknown) => void };
  mutateOrgs?: MutateFn;
  mutateSchools?: MutateFn;
  mutateSchoolPersons?: MutateFn;
  orgs?: unknown[] | undefined;
  schools?: unknown[] | undefined;
  orgsLoading?: boolean;
  schoolsLoading?: boolean;
  schoolAccounts?: unknown[];
  orgAccounts?: unknown[];
  allAccounts?: unknown[];
  accountsLoading?: boolean;
  schoolDevices?: unknown[];
  orgDevices?: unknown[];
  allDevices?: unknown[];
  devicesLoading?: boolean;
  schoolPersons?: unknown[];
  personsLoading?: boolean;
}

/**
 * Installs a default SWR mock that responds to every provisioning-related
 * key. Per-page tests pass in the mockUseSWR they hoisted at the top of the
 * file (vi.hoisted must be called in the test file itself).
 */
export function setupSWR(opts: SetupSWROptions): void {
  const {
    useSWRMock,
    mutateOrgs,
    mutateSchools,
    mutateSchoolPersons,
    orgs = [mockOrg],
    schools = [mockSchool],
    orgsLoading = false,
    schoolsLoading = false,
    schoolAccounts,
    orgAccounts,
    allAccounts,
    accountsLoading = false,
    schoolDevices,
    orgDevices,
    allDevices,
    devicesLoading = false,
    schoolPersons,
    personsLoading = false,
  } = opts;

  useSWRMock.mockImplementation((key: unknown) => {
    if (typeof key !== "string") {
      return { data: undefined, isLoading: false, mutate: () => undefined };
    }
    if (
      key === "operator-organizations" ||
      key === "operator-organization-summaries"
    ) {
      return {
        data: orgsLoading ? undefined : orgs,
        isLoading: orgsLoading,
        mutate: mutateOrgs,
      };
    }
    if (key === "operator-provisioning-stats") {
      return {
        data: {
          traegerCount: orgs.length,
          schulenCount: schools.length,
          kontenCount: 0,
          geraeteCount: 0,
          personenCount: 0,
        },
        isLoading: false,
        mutate: () => undefined,
      };
    }
    if (key === "operator-schools" || key === "operator-school-summaries") {
      return {
        data: schoolsLoading ? undefined : schools,
        isLoading: schoolsLoading,
        mutate: mutateSchools,
      };
    }
    if (key === "operator-all-accounts") {
      return {
        data: accountsLoading ? undefined : (allAccounts ?? []),
        isLoading: accountsLoading,
        mutate: () => undefined,
      };
    }
    if (key.startsWith("operator-school-accounts-")) {
      return {
        data: accountsLoading ? undefined : (schoolAccounts ?? []),
        isLoading: accountsLoading,
        mutate: () => undefined,
      };
    }
    if (key.startsWith("operator-org-accounts-")) {
      return {
        data: accountsLoading ? undefined : (orgAccounts ?? []),
        isLoading: accountsLoading,
        mutate: () => undefined,
      };
    }
    if (key === "operator-all-devices") {
      return {
        data: devicesLoading ? undefined : (allDevices ?? []),
        isLoading: devicesLoading,
        mutate: () => undefined,
      };
    }
    if (key.startsWith("operator-school-devices-")) {
      return {
        data: devicesLoading ? undefined : (schoolDevices ?? []),
        isLoading: devicesLoading,
        mutate: () => undefined,
      };
    }
    if (key.startsWith("operator-org-devices-")) {
      return {
        data: devicesLoading ? undefined : (orgDevices ?? []),
        isLoading: devicesLoading,
        mutate: () => undefined,
      };
    }
    if (key.startsWith("operator-school-persons-")) {
      return {
        data: personsLoading ? undefined : (schoolPersons ?? []),
        isLoading: personsLoading,
        mutate: mutateSchoolPersons ?? (() => undefined),
      };
    }
    return { data: undefined, isLoading: false, mutate: () => undefined };
  });
}
