import { readFile, mkdir, writeFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createElement as h } from "react";

const require = createRequire(import.meta.url);
const { ImageResponse } = require("../node_modules/next/dist/compiled/@vercel/og/index.node.js");
const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = join(__dirname, "..");
const providerSlugs = ["openai", "anthropic", "google", "xai", "deepseek", "zhipu"];

function svgDataURL(value) {
  return `data:image/svg+xml;base64,${Buffer.from(value).toString("base64")}`;
}

const [brandLogo, regularFont, boldFont, ...providerIcons] = await Promise.all([
  readFile(join(frontendRoot, "public/logo.svg"), "utf8"),
  readFile("C:/Windows/Fonts/Deng.ttf"),
  readFile("C:/Windows/Fonts/Dengb.ttf"),
  ...providerSlugs.map((slug) =>
    readFile(join(frontendRoot, "public/vendor/lobehub-icons", `${slug}.svg`), "utf8")),
]);

const providerNodes = providerIcons.map((icon, index) =>
  h(
    "div",
    {
      key: providerSlugs[index],
      style: {
        width: 66,
        height: 66,
        border: "1px solid #d9d4cc",
        borderRadius: 999,
        background: "rgba(255,255,255,0.62)",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
      },
    },
    h("img", {
      src: svgDataURL(icon),
      width: 29,
      height: 29,
      alt: "",
    }),
  ));

const image = new ImageResponse(
  h(
    "div",
    {
      style: {
        width: "100%",
        height: "100%",
        display: "flex",
        position: "relative",
        overflow: "hidden",
        background: "#f8f6f1",
        color: "#39372f",
        fontFamily: "Comi Public",
      },
    },
    h("div", {
      style: {
        position: "absolute",
        width: 720,
        height: 720,
        top: -390,
        right: -130,
        borderRadius: 999,
        background: "radial-gradient(circle, rgba(205,99,61,0.20) 0%, rgba(205,99,61,0) 70%)",
      },
    }),
    h("div", {
      style: {
        position: "absolute",
        width: 520,
        height: 520,
        bottom: -360,
        left: -120,
        borderRadius: 999,
        background: "radial-gradient(circle, rgba(57,55,47,0.10) 0%, rgba(57,55,47,0) 72%)",
      },
    }),
    h(
      "div",
      {
        style: {
          width: "100%",
          padding: "74px 88px 70px",
          display: "flex",
          flexDirection: "column",
          justifyContent: "space-between",
        },
      },
      h("img", {
        src: svgDataURL(brandLogo),
        width: 205,
        height: 42,
        alt: "Comi AI",
      }),
      h(
        "div",
        {
          style: {
            display: "flex",
            flexDirection: "column",
          },
        },
        h(
          "div",
          {
            style: {
              display: "flex",
              flexDirection: "column",
              fontSize: 82,
              lineHeight: 1.05,
              letterSpacing: "-4px",
              fontWeight: 700,
            },
          },
          h("span", null, "一个入口，"),
          h("span", null, "用遍主流 AI"),
        ),
        h(
          "div",
          {
            style: {
              marginTop: 28,
              fontSize: 24,
              lineHeight: 1.45,
              color: "#79756b",
              display: "flex",
            },
          },
          "11+ 模型厂商 · 官方 API 直连 · 价格与官方同步",
        ),
      ),
      h(
        "div",
        {
          style: {
            display: "flex",
            alignItems: "center",
            gap: 14,
          },
        },
        ...providerNodes,
      ),
    ),
  ),
  {
    width: 1200,
    height: 630,
    fonts: [
      {
        name: "Comi Public",
        data: regularFont,
        weight: 400,
        style: "normal",
      },
      {
        name: "Comi Public",
        data: boldFont,
        weight: 700,
        style: "normal",
      },
    ],
  },
);

const outputPath = join(frontendRoot, "public/og/comi-ai-landing.png");
await mkdir(dirname(outputPath), { recursive: true });
await writeFile(outputPath, Buffer.from(await image.arrayBuffer()));
console.log(outputPath);
