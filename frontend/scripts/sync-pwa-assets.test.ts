import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const scriptSource = readFileSync(new URL("./sync-pwa-assets.mjs", import.meta.url), "utf8");
const dockerfileSource = readFileSync(new URL("../../Dockerfile", import.meta.url), "utf8");
const appShellLogoFiles = ["logo.svg", "logo-color.svg", "logo-black.svg", "logo-white.svg"];

test("sync-pwa-assets includes public logo files in the service worker cache key", () => {
  for (const logoFile of appShellLogoFiles) {
    assert.match(scriptSource, new RegExp(JSON.stringify(logoFile)));
  }

  assert.match(scriptSource, /appShellAssetHashes/u);
});

test("sync-pwa-assets copies hashed files and compares the cache-key manifest", () => {
  assert.match(scriptSource, /const manifestContent = \[/u);
  assert.match(scriptSource, /export const pwaAssetCacheKey/u);
  assert.match(scriptSource, /if \(!isCurrent\)/u);
  assert.match(scriptSource, /copyFileSync\(sourceFile, join\(generatedDir, targetName\)\)/u);
});

test("Dockerfile makes app shell logos available before frontend postinstall runs", () => {
  const pnpmInstallIndex = dockerfileSource.indexOf("pnpm install --frozen-lockfile");

  assert.notEqual(pnpmInstallIndex, -1);

  for (const logoFile of appShellLogoFiles) {
    const logoCopyIndex = dockerfileSource.indexOf(
      `COPY frontend/public/${logoFile} ./public/${logoFile}`,
    );

    assert.notEqual(logoCopyIndex, -1, `${logoFile} must be copied before pnpm install`);
    assert.ok(logoCopyIndex < pnpmInstallIndex, `${logoFile} must be copied before pnpm install`);
  }
});
