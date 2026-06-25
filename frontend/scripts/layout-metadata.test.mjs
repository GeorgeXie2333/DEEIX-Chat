import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const layoutSource = readFileSync(new URL("../app/layout.tsx", import.meta.url), "utf8");

test("root metadata does not override app favicon with PWA tab icons", () => {
  assert.doesNotMatch(
    layoutSource,
    /icon:\s*\[[\s\S]*?pwaAsset\("\/pwa\/icon(?:\.svg|-192\.png|-512\.png)"\)/u,
  );
});
