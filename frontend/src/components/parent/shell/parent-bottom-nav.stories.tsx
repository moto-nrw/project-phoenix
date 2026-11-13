import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { ParentShellProvider } from "~/lib/shell-auth-context";
import { ParentBottomNav } from "./parent-bottom-nav";

const meta = {
  title: "components/parent/shell/ParentBottomNav",
  component: ParentBottomNav,
  parameters: {
    layout: "fullscreen",
    nextjs: {
      appDirectory: true,
      navigation: { pathname: "/parents/children", query: {} },
    },
  },
  decorators: [
    (Story) => (
      <ParentShellProvider>
        <div className="min-h-[640px] bg-gray-50">
          <Story />
        </div>
      </ParentShellProvider>
    ),
  ],
  args: {
    badges: { messages: 3, news: 2 },
    gates: { news: true, mealPlan: true },
    childCount: 2,
  },
} satisfies Meta<typeof ParentBottomNav>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Ein Alltagsziel ist aktiv, Zähler an Nachrichten und „Mehr“. */
export const Default: Story = {};

/** Die aktive Seite steht hinter „Mehr“: die Fläche liegt unter „Mehr“. */
export const ActiveBehindMore: Story = {
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: { pathname: "/parents/news", query: {} },
    },
  },
};

/** Das geöffnete „Mehr“-Blatt mit Zähler, Trennlinie und Abmelden. */
export const MoreOpened: Story = {
  play: async ({ canvasElement }) => {
    // Ab 1024 px blendet CSS die Leiste aus; dort gibt es nichts zu öffnen.
    const more = within(canvasElement).queryByRole("button", { name: "Mehr" });
    if (!more) return;
    await userEvent.click(more);
    await expect(await screen.findByRole("dialog")).toBeVisible();
  },
};
