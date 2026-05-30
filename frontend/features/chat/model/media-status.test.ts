import assert from "node:assert/strict";
import test from "node:test";

import { resolveMediaStatusProgress } from "./media-status.ts";

test("resolveMediaStatusProgress accepts finite percentages", () => {
  assert.equal(resolveMediaStatusProgress(33), 33);
  assert.equal(resolveMediaStatusProgress("44"), 44);
});

test("resolveMediaStatusProgress clamps and rejects invalid values", () => {
  assert.equal(resolveMediaStatusProgress(120), 100);
  assert.equal(resolveMediaStatusProgress(-10), 0);
  assert.equal(resolveMediaStatusProgress("not-a-number"), undefined);
  assert.equal(resolveMediaStatusProgress(null), undefined);
});
