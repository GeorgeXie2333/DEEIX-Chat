import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const source = readFileSync(
  new URL("../hooks/use-chat-data.ts", import.meta.url),
  "utf8",
);

test("chat resume adopts replacement text snapshots before appending deltas", () => {
  assert.match(source, /onTextSnapshot: \(content\) =>/);
  assert.match(source, /resumedTextByRun\[pendingRunID\] = content/);
  assert.match(source, /\{ \.\.\.message, content, contentType: "text" \}/);
});
