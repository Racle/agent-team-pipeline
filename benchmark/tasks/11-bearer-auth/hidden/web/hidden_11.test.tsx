import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import App from "../App";

const note = { id: 1, title: "Secret note", body: "", created_at: "", updated_at: "" };

function authHeader(init?: RequestInit): string | null {
  return new Headers(init?.headers).get("Authorization");
}

function stubApi(token: string) {
  const f = vi.fn((url: string, init?: RequestInit) =>
    Promise.resolve(
      authHeader(init) === `Bearer ${token}`
        ? new Response(JSON.stringify([note]), { status: 200 })
        : new Response(JSON.stringify({ error: "unauthorized" }), { status: 401 }),
    ),
  );
  vi.stubGlobal("fetch", f);
  return f;
}

beforeEach(() => localStorage.clear());
afterEach(() => vi.unstubAllGlobals());

it("hidden11: shows login without a stored token and makes no API calls", async () => {
  const f = stubApi("tok");
  render(<App />);
  expect(await screen.findByLabelText("API token")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Log in" })).toBeInTheDocument();
  expect(f).not.toHaveBeenCalled();
});

it("hidden11: logging in stores the token and sends it as Bearer", async () => {
  const f = stubApi("tok");
  render(<App />);
  await userEvent.type(await screen.findByLabelText("API token"), "tok");
  await userEvent.click(screen.getByRole("button", { name: "Log in" }));
  expect(await screen.findByText("Secret note")).toBeInTheDocument();
  expect(localStorage.getItem("api_token")).toBe("tok");
  expect(f.mock.calls.every((c) => authHeader(c[1] as RequestInit) === "Bearer tok")).toBe(true);
  expect(screen.queryByLabelText("API token")).not.toBeInTheDocument();
});

it("hidden11: a stored token skips login; Log out clears it", async () => {
  localStorage.setItem("api_token", "tok");
  stubApi("tok");
  render(<App />);
  expect(await screen.findByText("Secret note")).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Log out" }));
  expect(await screen.findByLabelText("API token")).toBeInTheDocument();
  expect(localStorage.getItem("api_token")).toBeNull();
});

it("hidden11: a 401 response clears the token and returns to login", async () => {
  localStorage.setItem("api_token", "stale");
  stubApi("tok");
  render(<App />);
  expect(await screen.findByLabelText("API token")).toBeInTheDocument();
  await waitFor(() => expect(localStorage.getItem("api_token")).toBeNull());
});
