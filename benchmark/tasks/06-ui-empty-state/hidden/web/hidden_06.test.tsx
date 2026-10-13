import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

function stub(list: unknown[]) {
  const f = vi.fn(() => Promise.resolve(new Response(JSON.stringify(list), { status: 200 })));
  vi.stubGlobal("fetch", f);
  return f;
}

it("hidden06: shows empty state when there are no notes", async () => {
  stub([]);
  render(<App />);
  expect(await screen.findByText("No notes yet")).toBeInTheDocument();
  expect(screen.queryAllByRole("listitem")).toHaveLength(0);
});

it("hidden06: hides empty state when notes exist", async () => {
  stub([{ id: 1, title: "First", body: "Hello", created_at: "", updated_at: "" }]);
  render(<App />);
  expect(await screen.findByText("First")).toBeInTheDocument();
  await waitFor(() => expect(screen.queryByText("No notes yet")).not.toBeInTheDocument());
});
