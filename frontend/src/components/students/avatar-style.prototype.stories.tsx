// PROTOTYPE for #3762, throwaway. Lives only on branch prototype/3762-avatar-styles.
// Question: which generated avatar style makes the marketing school look credible?
// Five candidate styles, one Storybook story each, all rendered through the real
// StudentCard with ~70 % of children getting a generated image and the rest
// falling back to initials, as planned for the `marketing` seed profile.
// Run: `cd frontend && pnpm storybook`, then open "PROTOTYPE/Avatar-Stil".

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { renderToStaticMarkup } from "react-dom/server";
import BoringAvatar from "boring-avatars";
import { createAvatar } from "@dicebear/core";
import {
  glass,
  notionists,
  openPeeps,
  thumbs,
  bigSmile,
  shapes,
} from "@dicebear/collection";
import { renderAvatar, selectAvatar } from "@usenavii/core";

import { StudentCard, SchoolClassIcon, StudentInfoRow } from "./student-card";
import { LocationBadge } from "~/components/ui/location-badge";

// moto brand colours from styles/globals.css (green, blue, orange, teal, amber).
const MOTO = ["#83cd2d", "#5080d8", "#f78c10", "#159e90", "#eab308"];
const MOTO_HEX = MOTO.map((c) => c.slice(1));
// Soft brand tints (moto-*-soft) as background behind character styles.
const MOTO_SOFT = ["eef9e1", "edf3fc", "fff3e5", "e8f8f5", "fef3c7"];

type Style = (seed: string) => string;

const boring =
  (variant: "bauhaus" | "marble" | "sunset"): Style =>
  (seed) =>
    `data:image/svg+xml;utf8,${encodeURIComponent(
      renderToStaticMarkup(
        <BoringAvatar name={seed} variant={variant} colors={MOTO} square />,
      ),
    )}`;

const STYLES: Record<string, Style> = {
  "Boring bauhaus": boring("bauhaus"),
  "Boring marble": boring("marble"),
  "Boring sunset": boring("sunset"),
  "DiceBear shapes": (seed) =>
    createAvatar(shapes, {
      seed,
      backgroundColor: MOTO_HEX,
      shape1Color: MOTO_HEX,
      shape2Color: MOTO_HEX,
      shape3Color: MOTO_HEX,
    }).toDataUri(),
  "DiceBear glass": (seed) =>
    createAvatar(glass, { seed, backgroundColor: MOTO_HEX }).toDataUri(),
  // Concrete styles found via X (round 2, "zu abstrakt").
  "DiceBear notionists": (seed) =>
    createAvatar(notionists, { seed, backgroundColor: MOTO_SOFT }).toDataUri(),
  "DiceBear open-peeps": (seed) =>
    createAvatar(openPeeps, { seed, backgroundColor: MOTO_SOFT }).toDataUri(),
  "DiceBear big-smile": (seed) =>
    createAvatar(bigSmile, { seed, backgroundColor: MOTO_SOFT }).toDataUri(),
  "DiceBear thumbs": (seed) =>
    createAvatar(thumbs, { seed, backgroundColor: MOTO_HEX }).toDataUri(),
  "Navii Maskottchen": (seed) =>
    `data:image/svg+xml;utf8,${encodeURIComponent(
      renderAvatar(selectAvatar(seed), { tileBg: "auto" }),
    )}`,
};

const CHILDREN: [string, string, string, string][] = [
  ["Mia", "Schmidt", "1a", "Anwesend"],
  ["Ben", "Müller", "1a", "Schulhof"],
  ["Emilia", "Weber", "1b", "Anwesend"],
  ["Noah", "Fischer", "2a", "Zuhause"],
  ["Hannah", "Wagner", "2a", "Anwesend"],
  ["Leon", "Becker", "2b", "Anwesend"],
  ["Lina", "Hoffmann", "3a", "Schulhof"],
  ["Elias", "Schäfer", "3a", "Anwesend"],
  ["Emma", "Koch", "3b", "Zuhause"],
  ["Finn", "Bauer", "3b", "Anwesend"],
  ["Sophia", "Richter", "4a", "Anwesend"],
  ["Paul", "Klein", "4a", "Schulhof"],
  ["Ella", "Wolf", "4b", "Anwesend"],
  ["Luis", "Schröder", "1b", "Anwesend"],
  ["Clara", "Neumann", "2b", "Zuhause"],
  ["Felix", "Schwarz", "3a", "Anwesend"],
  ["Leni", "Zimmermann", "4b", "Anwesend"],
  ["Jonas", "Braun", "1a", "Schulhof"],
  ["Mila", "Krüger", "2a", "Anwesend"],
  ["Anton", "Hofmann", "4a", "Anwesend"],
];

// Deterministic 70 % photo quota: indexes ending in 2, 5, 8 show initials (14/20 with image).
const hasPhoto = (i: number) => i % 10 !== 2 && i % 10 !== 5 && i % 10 !== 8;

function Grid({ style }: { style: string }) {
  const make = STYLES[style];
  if (!make) return null;
  const withPhoto = CHILDREN.filter((_, i) => hasPhoto(i)).length;
  return (
    <div className="min-h-screen bg-gray-50 p-6">
      <p className="mb-4 font-mono text-xs text-gray-500">
        Stil: {style} · {withPhoto}/{CHILDREN.length} Kinder mit Bild · Rest
        Initialen · Seed = Vor- und Nachname
      </p>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(18rem,1fr))] gap-4">
        {CHILDREN.map(([first, last, klasse, location], i) => (
          <StudentCard
            key={`${first}-${last}`}
            studentId={String(i)}
            firstName={first}
            lastName={last}
            photoUrl={hasPhoto(i) ? make(`${first} ${last}`) : null}
            onClick={() => undefined}
            locationBadge={
              <LocationBadge
                student={{ current_location: location }}
                displayMode="roomName"
              />
            }
            extraContent={
              <StudentInfoRow icon={<SchoolClassIcon />}>
                Klasse {klasse}
              </StudentInfoRow>
            }
          />
        ))}
      </div>
    </div>
  );
}

const meta = {
  title: "PROTOTYPE/Avatar-Stil",
  component: Grid,
  parameters: { layout: "fullscreen" },
  argTypes: {
    style: { control: "select", options: Object.keys(STYLES) },
  },
} satisfies Meta<typeof Grid>;

export default meta;

type Story = StoryObj<typeof meta>;

export const BoringBauhaus: Story = { args: { style: "Boring bauhaus" } };
export const BoringMarble: Story = { args: { style: "Boring marble" } };
export const BoringSunset: Story = { args: { style: "Boring sunset" } };
export const DiceBearShapes: Story = { args: { style: "DiceBear shapes" } };
export const DiceBearGlass: Story = { args: { style: "DiceBear glass" } };
export const DiceBearNotionists: Story = {
  args: { style: "DiceBear notionists" },
};
export const DiceBearOpenPeeps: Story = {
  args: { style: "DiceBear open-peeps" },
};
export const DiceBearBigSmile: Story = {
  args: { style: "DiceBear big-smile" },
};
export const DiceBearThumbs: Story = { args: { style: "DiceBear thumbs" } };
export const NaviiMaskottchen: Story = {
  args: { style: "Navii Maskottchen" },
};
