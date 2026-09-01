import { createHash, timingSafeEqual } from "node:crypto";

export interface StartupCredentials {
	username?: unknown;
	password?: unknown;
}

const startupUsername = "admin";
const startupPasswordSHA256 = "b4f580912236ecd026067bc331045eb12d59b1ec54fcff44e75ee4ba06356e27";

export const validateStartupCredentials = (value: StartupCredentials): boolean => {
	const username = typeof value?.username === "string" ? value.username.trim() : "";
	const password = typeof value?.password === "string" ? value.password : "";
	if (username !== startupUsername || !password) return false;

	const actual = Buffer.from(createHash("sha256").update(password, "utf8").digest("hex"), "utf8");
	const expected = Buffer.from(startupPasswordSHA256, "utf8");
	return actual.length === expected.length && timingSafeEqual(actual, expected);
};
