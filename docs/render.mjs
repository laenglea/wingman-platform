// Render docs/architecture.html → docs/architecture.png
// Usage:  npx --yes playwright@latest install chromium && node docs/render.mjs
// Or:     task docs:render

import { chromium } from 'playwright';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const html = resolve(__dirname, 'architecture.html');
const out  = resolve(__dirname, 'architecture.png');

// Match the .poster canvas: 2000 × 1414 plus 24px top/bottom margin.
const width  = 2000 + 48;
const height = 1414 + 48;

const browser = await chromium.launch();
const ctx = await browser.newContext({
  viewport: { width, height },
  deviceScaleFactor: 2, // retina-quality PNG
});
const page = await ctx.newPage();
await page.goto('file://' + html, { waitUntil: 'networkidle' });
await page.waitForTimeout(400); // let webfonts settle
await page.screenshot({
  path: out,
  clip: { x: 24, y: 24, width: 2000, height: 1414 },
});
await browser.close();
console.log('wrote', out);
