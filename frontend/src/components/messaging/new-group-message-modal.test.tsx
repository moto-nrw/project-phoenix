vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "account-1", tenantId: 1, scope: "tenant" },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { GroupChild } from "~/lib/parent-group-threads-api";
import type { ReactNode } from "react";
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

// The modal loads its lists and the audience through real SWR.
vi.unmock("swr");

const mocks = vi.hoisted(() => ({
  getGroups: vi.fn(),
  getSchoolClasses: vi.fn(),
  fetchActivities: vi.fn(),
  fetchGroupAudience: vi.fn(),
  createGroupThread: vi.fn(),
}));

vi.mock("~/lib/api", () => ({
  groupService: { getGroups: mocks.getGroups },
  studentService: { getSchoolClasses: mocks.getSchoolClasses },
}));
vi.mock("~/lib/activity-api", () => ({
  fetchActivities: mocks.fetchActivities,
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "schule",
}));
vi.mock("~/lib/parent-group-threads-api", () => ({
  fetchGroupAudience: mocks.fetchGroupAudience,
  createGroupThread: mocks.createGroupThread,
}));

import { NewGroupMessageModal } from "./new-group-message-modal";

function wrapper({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>{children}</ToastProvider>
    </SWRConfig>
  );
}

async function pickGroup(name: string) {
  const trigger = await screen.findByRole("combobox", { name: "An" });
  await waitFor(() => expect(trigger).not.toBeDisabled());
  fireEvent.click(trigger);
  fireEvent.click(await screen.findByRole("option", { name }));
}

describe("Neue Gruppennachricht", () => {
  beforeEach(() => {
    mocks.getGroups.mockResolvedValue([
      { id: "5", name: "Bären" },
      { id: "6", name: "Füchse" },
    ]);
    mocks.getSchoolClasses.mockResolvedValue(["1a", "2b"]);
    mocks.fetchActivities.mockResolvedValue([]);
    mocks.fetchGroupAudience.mockResolvedValue([
      {
        student_id: "1",
        first_name: "Felix",
        last_name: "Schneider",
        reachable: true,
        read: false,
      },
      {
        student_id: "2",
        first_name: "Mia",
        last_name: "Ohne",
        school_class: "1a",
        reachable: false,
        read: false,
      },
    ]);
    mocks.createGroupThread.mockReset();
  });

  it("shows whom the message reaches and who has no parent access", async () => {
    render(<NewGroupMessageModal onClose={vi.fn()} onCreated={vi.fn()} />, {
      wrapper,
    });
    await pickGroup("Bären");

    const audience = await screen.findByTestId("group-audience");
    expect(
      within(audience).getByText("1 Kind hat Eltern mit Zugang zu moto."),
    ).toBeInTheDocument();
    expect(within(audience).getByText(/Mia Ohne/)).toBeInTheDocument();
    expect(
      within(audience).getByText("Kein Eltern-Zugang"),
    ).toBeInTheDocument();
    expect(mocks.fetchGroupAudience).toHaveBeenCalledWith("group", "5");
  });

  it("does not carry the previous group audience into a newly selected group", async () => {
    render(<NewGroupMessageModal onClose={vi.fn()} onCreated={vi.fn()} />, {
      wrapper,
    });
    await pickGroup("Bären");
    await screen.findByText(/Mia Ohne/);
    let release!: (children: GroupChild[]) => void;
    mocks.fetchGroupAudience.mockReturnValueOnce(
      new Promise<GroupChild[]>((resolve) => {
        release = resolve;
      }),
    );
    await pickGroup("Füchse");
    expect(
      await screen.findByText("Empfänger werden geladen…"),
    ).toBeInTheDocument();
    await act(async () =>
      release([
        {
          student_id: "3",
          first_name: "Lina",
          last_name: "Fuchs",
          reachable: false,
          read: false,
        },
      ]),
    );
    expect(await screen.findByText(/Lina Fuchs/)).toBeInTheDocument();
  });

  it("sends the message to the chosen group and hands over the new thread", async () => {
    const detail = { thread: { id: "9" }, messages: [], children: [] };
    mocks.createGroupThread.mockResolvedValue(detail);
    const onCreated = vi.fn();
    render(<NewGroupMessageModal onClose={vi.fn()} onCreated={onCreated} />, {
      wrapper,
    });

    await pickGroup("Füchse");
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Ausflug" },
    });
    fireEvent.change(screen.getByLabelText("Nachricht"), {
      target: { value: "Am Freitag geht es in den Zoo." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(detail));
    expect(mocks.createGroupThread).toHaveBeenCalledWith({
      client_message_id: expect.any(String),
      target: { type: "group", id: "6", label: "Gruppe Füchse" },
      title: "Ausflug",
      body: "Am Freitag geht es in den Zoo.",
    });
  });

  it("names the missing fields instead of sending an empty message", async () => {
    render(<NewGroupMessageModal onClose={vi.fn()} onCreated={vi.fn()} />, {
      wrapper,
    });
    await screen.findByRole("combobox", { name: "An" });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText("Bitte füllen Sie alle Felder aus."),
    ).toBeInTheDocument();
    expect(mocks.createGroupThread).not.toHaveBeenCalled();
  });

  it("switches to classes and loads the class list", async () => {
    render(<NewGroupMessageModal onClose={vi.fn()} onCreated={vi.fn()} />, {
      wrapper,
    });
    fireEvent.click(screen.getByRole("button", { name: "Klasse" }));
    await pickGroup("2b");
    await waitFor(() =>
      expect(mocks.fetchGroupAudience).toHaveBeenCalledWith("class", "2b"),
    );
  });
});
