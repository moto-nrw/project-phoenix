import { act, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SWRConfig } from "~/lib/swr/testing";
import { useSWRAuth, useSWRSeedFrom, useTenantMutateMatching } from "./index";
import { roomKeys } from "./rooms-keys";
import { staffKeys } from "./staff-keys";
import { studentKeys } from "./portal-keys";
import { resetSWRScope, resumeSWRSession } from "./scope";
import { StudentDetailLoadingPage } from "~/app/[tenant]/(protected)/students/[id]/page-skeleton";
import { RoomDetailLoadingPage } from "~/app/[tenant]/(protected)/rooms/[id]/page-skeleton";
import type { ListPreview } from "./list-preview";

vi.unmock("swr");
vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => ({ enabled: false }),
}));
const owner = vi.hoisted(() => ({ id: "account-a", school: "school-a" }));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: owner.id, scope: "tenant", tenantId: 42 } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => owner.school,
  useTenantRoutingModeSafe: () => "path",
}));

beforeEach(() => {
  resetSWRScope({ revalidate: false });
  resumeSWRSession();
  Object.assign(owner, { id: "account-a", school: "school-a" });
});

const cases = [
  {
    label: "child search",
    source: "search-students-gall-filter",
    factory: studentKeys,
    target: studentKeys.detail("7"),
    data: {
      students: [
        {
          id: "7",
          name: "Mila Muster",
          school_class: "2a",
          group_name: "Eulen",
        },
      ],
    },
    expected: { id: "7", name: "Mila Muster", context: "2a · Eulen" },
  },
  {
    label: "OGS group",
    source: "ogs-students-3",
    factory: studentKeys,
    target: studentKeys.detail("7"),
    data: {
      students: [
        {
          id: "7",
          first_name: "Mila",
          last_name: "Muster",
          school_class: "2a",
        },
      ],
    },
    expected: { id: "7", name: "Mila Muster", context: "2a" },
  },
  {
    label: "room children",
    source: "room-students-3",
    factory: studentKeys,
    target: studentKeys.detail("7"),
    data: { students: [{ id: "7", name: "Mila Muster", school_class: "2a" }] },
    expected: { id: "7", name: "Mila Muster", context: "2a" },
  },
  {
    label: "absences",
    source: "student-status-days-overview-range",
    factory: studentKeys,
    target: studentKeys.detail("7"),
    data: {
      entries: [
        {
          id: "999",
          student_id: "7",
          first_name: "Mila",
          last_name: "Muster",
          school_class: "2a",
        },
      ],
    },
    expected: { id: "7", name: "Mila Muster", context: "2a" },
  },
  {
    label: "statistics",
    source: studentKeys.statisticsReport({ from: "2026-09-01" }),
    factory: studentKeys,
    target: studentKeys.detail("7"),
    data: {
      students: [
        {
          student_id: "7",
          first_name: "Mila",
          last_name: "Muster",
          school_class: "2a",
        },
      ],
    },
    expected: { id: "7", name: "Mila Muster", context: "2a" },
  },
  {
    label: "room",
    source: roomKeys.key({ includeSystem: true }),
    factory: roomKeys,
    target: roomKeys.detailKey("7"),
    data: [{ id: "7", name: "Aula", building: "Haus A" }],
    expected: { id: "7", name: "Aula", context: "Haus A" },
  },
  {
    label: "staff",
    source: staffKeys.key(),
    factory: staffKeys,
    target: staffKeys.recordKey("7"),
    data: [{ id: "7", name: "Mila Muster", role: "Betreuung" }],
    expected: { id: "7", name: "Mila Muster", context: "Betreuung" },
  },
  {
    label: "staff database",
    source: staffKeys.teachersKey(),
    factory: staffKeys,
    target: staffKeys.recordKey("7"),
    data: [{ id: "7", name: "Mila Muster", role: "Betreuung" }],
    expected: { id: "7", name: "Mila Muster", context: "Betreuung" },
  },
];

function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe("list to detail identity previews", () => {
  it.each(cases)(
    "shows $label synchronously, then completes from the detail response",
    async ({ source, factory, target, data, expected }) => {
      const response = deferred<{ name: string; detailOnly: string }>();
      const fetcher = vi.fn(() => response.promise);
      function List() {
        const { data: rows } = useSWRAuth(source, async () => data);
        return <p>{rows ? "Liste geladen" : "Liste lädt"}</p>;
      }
      function Detail() {
        const detail = useSWRAuth(target, fetcher, { keepPreviousData: false });
        const seedFrom = useSWRSeedFrom();
        const preview = seedFrom(
          factory.previewSources,
          target,
          (rows: unknown) => factory.preview(rows, "7"),
        );
        if (!detail.data && factory === studentKeys)
          return <StudentDetailLoadingPage preview={preview} />;
        if (!detail.data && factory === roomKeys)
          return <RoomDetailLoadingPage preview={preview} />;
        return (
          <>
            <h1>{detail.data?.name ?? preview?.name ?? "Lädt"}</h1>
            <p>
              {detail.data ? detail.data.detailOnly : "Details werden geladen"}
            </p>
            <span>{preview?.context}</span>
          </>
        );
      }
      const cache = new Map();
      const list = render(
        <SWRConfig value={{ provider: () => cache }}>
          <List />
        </SWRConfig>,
      );
      await screen.findByText("Liste geladen");
      list.unmount();
      render(
        <SWRConfig value={{ provider: () => cache }}>
          <Detail />
        </SWRConfig>,
      );
      expect(
        screen.getByRole("heading", { name: expected.name }),
      ).toBeInTheDocument();
      expect(screen.getByText(expected.context)).toBeInTheDocument();
      if (factory === studentKeys)
        expect(
          screen.getByRole("status", { name: "Kind wird geladen" }),
        ).toBeInTheDocument();
      else if (factory === roomKeys)
        expect(
          screen.getByRole("status", { name: "Raumdetails werden geladen" }),
        ).toBeInTheDocument();
      else
        expect(screen.getByText("Details werden geladen")).toBeInTheDocument();
      expect(fetcher).toHaveBeenCalledTimes(1);
      await act(async () =>
        response.resolve({
          name: "Bestätigter Name",
          detailOnly: "Bestätigte Details",
        }),
      );
      expect(
        await screen.findByRole("heading", { name: "Bestätigter Name" }),
      ).toBeInTheDocument();
      expect(screen.getByText("Bestätigte Details")).toBeInTheDocument();
    },
  );

  it.each(cases)(
    "projects only allowed identity fields for $label",
    ({ data, factory, expected }) => {
      const addSecrets = (row: object) => ({
        ...row,
        has_full_access: true,
        has_write_access: true,
        health_info: "private",
        address: "private",
        email: "private",
        staffNotes: "private",
      });
      const enriched = Array.isArray(data)
        ? data.map(addSecrets)
        : "entries" in data
          ? { entries: data.entries!.map(addSecrets) }
          : { students: data.students.map(addSecrets) };
      const preview = factory.preview(enriched, "7");
      expect(preview).toEqual(expected);
      expect(factory.preview(data, "other-child")).toBeUndefined();
      // The preview type cannot authorize or stand in for a detail record.
      const typed: ListPreview = preview!;
      // @ts-expect-error Access flags are never part of a list preview.
      expect(typed.has_full_access).toBeUndefined();
    },
  );

  it.each(["account", "school", "generation", "target"])(
    "withdraws a preview after a %s change, even when the detail request is pending",
    async (change) => {
      const source = roomKeys.key();
      const target = roomKeys.detailKey("7");
      const pending = deferred<{ name: string }>();
      let clear!: ReturnType<typeof useTenantMutateMatching>;
      function Detail({ list = false }: { list?: boolean }) {
        const rows = useSWRAuth(list ? source : null, async () => [
          { id: "7", name: "Aula" },
        ]);
        const detail = useSWRAuth(list ? null : target, () => pending.promise, {
          keepPreviousData: false,
        });
        const seedFrom = useSWRSeedFrom();
        clear = useTenantMutateMatching([target]);
        const preview = seedFrom(
          roomKeys.previewSources,
          target,
          (data: unknown) => roomKeys.preview(data, "7"),
        );
        return (
          <>
            <h1>{detail.data?.name ?? preview?.name ?? "Lädt"}</h1>
            <p>{list && rows.data ? "Liste geladen" : "Detailseite"}</p>
          </>
        );
      }
      const cache = new Map();
      const ui = (list: boolean) => (
        <SWRConfig value={{ provider: () => cache }}>
          <Detail list={list} />
        </SWRConfig>
      );
      const view = render(ui(true));
      await screen.findByText("Liste geladen");
      view.rerender(ui(false));
      expect(screen.getByRole("heading", { name: "Aula" })).toBeInTheDocument();
      await act(async () => {
        if (change === "account") owner.id = "account-b";
        if (change === "school") owner.school = "school-b";
        if (change === "generation") resetSWRScope({ revalidate: false });
        if (change === "target") await clear({ clear: true });
      });
      view.rerender(ui(false));
      // Anker: die neue Detailseite ist sichtbar, ohne Identität des alten Scopes.
      expect(screen.getByRole("heading", { name: "Lädt" })).toBeInTheDocument();
      expect(
        screen.queryByRole("heading", { name: "Aula" }),
      ).not.toBeInTheDocument();
    },
  );

  it("keeps the direct-call loading state without a list", () => {
    function Detail() {
      useSWRAuth(roomKeys.detailKey("7"), () => new Promise<never>(() => {}));
      const seedFrom = useSWRSeedFrom();
      const preview = seedFrom(
        roomKeys.previewSources,
        roomKeys.detailKey("7"),
        (data: unknown) => roomKeys.preview(data, "7"),
      );
      return <h1>{preview?.name ?? "Raum wird geladen"}</h1>;
    }
    render(
      <SWRConfig value={{ provider: () => new Map() }}>
        <Detail />
      </SWRConfig>,
    );
    expect(
      screen.getByRole("heading", { name: "Raum wird geladen" }),
    ).toBeInTheDocument();
  });
});
