import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

const notes = [
  { id: 1, title: "A", body: "", tags: ["work", "urgent"], created_at: "", updated_at: "" },
  { id: 2, title: "B", body: "", tags: [], created_at: "", updated_at: "" },
];

it("hidden09: renders tag chips and filters by clicking one", async () => {
  const f = vi.fn((url: string) => {
    const tag = new URL(url, "http://x").searchParams.get("tag");
    const out = tag ? notes.filter((n) => n.tags.includes(tag)) : notes;
    return Promise.resolve(new Response(JSON.stringify(out), { status: 200 }));
  });
  vi.stubGlobal("fetch", f);
  render(<App />);
  expect(await screen.findByRole("button", { name: "Tag: work" })).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Tag: urgent" })).toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "Tag: work" }));
  await waitFor(() =>
    expect(
      f.mock.calls.some((c) => new URL(c[0] as string, "http://x").searchParams.get("tag") === "work"),
    ).toBe(true),
  );
  await waitFor(() => expect(screen.queryByText("B")).not.toBeInTheDocument());
  expect(screen.getByText("A")).toBeInTheDocument();
});

it("hidden09: form sends comma-separated tags as an array", async () => {
  const f = vi.fn((url: string, init?: RequestInit) =>
    Promise.resolve(
      init?.method === "POST"
        ? new Response(JSON.stringify(notes[0]), { status: 201 })
        : new Response("[]", { status: 200 }),
    ),
  );
  vi.stubGlobal("fetch", f);
  render(<App />);
  await userEvent.type(await screen.findByLabelText("Title"), "T");
  await userEvent.type(screen.getByLabelText("Tags"), "a, b");
  await userEvent.click(screen.getByRole("button", { name: "Add note" }));
  await waitFor(() => {
    const call = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "POST");
    expect(call).toBeTruthy();
    expect(JSON.parse((call![1] as RequestInit).body as string).tags).toEqual(["a", "b"]);
  });
});
