import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

it("hidden07: checkbox reflects done and toggling sends PATCH", async () => {
  let notes = [
    { id: 1, title: "Alpha", body: "", done: false, created_at: "", updated_at: "" },
    { id: 2, title: "Beta", body: "", done: true, created_at: "", updated_at: "" },
  ];
  const f = vi.fn((url: string, init?: RequestInit) => {
    if (init?.method === "PATCH") {
      const body = JSON.parse(init.body as string);
      notes = notes.map((n) => (n.id === 1 ? { ...n, ...body } : n));
      return Promise.resolve(new Response(JSON.stringify(notes[0]), { status: 200 }));
    }
    return Promise.resolve(new Response(JSON.stringify(notes), { status: 200 }));
  });
  vi.stubGlobal("fetch", f);
  render(<App />);
  const alpha = await screen.findByRole("checkbox", { name: "Done: Alpha" });
  const beta = await screen.findByRole("checkbox", { name: "Done: Beta" });
  expect(alpha).not.toBeChecked();
  expect(beta).toBeChecked();
  await userEvent.click(alpha);
  await waitFor(() => {
    const call = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "PATCH");
    expect(call).toBeTruthy();
    expect(call![0]).toBe("/api/notes/1");
    expect(JSON.parse((call![1] as RequestInit).body as string)).toEqual({ done: true });
  });
  await waitFor(() => expect(screen.getByRole("checkbox", { name: "Done: Alpha" })).toBeChecked());
});
