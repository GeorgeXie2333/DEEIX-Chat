type PreventableMenuEvent = {
  preventDefault: () => void;
};

type StoppableMenuEvent = {
  stopPropagation: () => void;
};

type PointerPosition = {
  clientX: number;
  clientY: number;
};

export const MODEL_MENU_AUXILIARY_LONG_PRESS_MS = 500;
export const MODEL_MENU_AUXILIARY_MOVE_TOLERANCE_PX = 8;

export function keepDropdownMenuOpenAfterModelSelect(event: PreventableMenuEvent): void {
  event.preventDefault();
}

export function stopModelMenuClickPropagation(event: StoppableMenuEvent): void {
  event.stopPropagation();
}

export function shouldCancelModelMenuAuxiliaryLongPress(
  start: PointerPosition,
  current: PointerPosition,
  tolerance = MODEL_MENU_AUXILIARY_MOVE_TOLERANCE_PX,
): boolean {
  return (
    Math.abs(current.clientX - start.clientX) > tolerance ||
    Math.abs(current.clientY - start.clientY) > tolerance
  );
}
