import { expect, test } from "@playwright/test";
import { pickDate } from "../date-picker";

// A picker rerender can detach the day while Playwright waits for stability.
// Reproduce that lifecycle without a server, login, or seeded database.
test("date selection reopens a picker detached during the day click", async ({
  page,
}) => {
  test.setTimeout(15000);
  await page.setContent(`
    <button aria-label="Startdatum" aria-expanded="false">21.10.2026</button>
    <style>
      @keyframes moving { from { transform: translateX(0); } to { transform: translateX(20px); } }
      .moving { animation: moving 50ms infinite alternate; }
    </style>
    <script>
      const trigger = document.querySelector('button');
      let first = true;
      trigger.onclick = () => {
        trigger.setAttribute('aria-expanded', 'true');
        const calendar = document.createElement('table');
        calendar.innerHTML = '<tbody><tr><td data-day="2026-10-22"><button>22</button></td></tr></tbody>';
        const day = calendar.querySelector('button');
        document.body.append(calendar);
        if (first) {
          first = false;
          day.className = 'moving';
          setTimeout(() => {
            calendar.remove();
            trigger.setAttribute('aria-expanded', 'false');
          }, 500);
        } else {
          day.onclick = () => {
            trigger.textContent = '22.10.2026';
            trigger.setAttribute('aria-expanded', 'false');
            calendar.remove();
          };
        }
      };
    </script>
  `);
  const trigger = page.getByRole("button", { name: "Startdatum" });
  await pickDate(page, trigger, "2026-10-22");
  await expect(trigger).toHaveText("22.10.2026");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});

// A weekend's visible school day can be in the previous calendar month.
test("date selection navigates backward across a month boundary", async ({
  page,
}) => {
  await page.setContent(`
    <button aria-label="Startdatum" aria-expanded="false">01.11.2026</button>
    <div id="calendar" hidden>
      <button aria-label="Vorheriger Monat">Zurück</button>
      <button aria-label="Nächster Monat">Weiter</button>
      <table><tbody><tr><td data-day="2026-11-01"><button>1</button></td></tr></tbody></table>
    </div>
    <script>
      const trigger = document.querySelector('button');
      const calendar = document.querySelector('#calendar');
      trigger.onclick = () => {
        calendar.hidden = false;
        trigger.setAttribute('aria-expanded', 'true');
      };
      document.querySelector('[aria-label="Vorheriger Monat"]').onclick = () => {
        const day = calendar.querySelector('td');
        day.dataset.day = '2026-10-30';
        day.querySelector('button').textContent = '30';
      };
      document.querySelector('[aria-label="Nächster Monat"]').onclick = () => {
        calendar.querySelector('td').dataset.day = '2026-12-01';
      };
      calendar.querySelector('td button').onclick = () => {
        if (calendar.querySelector('td').dataset.day !== '2026-10-30') return;
        trigger.textContent = '30.10.2026';
        trigger.setAttribute('aria-expanded', 'false');
        calendar.hidden = true;
      };
    </script>
  `);
  const trigger = page.getByRole("button", { name: "Startdatum" });
  await pickDate(page, trigger, "2026-10-30");
  await expect(trigger).toHaveText("30.10.2026");
  await expect(trigger).toHaveAttribute("aria-expanded", "false");
});
