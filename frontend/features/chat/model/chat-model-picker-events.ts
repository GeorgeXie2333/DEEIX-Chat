type PreventableMenuEvent = {
  preventDefault: () => void;
};

type StoppableMenuEvent = {
  stopPropagation: () => void;
};

export function keepDropdownMenuOpenAfterModelSelect(event: PreventableMenuEvent): void {
  event.preventDefault();
}

export function stopModelMenuClickPropagation(event: StoppableMenuEvent): void {
  event.stopPropagation();
}
