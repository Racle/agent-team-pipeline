import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

const all = Array.from({ length: 12 }, (_, i) => ({
  id: i + 1, title: `Note ${i + 1}`, body: "", created_at: "", updated_at: "",
}));

it("hidden08: pager fetches 5 per page and shows page indicator", async () => {
  const f = vi.fn((url: string) => {
    const u = new URL(url, "http://x");
    const limit = Number(u.searchParams.get("limit") ?? 1000);
    const offset = Number(u.searchParams.get("offset") ?? 0);
    return Promise.resolve(
      new Response(JSON.stringify(all.slice(offset, offset + limit)), {
        status: 200,
        headers: { "X-Total-Count": "12" },
      }),
    );
  });
  vi.stubGlobal("fetch", f);
  render(<App />);
  expect(await screen.findByText("Note 1")).toBeInTheDocument();
  expect(screen.queryByText("Note 6")).not.toBeInTheDocument();
  expect(screen.getByText("Page 1 of 3")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Next" })).toBeEnabled();

  await userEvent.click(screen.getByRole("button", { name: "Next" }));
  expect(await screen.findByText("Note 6")).toBeInTheDocument();
  expect(screen.queryByText("Note 1")).not.toBeInTheDocument();
  expect(screen.getByText("Page 2 of 3")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Previous" })).toBeEnabled();
  const urls = f.mock.calls.map((c) => new URL(c[0] as string, "http://x"));
  expect(urls.some((u) => u.searchParams.get("limit") === "5" && u.searchParams.get("offset") === "5")).toBe(true);

  await userEvent.click(screen.getByRole("button", { name: "Next" }));
  expect(await screen.findByText("Note 11")).toBeInTheDocument();
  await waitFor(() => expect(screen.getByRole("button", { name: "Next" })).toBeDisabled());
  expect(screen.getByText("Page 3 of 3")).toBeInTheDocument();
});
