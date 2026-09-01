import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ProductionProfileSelector } from "@/domains/episode/components/ProductionProfileSelector";

describe("ProductionProfileSelector", () => {
	afterEach(cleanup);

	it("stays disabled when verified production modes are not configured", () => {
		const onChange = vi.fn();
		render(<ProductionProfileSelector profiles={[]} onChange={onChange} />);
		const select = screen.getByLabelText("制作模式") as HTMLSelectElement;
		expect(select.disabled).toBe(true);
		expect(screen.getByText("未配置制作模式")).toBeTruthy();
		expect(screen.getByText(/不会用占位名称替代/)).toBeTruthy();
	});

	it("selects one of six registered profiles by stable id", () => {
		const onChange = vi.fn();
		const profiles = Array.from({ length: 6 }, (_, index) => ({
			id: `mode-${index + 1}`,
			label: `模式 ${index + 1}`,
			description: "fixture",
			version: 1,
		}));
		render(<ProductionProfileSelector profiles={profiles} value="mode-1" onChange={onChange} />);
		fireEvent.change(screen.getByLabelText("制作模式"), { target: { value: "mode-6" } });
		expect(onChange).toHaveBeenCalledWith("mode-6");
	});
});
