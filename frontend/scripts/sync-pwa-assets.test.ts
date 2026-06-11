import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const scriptSource = readFileSync(new URL("./sync-pwa-assets.mjs", import.meta.url), "utf8");

test("sync-pwa-assets includes public logo files in the service worker cache key", () => {
  for (const logoFile of ["logo.svg", "logo-color.svg", "logo-black.svg", "logo-white.svg"]) {
    assert.match(scriptSource, new RegExp(JSON.stringify(logoFile)));
  }

  assert.match(scriptSource, /appShellAssetHashes/u);
});
