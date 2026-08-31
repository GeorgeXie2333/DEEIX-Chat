import assert from "node:assert/strict";
import test from "node:test";

import {
  removeOptionAtPath,
  resolveServerOptionControls,
  setOptionAtPath,
} from "./chat-model-controls.ts";

test("resolveServerOptionControls supports scalar control types and locked defaults", () => {
  const controls = resolveServerOptionControls({
    controls: [
      { path: "temperature", type: "number" },
      { path: "reasoning.effort", type: "select", options: ["low", "high"] },
      { path: "user_note", type: "text" },
      { path: "streaming", type: "boolean" },
      { path: "locked.mode", type: "select", options: ["safe", "fast"] },
    ],
    options: {
      temperature: 0.7,
      reasoning: { effort: "high" },
      user_note: "hello",
      streaming: false,
      locked: { mode: "fast" },
    },
    defaultOptions: {
      temperature: 1,
      reasoning: { effort: "low" },
      user_note: "default",
      streaming: true,
      locked: { mode: "safe" },
    },
    lockedOptionPaths: ["locked.mode"],
    policy: null,
    protocol: "custom",
  });

  assert.deepEqual(
    controls.map(({ key, valueType, value, locked }) => ({ key, valueType, value, locked })),
    [
      { key: "temperature", valueType: "number", value: 0.7, locked: false },
      { key: "reasoning.effort", valueType: "select", value: "high", locked: false },
      { key: "user_note", valueType: "text", value: "hello", locked: false },
      { key: "streaming", valueType: "boolean", value: false, locked: false },
      { key: "locked.mode", valueType: "select", value: "safe", locked: true },
    ],
  );
});

test("option path helpers update nested values without disturbing siblings", () => {
  const updated = setOptionAtPath({ sibling: 1, nested: { keep: true } }, ["nested", "value"], "x");
  assert.deepEqual(updated, { sibling: 1, nested: { keep: true, value: "x" } });
  assert.deepEqual(removeOptionAtPath(updated, ["nested", "value"]), { sibling: 1, nested: { keep: true } });
});
