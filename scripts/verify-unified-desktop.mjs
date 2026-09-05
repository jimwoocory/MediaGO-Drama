import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { existsSync, readFileSync, readdirSync, writeFileSync } from "node:fs";
import { createRequire } from "node:module";
import path from "node:path";

// Read-only package verification; the only output is an external evidence report.
const root = path.resolve(process.env.JW_DESKTOP_OUTPUT || "D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905");
const baseline = "D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked";
const packaged = path.join(root, "desktop", "win-unpacked");
const build = path.join(root, "_build");
const require = createRequire(new URL("../apps/workspace/package.json", import.meta.url));
const asar = require("@electron/asar");
const hash = data => createHash("sha256").update(data).digest("hex");
const fileHash = file => hash(readFileSync(file));
function files(dir, prefix = "") {
  return readdirSync(dir, { withFileTypes: true }).flatMap(entry => {
    // A user may already have launched the unpacked app. Never inspect or
    // package its runtime data; ZIP contents are verified separately.
    if (dir === packaged && entry.name === "data" && entry.isDirectory()) return [];
    assert(!entry.isSymbolicLink(), `unexpected symlink: ${entry.name}`);
    const name = prefix + entry.name;
    return entry.isDirectory() ? files(path.join(dir, entry.name), name + "/") : [name];
  }).sort();
}
const report = { baseline, packaged, excludedRuntimeDirectories: ["data"], passed: false, preservedFiles: [], files: [] };
try {
  const names = files(packaged);
  assert(!names.some(name => /(^|\/)(data|\.codex|workspace|auth\.json|.*\.db)(\/|$)/i.test(name)), "user data found in package");
  // These resources must remain byte-identical to the explicitly selected base.
  for (const directory of ["resources/agents", "resources/tools"]) {
    const expected = files(path.join(baseline, directory));
    assert.deepEqual(files(path.join(packaged, directory)), expected, `${directory} inventory changed`);
    for (const name of expected) {
      const relative = directory + "/" + name;
      assert.equal(fileHash(path.join(packaged, relative)), fileHash(path.join(baseline, relative)), relative);
      report.preservedFiles.push(relative);
    }
  }
  for (const name of ["resources/local-cli.json", "resources/model-platform.json"]) {
    assert.equal(fileHash(path.join(packaged, name)), fileHash(path.join(baseline, name)), name);
    report.preservedFiles.push(name);
  }
  const originalAsar = path.join(baseline, "resources/app.asar");
  const finalAsar = path.join(packaged, "resources/app.asar");
  const originalFiles = asar.listPackage(originalAsar).map(name => name.replaceAll("\\", "/").replace(/^\//, ""));
  for (const name of originalFiles.filter(name => !name.includes("/") && /\.(js|cjs)$/.test(name))) {
    assert.equal(hash(asar.extractFile(finalAsar, name)), hash(asar.extractFile(originalAsar, name)), `desktop shell ${name}`);
    report.preservedFiles.push("app.asar/" + name);
  }
  for (const name of files(path.join(build, "renderer"))) {
    assert.equal(hash(asar.extractFile(finalAsar, path.join("renderer", name))), fileHash(path.join(build, "renderer", name)), `renderer ${name}`);
  }
  for (const name of ["mediago-server.exe", "mediago-document-mcp.exe", "mediago-generation-mcp.exe"]) {
    assert.equal(fileHash(path.join(packaged, "resources/bin", name)), fileHash(path.join(build, "bin", name)), name);
  }
  report.baselineAsarSHA256 = fileHash(originalAsar);
  report.packagedAsarSHA256 = fileHash(finalAsar);
  report.files = names.map(name => ({ name, sha256: fileHash(path.join(packaged, name)) }));
  const zip = path.join(root, "desktop", path.basename(root) + "-x64.zip");
  assert(existsSync(zip), "ZIP is not built");
  report.zip = { path: zip, sha256: fileHash(zip) };
  report.passed = true;
} catch (error) {
  report.error = String(error);
  process.exitCode = 1;
} finally {
  writeFileSync(path.join(root, "package-verification.json"), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ passed: report.passed, files: report.files.length, preserved: report.preservedFiles.length, zip: report.zip, error: report.error }, null, 2));
}
