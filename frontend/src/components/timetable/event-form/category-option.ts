export interface RoomOption {
  id: number;
  name: string;
  building?: string;
}

export interface CategoryOption {
  readonly id: string;
  readonly name: string;
  readonly disabled?: boolean;
}

const CATEGORY_DISABLED_LABEL = "nicht mehr angeboten";

export function withUnavailableCurrentCategory(
  freshCategories: readonly CategoryOption[],
  currentId: string,
  previousCategories: readonly CategoryOption[],
  fallbackName: string,
): CategoryOption[] {
  if (
    !currentId ||
    freshCategories.some((category) => category.id === currentId)
  ) {
    return [...freshCategories];
  }
  const previousCategory = previousCategories.find(
    (category) => category.id === currentId,
  );
  return [
    ...freshCategories,
    {
      id: currentId,
      name: previousCategory?.name ?? fallbackName,
      disabled: true,
    },
  ];
}

export function categorySelectProps(category: CategoryOption) {
  return {
    label: category.disabled
      ? `${category.name} (${CATEGORY_DISABLED_LABEL})`
      : category.name,
    disabled: category.disabled,
  };
}
