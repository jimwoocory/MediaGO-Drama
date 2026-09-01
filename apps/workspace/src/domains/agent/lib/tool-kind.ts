export const inferToolKind = (title: string) => {
	let normalized = title.trim().toLowerCase();
	normalized = normalized.startsWith("tool:")
		? normalized.slice("tool:".length).trim()
		: normalized;
	normalized = normalized.includes("/")
		? normalized.slice(normalized.lastIndexOf("/") + 1)
		: normalized;
	normalized = normalized.includes("__")
		? normalized.slice(normalized.lastIndexOf("__") + 2)
		: normalized;
	switch (normalized) {
		case "list_files":
		case "list_file":
		case "list_directory":
		case "list_dir":
		case "glob":
		case "read":
		case "read_file":
		case "read_text_file":
		case "grep":
		case "search":
		case "search_files":
		case "list_projects":
		case "load_skill":
		case "get_project_config":
		case "list_comments":
		case "get_comment":
			return "read";
		case "write":
		case "write_file":
		case "write_text_file":
		case "edit":
		case "edit_file":
		case "apply_patch":
		case "patch":
		case "update_project_config":
		case "mutate_comment":
			return "edit";
		case "bash":
		case "shell":
		case "execute":
		case "run_command":
		case "generate_media":
		case "generate_media_batch":
			return "execute";
		default:
			return "";
	}
};
