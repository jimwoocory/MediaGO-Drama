import { app } from "electron";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, isAbsolute, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const electronDir = dirname(fileURLToPath(import.meta.url));

export const workspaceDir = resolve(electronDir, "..", "..");
export const isPackaged = () => app.isPackaged;

export const rendererDistDir = () =>
	isPackaged() ? join(electronDir, "renderer") : join(workspaceDir, "dist");
export const preloadPath = () => join(electronDir, "preload.cjs");

export const resourceRoot = () =>
	isPackaged() ? process.resourcesPath : join(workspaceDir, "electron", "resources");

const workspaceLocationPreferenceFile = "workspace-location.json";

type WorkspaceLocationPreference = {
	workspaceDir?: unknown;
};

export const defaultPortableWorkspaceDir = () =>
	join(dirname(process.execPath), "data", "workspace");

const workspaceLocationPreferencePath = () =>
	join(app.getPath("userData"), workspaceLocationPreferenceFile);

const selectedPortableWorkspaceDir = () => {
	try {
		const raw = readFileSync(workspaceLocationPreferencePath(), "utf8");
		const preference = JSON.parse(raw) as WorkspaceLocationPreference;
		const candidate =
			typeof preference.workspaceDir === "string" ? preference.workspaceDir.trim() : "";
		return candidate && isAbsolute(candidate) ? resolve(candidate) : "";
	} catch {
		return "";
	}
};

// Keep the portable location as the default, while allowing a user-selected
// workspace to survive replacing or moving a packaged application folder.
export const portableWorkspaceDir = () =>
	selectedPortableWorkspaceDir() || defaultPortableWorkspaceDir();

export const setPortableWorkspaceDir = (directory: string) => {
	const selected = directory.trim();
	if (!selected || !isAbsolute(selected)) throw new Error("workspace directory must be absolute");
	mkdirSync(app.getPath("userData"), { recursive: true });
	writeFileSync(
		workspaceLocationPreferencePath(),
		`${JSON.stringify({ workspaceDir: resolve(selected) })}\n`,
		"utf8",
	);
};

export const resetPortableWorkspaceDir = () => {
	mkdirSync(app.getPath("userData"), { recursive: true });
	writeFileSync(workspaceLocationPreferencePath(), "{}\n", "utf8");
};

export const legacyUserWorkspaceDir = () =>
	join(app.getPath("appData"), "mediago-drama", "workspace");

export const serverBinaryPath = () => {
	const binary = process.platform === "win32" ? "mediago-server.exe" : "mediago-server";
	return join(resourceRoot(), "bin", binary);
};

export const agentsDir = () => join(resourceRoot(), "agents");
export const toolsDir = () => join(resourceRoot(), "tools");
