export const settingsErrorMessage = (error: unknown, fallback: string): string => {
	if (
		error &&
		typeof error === "object" &&
		"message" in error &&
		typeof error.message === "string" &&
		error.message.trim()
	) {
		return error.message;
	}
	return fallback;
};
