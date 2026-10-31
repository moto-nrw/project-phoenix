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

for (const [initialMonth, target] of [
  ["2026-11", "2026-10-30"],
  ["2026-01", "2025-12-31"],
  ["2026-10", "2026-12-01"],
] as const) {
  test(`date selection reaches ${target} from ${initialMonth}`, async ({
    page,
  }) => {
    await page.setContent(`
      <button aria-label="Startdatum" aria-expanded="false">Datum wählen</button>
      <div id="calendar"></div>
      <script>
        const trigger = document.querySelector('button');
        const calendar = document.querySelector('#calendar');
        const initial = ${JSON.stringify(initialMonth)};
        const target = ${JSON.stringify(target)};
        let monthIndex = Number(initial.slice(0, 4)) * 12 + Number(initial.slice(5)) - 1;
        function month(index) {
          return String(Math.floor(index / 12)).padStart(4, '0') + '-' + String(index % 12 + 1).padStart(2, '0');
        }
        function draw() {
          const current = month(monthIndex);
          const first = current + '-01';
          const extra = target.startsWith(current) && target !== first
            ? '<td data-day="' + target + '"><button>Target</button></td>' : '';
          calendar.innerHTML = '<button aria-label="Vorheriger Monat">Zurück</button>'
            + '<button aria-label="Nächster Monat">Weiter</button>'
            + '<table><tbody><tr><td data-day="' + month(monthIndex - 1) + '-28" data-outside><button>Outside</button></td>'
            + '<td data-day="' + first + '"><button>1</button></td>' + extra + '</tr></tbody></table>';
          calendar.querySelector('[aria-label="Vorheriger Monat"]').onclick = () => { monthIndex--; draw(); };
          calendar.querySelector('[aria-label="Nächster Monat"]').onclick = () => { monthIndex++; draw(); };
          const day = calendar.querySelector('td[data-day="' + target + '"]:not([data-outside]) button');
          if (day) day.onclick = () => {
            trigger.textContent = target.split('-').reverse().join('.');
            trigger.setAttribute('aria-expanded', 'false');
            calendar.innerHTML = '';
          };
        }
        trigger.onclick = () => { trigger.setAttribute('aria-expanded', 'true'); draw(); };
      </script>
    `);
    const trigger = page.getByRole("button", { name: "Startdatum" });
    await pickDate(page, trigger, target);
    await expect(trigger).toHaveText(target.split("-").reverse().join("."));
    await expect(trigger).toHaveAttribute("aria-expanded", "false");
  });
}
