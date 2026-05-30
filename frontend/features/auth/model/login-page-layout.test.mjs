import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const loginPageSource = readFileSync(join(__dirname, "../components/login-page.tsx"), "utf8");

function getClassNameAfter(marker) {
  const markerIndex = loginPageSource.indexOf(marker);
  assert.notEqual(markerIndex, -1, `missing ${marker}`);

  const afterMarker = loginPageSource.slice(markerIndex);
  const classMatch = afterMarker.match(/className="([^"]+)"/);
  assert.ok(classMatch, `missing className after ${marker}`);
  return classMatch[1];
}

test("login page owns vertical scrolling inside the fixed app shell", () => {
  const mainClassName = getClassNameAfter("<main");

  assert.match(mainClassName, /\bh-svh\b/, "login main must use a fixed viewport height so its overflow can scroll");
  assert.match(mainClassName, /\boverflow-y-auto\b/, "login main must keep vertical scrolling enabled");
  assert.match(mainClassName, /\boverflow-x-hidden\b/, "login main must prevent horizontal page drift while scrolling");
});

