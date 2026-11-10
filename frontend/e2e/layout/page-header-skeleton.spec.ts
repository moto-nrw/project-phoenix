import { expect, test, type Page } from "@playwright/test";
import { createElement, type ComponentProps, type ComponentType } from "react";
import { renderToStaticMarkup } from "react-dom/server";
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
