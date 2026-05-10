import fs from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";

const [inputFile = "docs/proje-raporu.md", outputDir = "build/mermaid-jpeg", toolsDir = ".pdf-tools"] = process.argv.slice(2);
const root = process.cwd();
const absInput = path.resolve(root, inputFile);
const absOutputDir = path.resolve(root, outputDir);
const absToolsDir = path.resolve(root, toolsDir);
const requireFromTools = createRequire(path.join(absToolsDir, "package.json"));

const { chromium } = requireFromTools("playwright");
const mermaidPath = requireFromTools.resolve("mermaid/dist/mermaid.min.js");

function extractMermaidBlocks(markdown) {
  const blocks = [];
  const pattern = /```mermaid\s*\r?\n([\s\S]*?)```/g;
  let match;

  while ((match = pattern.exec(markdown)) !== null) {
    const before = markdown.slice(0, match.index);
    const line = before.split(/\r?\n/).length;
    blocks.push({
      index: blocks.length + 1,
      line,
      source: match[1].trim(),
    });
  }

  return blocks;
}

async function launchBrowser() {
  try {
    return await chromium.launch();
  } catch (error) {
    const channels = ["msedge", "chrome"];
    for (const channel of channels) {
      try {
        return await chromium.launch({ channel });
      } catch {
        // Try the next locally installed browser.
      }
    }

    throw error;
  }
}

function pageHtml(mermaidSource) {
  return `<!doctype html>
<html lang="tr">
<head>
  <meta charset="utf-8">
  <style>
    body {
      background: #ffffff;
      margin: 0;
      padding: 24px;
      font-family: "Segoe UI", Arial, sans-serif;
    }

    #wrap {
      background: #ffffff;
      border: 1px solid #d8dee9;
      border-radius: 12px;
      display: inline-block;
      padding: 20px;
    }

    .mermaid {
      background: #ffffff;
    }
  </style>
</head>
<body>
  <div id="wrap"></div>
  <script>${mermaidSource}</script>
  <script>
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: "loose",
      theme: "default",
      flowchart: { htmlLabels: true, useMaxWidth: false },
      sequence: { useMaxWidth: false }
    });

    window.renderDiagram = async (source, id) => {
      const target = document.querySelector("#wrap");
      const result = await mermaid.render("diagram_" + id, source);
      target.innerHTML = result.svg;

      const svg = target.querySelector("svg");
      if (!svg) {
        throw new Error("Mermaid SVG uretmedi.");
      }

      svg.style.background = "#ffffff";
      svg.style.maxWidth = "none";
      return true;
    };
  </script>
</body>
</html>`;
}

const markdown = await fs.readFile(absInput, "utf8");
const mermaidSource = await fs.readFile(mermaidPath, "utf8");
const blocks = extractMermaidBlocks(markdown);

if (blocks.length === 0) {
  console.log("Mermaid diyagrami bulunamadi.");
  process.exit(0);
}

await fs.rm(absOutputDir, { recursive: true, force: true });
await fs.mkdir(absOutputDir, { recursive: true });

const browser = await launchBrowser();
try {
  const manifest = [];

  for (const block of blocks) {
    const page = await browser.newPage({
      viewport: {
        width: 2200,
        height: 1800,
        deviceScaleFactor: 2,
      },
    });

    await page.setContent(pageHtml(mermaidSource), { waitUntil: "networkidle" });
    await page.evaluate(
      ({ source, id }) => window.renderDiagram(source, id),
      { source: block.source, id: block.index },
    );

    const wrap = page.locator("#wrap");
    const filename = `diagram-${String(block.index).padStart(2, "0")}-line-${block.line}.jpeg`;
    const outPath = path.join(absOutputDir, filename);

    await wrap.screenshot({
      path: outPath,
      type: "jpeg",
      quality: 95,
    });

    manifest.push({
      file: filename,
      sourceLine: block.line,
    });

    console.log(`${filename} yazildi.`);
    await page.close();
  }

  await fs.writeFile(
    path.join(absOutputDir, "manifest.json"),
    `${JSON.stringify(manifest, null, 2)}\n`,
    "utf8",
  );
} finally {
  await browser.close();
}

console.log(`${blocks.length} Mermaid diyagrami JPEG olarak kaydedildi: ${absOutputDir}`);
