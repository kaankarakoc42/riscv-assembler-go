import fs from "node:fs";

const [source = "docs/proje-raporu.md", output = "docs/proje-raporu-preview.md", manifestPath = "build/mermaid-jpeg/manifest.json"] = process.argv.slice(2);

const manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
let diagramIndex = 0;

const markdown = fs.readFileSync(source, "utf8").replace(
  /```mermaid\s*\r?\n[\s\S]*?```/g,
  () => {
    const item = manifest[diagramIndex];
    diagramIndex += 1;

    if (!item) {
      throw new Error(`Manifest entry missing for Mermaid block ${diagramIndex}.`);
    }

    return `![Diyagram ${diagramIndex}](../build/mermaid-jpeg/${item.file})`;
  },
);

if (diagramIndex !== manifest.length) {
  throw new Error(`Manifest count is ${manifest.length}, but replaced ${diagramIndex} Mermaid blocks.`);
}

fs.writeFileSync(output, markdown, "utf8");
console.log(`${output} yazildi; ${diagramIndex} JPEG linki eklendi.`);
