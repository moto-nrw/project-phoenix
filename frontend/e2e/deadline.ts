/**
 * Ein gemeinsames Zeitbudget für mehrere Wartestellen einer Prüfung. Jede
 * Stelle bekommt nur die Restzeit; ist sie aufgebraucht, scheitert die
 * Prüfung sofort. Ein Playwright-`timeout` von 0 hieße „ohne Grenze“, deshalb
 * liefert `remaining` nie 0.
 */
export class Deadline {
  private readonly end: number;

  constructor(budgetMs: number) {
    this.end = Date.now() + budgetMs;
  }

  /** Restzeit in Millisekunden für `what`; wirft, wenn keine bleibt. */
  remaining(what: string): number {
    const left = this.end - Date.now();
    if (left <= 0) throw new Error(`${what}: Zeitbudget überschritten.`);
    return left;
  }

  /** Wartet auf `promise`, höchstens bis zum Ende des Budgets. */
  async within<T>(what: string, promise: Promise<T>): Promise<T> {
    const left = this.remaining(what);
    let timer: ReturnType<typeof setTimeout> | undefined;
    const expired = new Promise<never>((_, reject) => {
      timer = setTimeout(
        () => reject(new Error(`${what}: Zeitbudget überschritten.`)),
        left,
      );
    });
    try {
      return await Promise.race([promise, expired]);
    } finally {
      clearTimeout(timer);
    }
  }
}
