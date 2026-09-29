import { defineConfig } from "@playwright/test";

// Produkt-Screenshots (#3759): fotografiert den laufenden lokalen Stack und
// rahmt die Bilder. Eigene Config, damit der Lauf nie Teil der normalen
// e2e-Suite ist (und umgekehrt); Vorbild ist playwright.guides.config.ts.
//
//   pnpm run generate:screenshots   alle Shots (SHOT_IDS=a,b schränkt ein)
//   pnpm run test:screenshots       Pipeline-Test mit einer Mini-Shot-Liste
//
// Es gibt keinen webServer: der Stack läuft schon (scripts/dev-native.sh up)
// und ist mit dem Profil `marketing` geseedet. Playwright dient hier als
// Runner, weil er TypeScript ohne eigenen Build lädt. Die Dateien heißen
// `.pw.ts`, damit vitest sie nicht aufgreift.
export default defineConfig({
  testDir: "./scripts/product-screenshots",
  testMatch: "*.pw.ts",
  // Dev-Server kompilieren Routen beim ersten Aufruf; ein Lauf fotografiert
  // mehrere Seiten auf mehreren Geräten.
  timeout: 900_000,
  workers: 1,
  retries: 0,
  reporter: "list",
  outputDir: "test-results/product-screenshots",
});
