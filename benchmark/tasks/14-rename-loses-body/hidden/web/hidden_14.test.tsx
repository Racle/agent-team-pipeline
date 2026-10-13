import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

const note = { id: 1, title: "Old title", body: "Precious body", created_at: "", updated_at: "" };

// Fake server with the contract's PUT semantics: absent fields keep their value.
function setup(opts: { fail?: "status" | "network" } = {}) {
  let current = { ...note };
  const f = vi.fn(async (url: string, init?: RequestInit) => {
    if (init?.method === "PUT") {
      if (opts.fail === "network") throw new TypeError("network down");
      if (opts.fail === "status") return new Response(JSON.stringify({ error: "boom" }), { status: 500 });
      current = { ...current, ...JSON.parse(init.body as string) };
      return new Response(JSON.stringify(current), { status: 200 });
    }
    return new Response(JSON.stringify([current]), { status: 200 });
  });
  vi.stubGlobal("fetch", f);
  return f;
}

const puts = (f: ReturnType<typeof setup>) =>
  f.mock.calls.filter((c) => (c[1] as RequestInit | undefined)?.method === "PUT");

async function rename(newTitle: string) {
  await userEvent.click(await screen.findByRole("button", { name: "Rename" }));
  const input = screen.getByRole("textbox", { name: "New title" });
  expect(input).toHaveValue("Old title");
  await userEvent.clear(input);
  await userEvent.type(input, newTitle);
  await userEvent.click(screen.getByRole("button", { name: "Confirm" }));
}

it("hidden14: rename sends only the title and keeps the body", async () => {
  const f = setup();
  render(<App />);
  await rename("Fresh title");
  expect(await screen.findByText("Fresh title")).toBeInTheDocument();
  expect(screen.getByText("Precious body")).toBeInTheDocument();
  expect(screen.queryByText("Old title")).not.toBeInTheDocument();
  const [url, init] = puts(f)[0] as [string, RequestInit];
  expect(url).toBe("/api/notes/1");
  expect(new Headers(init.headers).get("Content-Type")).toBe("application/json");
  expect(JSON.parse(init.body as string)).toEqual({ title: "Fresh title" });
  await waitFor(() => expect(screen.queryByRole("textbox", { name: "New title" })).not.toBeInTheDocument());
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

for (const fail of ["status", "network"] as const) {
  it(`hidden14: failed rename (${fail}) keeps old data, shows alert, box stays open`, async () => {
    setup({ fail });
    render(<App />);
    await rename("Fresh title");
    const alert = await screen.findByRole("alert");
    expect(alert.textContent?.toLowerCase()).toContain("rename failed");
    expect(screen.getByText("Old title")).toBeInTheDocument();
    expect(screen.getByText("Precious body")).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "New title" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Confirm" })).toBeInTheDocument();
  });
}

it("hidden14: form and delete still present", async () => {
  setup();
  render(<App />);
  expect(await screen.findByRole("button", { name: "Delete" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Add note" })).toBeInTheDocument();
});
