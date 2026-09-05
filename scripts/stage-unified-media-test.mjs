import { cpSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

// Creates a fresh desktop staging directory. Never copy the portable data folder.
const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const root = path.resolve(process.env.JW_DESKTOP_OUTPUT || 'D:/openai/MediaGo-Builds/JW-Drama-Unified-20260905');
const build = path.join(root, '_build');
const buildName = path.basename(root);
const original = 'D:/openai/MediaGo-Builds/JW-Drama-Providers-20260905/win-unpacked';
const app = path.resolve(build, 'desktop-app');
if (existsSync(app)) throw new Error('Desktop staging already exists; refusing to overwrite it.');
if (!existsSync(path.join(build, 'bin', 'mediago-server.exe')) || !existsSync(path.join(build, 'renderer', 'index.html'))) throw new Error('Build renderer and server into _build first.');
const require = createRequire(path.join(repo, 'apps', 'workspace', 'package.json'));
const asar = require('@electron/asar');
mkdirSync(app, { recursive: true });
asar.extractAll(path.join(original, 'resources', 'app.asar'), app);
const renderer = path.resolve(app, 'renderer');
if (!renderer.startsWith(app + path.sep)) throw new Error('Invalid staging path');
rmSync(renderer, { recursive: true, force: true });
cpSync(path.join(build, 'renderer'), renderer, { recursive: true });
const pkg = JSON.parse(readFileSync(path.join(app, 'package.json'), 'utf8'));
pkg.version = process.env.JW_DESKTOP_VERSION || '0.1.0-beta.unified.20260905';
writeFileSync(path.join(app, 'package.json'), JSON.stringify(pkg, null, 2));
const config = {
 appId: 'team.torchstellar.mediagodrama', productName: 'JW Drama', electronVersion: '42.4.1', asar: true, npmRebuild: false,
 directories: { output: path.join(root, 'desktop') }, artifactName: buildName + '-${arch}.${ext}',
 files: ['**/*', '!**/*.map'],
 extraResources: [
  { from: path.join(original, 'resources', 'agents'), to: 'agents' },
  { from: path.join(original, 'resources', 'tools'), to: 'tools' },
  { from: path.join(original, 'resources', 'local-cli.json'), to: 'local-cli.json' },
  { from: path.join(original, 'resources', 'model-platform.json'), to: 'model-platform.json' },
  { from: path.join(build, 'bin'), to: 'bin' },
 ],
 win: { target: ['zip'], icon: path.join(repo, 'apps', 'workspace', 'build', 'icons', 'icon.ico') },
 electronFuses: { runAsNode: false, enableCookieEncryption: true, enableNodeOptionsEnvironmentVariable: false, enableNodeCliInspectArguments: false, enableEmbeddedAsarIntegrityValidation: true, onlyLoadAppFromAsar: true, loadBrowserProcessSpecificV8Snapshot: false, grantFileProtocolExtraPrivileges: false },
 publish: null,
};
writeFileSync(path.join(build, 'electron-builder.json'), JSON.stringify(config, null, 2));
console.log('Staged desktop app without account or project data:', app);
