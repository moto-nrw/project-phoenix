import { expect, test } from "@playwright/test";
import { selectOverflowAction } from "../overflow-menu";

// OverflowMenu closes on an outside scroll. Reproduce a delayed scroll after
// opening while Playwright waits for the menu entry to stop moving.
test("reopens an overflow menu closed before its action can be clicked", async ({
  page,
}) => {
  test.setTimeout(15000);
  await page.setContent(`
    <button aria-label="Aktionen" aria-expanded="false">⋮</button>
    <p role="status">Bereit</p>
    <style>
      @keyframes moving { from { transform: translateX(0); } to { transform: translateX(20px); } }
      .moving { animation: moving 50ms infinite alternate; }
    </style>
    <script>
      const trigger = document.querySelector('button');
      let first = true;
      trigger.onclick = () => {
        trigger.setAttribute('aria-expanded', 'true');
        const menu = document.createElement('div');
        menu.setAttribute('role', 'menu');
        menu.innerHTML = '<button role="menuitem">Krank melden</button>';
        const action = menu.querySelector('button');
        document.body.append(menu);
        const close = () => {
          menu.remove();
          trigger.setAttribute('aria-expanded', 'false');
          window.removeEventListener('scroll', close);
        };
        window.addEventListener('scroll', close);
        action.onclick = () => {
          document.querySelector('[role="status"]').textContent = 'Ausgewählt';
          close();
        };
        if (first) {
          first = false;
          action.className = 'moving';
          setTimeout(() => window.dispatchEvent(new Event('scroll')), 300);
        }
      };
    </script>
  `);
  const trigger = page.getByRole("button", { name: "Aktionen", exact: true });
  await selectOverflowAction(page, trigger, "Krank melden");
  await expect(page.getByRole("status")).toHaveText("Ausgewählt");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});
