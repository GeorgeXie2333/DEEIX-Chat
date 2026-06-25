import assert from "node:assert/strict";
import test from "node:test";

import {
  MODEL_MENU_AUXILIARY_LONG_PRESS_MS,
  MODEL_MENU_AUXILIARY_MOVE_TOLERANCE_PX,
  keepDropdownMenuOpenAfterModelSelect,
  shouldCancelModelMenuAuxiliaryLongPress,
  stopModelMenuClickPropagation,
} from "./chat-model-picker-events.ts";

test("keepDropdownMenuOpenAfterModelSelect prevents Radix default item close", () => {
  let prevented = false;

  keepDropdownMenuOpenAfterModelSelect({
    preventDefault() {
      prevented = true;
    },
  });

  assert.equal(prevented, true);
});

test("stopModelMenuClickPropagation prevents composer focus bubbling", () => {
  let stopped = false;

  stopModelMenuClickPropagation({
    stopPropagation() {
      stopped = true;
    },
  });

  assert.equal(stopped, true);
});

test("model menu auxiliary icon long press uses a deliberate mobile threshold", () => {
  assert.equal(MODEL_MENU_AUXILIARY_LONG_PRESS_MS, 500);
  assert.equal(MODEL_MENU_AUXILIARY_MOVE_TOLERANCE_PX, 8);
});

test("model menu auxiliary icon long press cancels after drag tolerance", () => {
  assert.equal(
    shouldCancelModelMenuAuxiliaryLongPress(
      { clientX: 12, clientY: 20 },
      { clientX: 19, clientY: 20 },
    ),
    false,
  );
  assert.equal(
    shouldCancelModelMenuAuxiliaryLongPress(
      { clientX: 12, clientY: 20 },
      { clientX: 21, clientY: 20 },
    ),
    true,
  );
  assert.equal(
    shouldCancelModelMenuAuxiliaryLongPress(
      { clientX: 12, clientY: 20 },
      { clientX: 12, clientY: 29 },
    ),
    true,
  );
});
