import { expect, test, type Page } from "@playwright/test";
import {
  createElement,
  type ComponentProps,
  type ComponentType,
  type ReactNode,
} from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { ParentPageSkeleton } from "../../src/components/parent/parent-page";
import type { TenantPageHeaderSkeleton } from "../../src/components/ui/page-skeletons";
import type { TenantPage } from "../../src/components/ui/tenant-page";
import { compileGlobalsCss, loadSourceModules, render } from "./harness";

// Der Skeleton-Kopf steht während des Ladens dort, wo danach der Kopf von
// `TenantPage` steht (#6568). Ist er niedriger oder höher, springt die Seite
// beim Wechsel. Beide Köpfe kommen aus dem Produktionscode.

// Nur die Typen kommen über `import type`; zur Laufzeit lädt Vite die Module.
type HeadProps = ComponentProps<typeof TenantPage>;
type SkeletonProps = NonNullable<
  ComponentProps<typeof TenantPageHeaderSkeleton>
>;

let compiledCss = "";
let LoadedHead: ComponentType<HeadProps>;
let SkeletonHead: ComponentType<SkeletonProps>;
let ParentSkeleton: ComponentType<ComponentProps<typeof ParentPageSkeleton>>;
let IntlProvider: ComponentType<{
  locale: string;
  timeZone: string;
  messages: Record<string, unknown>;
  children?: ReactNode;
}>;

// Beide Tests in einem Worker: Er lädt die Quellen nur einmal über Vite.
test.describe.configure({ mode: "default" });

test.beforeAll(async () => {
  compiledCss = await compileGlobalsCss();
  await loadSourceModules(async (importSource) => {
    const page = await importSource("/src/components/ui/tenant-page.tsx");
    const skeletons = await importSource(
      "/src/components/ui/page-skeletons.tsx",
    );
    LoadedHead = page.TenantPage as ComponentType<HeadProps>;
    SkeletonHead =
      skeletons.TenantPageHeaderSkeleton as ComponentType<SkeletonProps>;
    const parentPage = await importSource(
      "/src/components/parent/parent-page.tsx",
    );
    ParentSkeleton = parentPage.ParentPageSkeleton as typeof ParentSkeleton;
    // Über denselben Modulgraphen wie `parent-page.tsx`, sonst fände
    // `useTranslations` den Kontext des Providers nicht.
    const intl = await importSource("next-intl");
    IntlProvider = intl.NextIntlClientProvider as typeof IntlProvider;
  });
});

// Desktop, die Dichte-Stufe `compact` (Tablet) und ein Telefon.
const VIEWPORTS = [
  { name: "desktop", width: 1280, height: 1000 },
  { name: "compact", width: 900, height: 800 },
  { name: "phone", width: 390, height: 844 },
] as const;

const search = { value: "", onChange: () => undefined, placeholder: "Suchen" };

const CASES: readonly {
  name: string;
  skeleton: SkeletonProps;
  page: Omit<HeadProps, "title">;
}[] = [
  {
    name: "title and status line",
    skeleton: { stats: true },
    page: { stats: "116 Kinder · 107 zuhause" },
  },
  { name: "title only", skeleton: { stats: false }, page: {} },
  {
    name: "info button",
    skeleton: { stats: true, info: true },
    page: { stats: "116 Kinder", info: "Erklärung" },
  },
  {
    name: "search row",
    skeleton: { stats: true, controls: true },
    page: { stats: "116 Kinder", search },
  },
];

async function headHeight(page: Page, markup: string): Promise<number> {
  await render(page, compiledCss, `<div style="padding:16px">${markup}</div>`);
  const head = await page.locator("header").first().boundingBox();
  if (!head) throw new Error("head is not rendered");
  return head.height;
}

// Ein Test für alle Fälle: Jeder Worker lädt die Quellen einmal über Vite, das
// kostet mehr als alle Messungen zusammen. `expect.soft` meldet trotzdem jeden
// abweichenden Fall einzeln.
test("the skeleton head has the height of the loaded head", async ({
  page,
}) => {
  for (const viewport of VIEWPORTS) {
    await page.setViewportSize(viewport);
    for (const testCase of CASES) {
      const skeleton = await headHeight(
        page,
        renderToStaticMarkup(createElement(SkeletonHead, testCase.skeleton)),
      );
      const loaded = await headHeight(
        page,
        renderToStaticMarkup(
          createElement(LoadedHead, { title: "Kinder", ...testCase.page }),
        ),
      );
      expect.soft(skeleton, `${testCase.name}, ${viewport.name}`).toBe(loaded);
    }
  }
});

// Das Elternportal lädt mit seinem eigenen Kopf (`ParentPageHeader`), nicht mit
// dem von `TenantPage`. Sein Ladezustand behält bis #6613 die Maße von vor
// #6568: 20 px Rand auf jedem Bildschirm, ein 28-px-Titelbalken und 8 px
// darunter ein 16-px-Statusbalken.
test("the parents portal keeps its loading head until #6613", async ({
  page,
}) => {
  const markup = renderToStaticMarkup(
    createElement(
      IntlProvider,
      {
        locale: "de",
        timeZone: "Europe/Berlin",
        messages: { parentNav: { sectionLoading: "Inhalt wird geladen" } },
      },
      createElement(ParentSkeleton, { rows: 1 }),
    ),
  );
  for (const viewport of VIEWPORTS) {
    await page.setViewportSize(viewport);
    await render(
      page,
      compiledCss,
      `<div style="padding:16px">${markup}</div>`,
    );
    const head = page.locator("header").first();
    const geometry = await head.evaluate((header) => {
      const outer = header.getBoundingClientRect();
      return {
        height: outer.height,
        bars: Array.from(header.querySelectorAll(".animate-pulse"), (bar) => {
          const box = bar.getBoundingClientRect();
          return {
            top: box.top - outer.top,
            left: box.left - outer.left,
            width: box.width,
            height: box.height,
          };
        }),
      };
    });
    expect.soft(geometry, viewport.name).toEqual({
      height: 94,
      bars: [
        { top: 21, left: 21, width: 208, height: 28 },
        { top: 57, left: 21, width: 256, height: 16 },
      ],
    });
  }
});
