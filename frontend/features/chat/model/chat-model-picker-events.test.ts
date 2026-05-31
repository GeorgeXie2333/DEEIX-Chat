import assert from "node:assert/strict";
import test from "node:test";

import {
  keepDropdownMenuOpenAfterModelSelect,
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
