import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { ApiError } from "~/lib/api-error";
import {
  getChildFeatures,
  getGroupThread,
  postChildMessage,
} from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";
import { ParentGroupThreadView } from "./group-thread-view";

vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("~/lib/parent-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-api")>()),
  getGroupThread: vi.fn(),
  getChildFeatures: vi.fn(),
  postChildMessage: vi.fn(),
}));

const mockedThread = vi.mocked(getGroupThread);
const mockedFeatures = vi.mocked(getChildFeatures);
const mockedPost = vi.mocked(postChildMessage);

const felix = { student_id: "42", first_name: "Felix", last_name: "Schneider" };
const mia = { student_id: "43", first_name: "Mia", last_name: "Schneider" };

const sommerfest = {
  id: "9",
  title: "Sommerfest am Freitag",
  target_label: "Gruppe Sonnenblumen",
  school_name: "Schule am Berg",
  counterpart_name: "OGS Schule am Berg",
  closed: false,
  last_message_at: "2026-09-09T08:00:00Z",
  last_message_body: "Bitte an die Kopfbedeckung denken.",
  unread: 0,
  children: [felix],
  messages: [
    {
      id: "1",
      body: "Wer kann einen Kuchen mitbringen?",
      created_at: "2026-09-09T07:00:00Z",
    },
    {
      id: "2",
      body: "Bitte an die Kopfbedeckung denken.",
      created_at: "2026-09-09T08:00:00Z",
    },
  ],
};

function features(notesEnabled: boolean) {
  return { notes_enabled: notesEnabled } as unknown as Awaited<
    ReturnType<typeof getChildFeatures>
  >;
}

function renderView() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ParentGroupThreadView groupThreadId="9" />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedFeatures.mockResolvedValue(features(true));
});

describe("ParentGroupThreadView (#4784)", () => {
  it("zeigt die Nachrichten der OGS und sagt, wer die Antwort sieht", async () => {
    mockedThread.mockResolvedValue(sommerfest);

    renderView();

    expect(
      await screen.findByRole("heading", {
        level: 1,
        name: "Sommerfest am Freitag",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Gruppe Sonnenblumen")).toBeInTheDocument();
    expect(
      screen.getByText("Wer kann einen Kuchen mitbringen?"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Ihre Antwort geht nur an die OGS. Andere Eltern sehen sie nicht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zurück zu Nachrichten" }),
    ).toHaveAttribute("href", "/parents/messages");
  });

  it("schickt die Antwort fuer das gewaehlte Kind an die OGS", async () => {
    mockedThread.mockResolvedValue({ ...sommerfest, children: [felix, mia] });
    mockedPost.mockResolvedValue(
      {} as Awaited<ReturnType<typeof postChildMessage>>,
    );

    renderView();

    fireEvent.click(
      await screen.findByRole("button", { name: "Mia Schneider" }),
    );
    fireEvent.change(screen.getByLabelText("Antwort"), {
      target: { value: "Wir bringen Muffins." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    await waitFor(() =>
      expect(mockedPost).toHaveBeenCalledWith(
        "43",
        "Wir bringen Muffins.",
        "9",
      ),
    );
    expect(
      await screen.findByText("Ihre Antwort ist bei der OGS."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zum Chat mit der OGS" }),
    ).toHaveAttribute("href", "/parents/messages/43");
  });

  it("bietet bei einer geschlossenen Gruppennachricht nur den Chat an", async () => {
    mockedThread.mockResolvedValue({ ...sommerfest, closed: true });

    renderView();

    expect(
      await screen.findByText(
        "Diese Gruppennachricht ist geschlossen. Sie können der OGS weiter im Chat zu Ihrem Kind schreiben.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Zum Chat mit der OGS" }),
    ).toHaveAttribute("href", "/parents/messages/42");
    expect(screen.queryByLabelText("Antwort")).not.toBeInTheDocument();
  });

  it("zeigt ohne Schreibrecht keinen Antwortweg", async () => {
    mockedThread.mockResolvedValue(sommerfest);
    mockedFeatures.mockResolvedValue(features(false));

    renderView();

    expect(
      await screen.findByText(
        "Sie können auf diese Nachricht nicht antworten. Lesen können Sie sie weiterhin.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Antwort")).not.toBeInTheDocument();
  });

  it("zeigt eine geschlossene Gruppennachricht beim Senden als Formularfehler", async () => {
    mockedThread.mockResolvedValue(sommerfest);
    mockedPost.mockRejectedValue(
      new ApiError("closed", 409, {
        code: "communication.group_thread_closed",
      }),
    );

    renderView();

    fireEvent.change(await screen.findByLabelText("Antwort"), {
      target: { value: "Hallo" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText(
        catalogText("communication.group_thread_closed", "die Antwort"),
      ),
    ).toBeInTheDocument();
  });
});
