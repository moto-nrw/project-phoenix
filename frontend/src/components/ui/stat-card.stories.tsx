import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { UsersThreeIcon } from "@phosphor-icons/react";
import { expect, userEvent, within } from "storybook/test";

import { Button } from "./button";
import { StatCard } from "./stat-card";

const meta = {
  title: "components/ui/StatCard",
} satisfies Meta;

export default meta;
type Story = StoryObj;

const TONES = ["blue", "green", "orange", "red", "gray", "neutral"] as const;

// Figure in the accessible shade of its tone, bar in the raw tone.
export const AllTones: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      {TONES.map((tone) => (
        <StatCard
          key={tone}
          label={tone}
          value={42}
          hint="von 60"
          progressPct={70}
          tone={tone}
        />
      ))}
    </div>
  ),
};

export const Tiles: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      {TONES.map((tone) => (
        <StatCard
          key={tone}
          variant="tile"
          label={tone}
          value={12}
          tone={tone}
        />
      ))}
    </div>
  ),
};

const GROUP_ICON = <UsersThreeIcon weight="duotone" aria-hidden="true" />;

/** Verlinkte Kachel mit Symbol, wie auf der Startseite. */
export const Linked: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      <StatCard
        label="Kinder anwesend"
        value={86}
        tone="neutral"
        icon={GROUP_ICON}
        href="/students"
      />
      <StatCard label="Krank" value={4} tone="red" href="/students?s=sick" />
      <StatCard label="Ohne Link" value={12} hint="von 20" />
    </div>
  ),
};

/** Tastaturfokus auf der verlinkten Kachel (#6565). */
export const LinkedFocused: Story = {
  ...Linked,
  play: async ({ canvasElement }) => {
    await userEvent.tab();
    await expect(
      within(canvasElement).getByRole("link", { name: /Kinder anwesend/ }),
    ).toHaveFocus();
  },
};

export const LinkedLoading: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      <StatCard label="Kinder anwesend" value={86} href="/students" loading />
      <StatCard
        label="Saldo"
        value="+12h 30min"
        compactValue
        title="Summe aller Salden"
        action={
          <Button type="button" variant="ghost" size="icon" aria-label="Info">
            i
          </Button>
        }
      />
    </div>
  ),
};

/** Dieselbe verlinkte Kennzahl als Listenzeile. */
export const Rows: Story = {
  render: () => (
    <div className="w-80 space-y-2">
      <StatCard
        variant="row"
        label="Kinder anwesend"
        value={86}
        tone="green"
        icon={GROUP_ICON}
        href="/students"
      />
      <StatCard
        variant="row"
        label="Krank"
        value={4}
        tone="red"
        href="/students?s=sick"
        loading
      />
    </div>
  ),
};

export const RowFocused: Story = {
  ...Rows,
  play: async ({ canvasElement }) => {
    await userEvent.tab();
    await expect(
      within(canvasElement).getByRole("link", { name: /Kinder anwesend/ }),
    ).toHaveFocus();
  },
};

export const Inline: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-2 gap-3">
      <StatCard
        variant="inline"
        label="Plätze belegt"
        value={18}
        detail="2 frei"
        icon={GROUP_ICON}
      />
      <StatCard variant="inline" label="Anmeldungen" value={7} />
    </div>
  ),
};
