import { render, screen } from "@testing-library/react";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import App from "../App";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(new Response("[]", { status: 200 }))));
});
afterEach(() => vi.unstubAllGlobals());

it("hidden03: submit button is labelled 'Create note'", async () => {
  render(<App />);
  expect(await screen.findByRole("button", { name: "Create note" })).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Add note" })).not.toBeInTheDocument();
});
