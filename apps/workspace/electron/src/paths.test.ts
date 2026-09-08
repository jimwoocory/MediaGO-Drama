import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
	appData: "",
	packaged: true,
	userData: "",
}));

vi.mock("electron", () => ({
	app: {
		get isPackaged() {
			return mocks.packaged;
		},
		getPath: (name: string) => (name === "userData" ? mocks.userData : mocks.appData),
	},
}));

describe("portable workspace directory", () => {
	let temporaryRoot = "";

	beforeEach(() => {
		vi.resetModules();
		temporaryRoot = mkdtempSync(join(tmpdir(), "mediago-paths-"));
		mocks.userData = join(temporaryRoot, "user-data");
		mocks.appData = join(temporaryRoot, "app-data");
		mocks.packaged = true;
	});

	afterEach(() => {
		rmSync(temporaryRoot, { force: true, recursive: true });
	});

	it("uses the portable location until a user chooses another directory", async () => {
		const paths = await import("./paths.js");

		expect(paths.portableWorkspaceDir()).toBe(join(dirname(process.execPath), "data", "workspace"));
	});

	it("persists an absolute user-selected directory", async () => {
		const paths = await import("./paths.js");
		const selected = join(temporaryRoot, "projects");

		paths.setPortableWorkspaceDir(selected);

		expect(paths.portableWorkspaceDir()).toBe(selected);
		expect(readFileSync(join(mocks.userData, "workspace-location.json"), "utf8")).toContain(
			'"workspaceDir"',
		);
	});

	it("clears the selected directory when restoring the default", async () => {
		const paths = await import("./paths.js");
		paths.setPortableWorkspaceDir(join(temporaryRoot, "projects"));

		paths.resetPortableWorkspaceDir();

		expect(paths.portableWorkspaceDir()).toBe(join(dirname(process.execPath), "data", "workspace"));
	});
});
