const cancelledNavigationEvents = new WeakSet<Event>();

/**
 * Kennzeichnet einen von einer Navigation Guard abgebrochenen Link-Klick.
 * `next/link` ruft selbst `preventDefault()` auf; der Fortschrittsmelder kann
 * den allgemeinen Event-Status daher nicht als Abbruchsignal verwenden.
 */
export function cancelNavigationProgressFor(event: Event): void {
  cancelledNavigationEvents.add(event);
}

export function wasNavigationProgressCancelled(event: Event): boolean {
  return cancelledNavigationEvents.has(event);
}
