import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import App from "../App";

afterEach(() => vi.unstubAllGlobals());

const note = { id: 1, title: "Old title", body: "", version: 1, created_at: "", updated_at: "" };

// Stateful fake server: GET returns the current note; a successful (2xx) PUT replaces it with the response body,
// so both "use the PUT response" and "refetch after PUT" implementations see consistent data.
function setup(putResponse: (init: RequestInit) => Promise<Response>) {
  let current = { ...note };
  const f = vi.fn(async (url: string, init?: RequestInit) => {
    if (init?.method === "PUT") {
      const res = await putResponse(init);
      if (res.ok) current = await res.clone().json();
      return res;
    }
    return new Response(JSON.stringify([current]), { status: 200 });
  });
  vi.stubGlobal("fetch", f);
  return f;
}

async function edit(newTitle: string) {
  await userEvent.click(await screen.findByRole("button", { name: "Edit" }));
  const input = screen.getByRole("textbox", { name: "Edit title" });
  await userEvent.clear(input);
  await userEvent.type(input, newTitle);
  await userEvent.click(screen.getByRole("button", { name: "Save" }));
}

it("hidden12: optimistic update is shown before the server answers; sends If-Match", async () => {
  let resolve!: (r: Response) => void;
  const f = setup(() => new Promise<Response>((r) => (resolve = r)));
  const serverNote = { ...note, title: "Server title", version: 2 };
  render(<App />);
  await edit("New title");
  expect(await screen.findByText("New title")).toBeInTheDocument();
  expect(screen.queryByText("Old title")).not.toBeInTheDocument();
  const put = f.mock.calls.find((c) => (c[1] as RequestInit | undefined)?.method === "PUT")!;
  expect(put[0]).toBe("/api/notes/1");
  expect(new Headers((put[1] as RequestInit).headers).get("If-Match")).toBe('"1"');
  expect(JSON.parse((put[1] as RequestInit).body as string).title).toBe("New title");
  resolve(new Response(JSON.stringify(serverNote), { status: 200 }));
  // the list reflects the server's response, and no failure toast appears
  expect(await screen.findByText("Server title")).toBeInTheDocument();
  expect(screen.queryByText("New title")).not.toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  // a second edit uses the new version
  await edit("Third title");
  await waitFor(() => expect(f.mock.calls.filter((c) => (c[1] as RequestInit | undefined)?.method === "PUT")).toHaveLength(2));
  const puts = f.mock.calls.filter((c) => (c[1] as RequestInit | undefined)?.method === "PUT");
  expect(new Headers((puts[1][1] as RequestInit).headers).get("If-Match")).toBe('"2"');
});

it("hidden12: 409 rolls back the title and shows a conflict toast", async () => {
  setup(() => Promise.resolve(new Response(JSON.stringify({ error: "version conflict" }), { status: 409 })));
  render(<App />);
  await edit("New title");
  const alert = await screen.findByRole("alert");
  expect(within(alert).getByText(/conflict/i)).toBeInTheDocument();
  await waitFor(() => expect(screen.getByText("Old title")).toBeInTheDocument());
  expect(screen.queryByText("New title")).not.toBeInTheDocument();
});

it("hidden12: other failures also roll back and show a toast", async () => {
  setup(() => Promise.resolve(new Response("{}", { status: 500 })));
  render(<App />);
  await edit("New title");
  expect(await screen.findByRole("alert")).toBeInTheDocument();
  await waitFor(() => expect(screen.getByText("Old title")).toBeInTheDocument());
  expect(screen.queryByText("New title")).not.toBeInTheDocument();
});
