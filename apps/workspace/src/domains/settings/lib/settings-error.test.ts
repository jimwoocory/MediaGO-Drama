import { describe, expect, it } from "vitest";
import { settingsErrorMessage } from "./settings-error";

describe("settingsErrorMessage", () => {
	it("preserves structured HTTP errors as well as Error instances", () => {
		expect(
			settingsErrorMessage({ code: 400, message: "Use /v1 (number 1), not /vl" }, "保存失败"),
		).toContain("number 1");
		expect(settingsErrorMessage(new Error("disk error"), "保存失败")).toBe("disk error");
		expect(settingsErrorMessage(null, "保存失败")).toBe("保存失败");
	});
});
