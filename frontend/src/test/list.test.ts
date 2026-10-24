import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, sampleDownload, settled } from "./fake-api";
import { button, renderApp, row } from "./render";

const names = (): string[] =>
  screen.queryAllByRole("option").map((r) => r.getAttribute("aria-label") ?? "");

describe("the list", () => {
  it("shows every download, newest first, with its state", async () => {
    await renderApp();
    expect(names()).toEqual([
      "ubuntu-26.04-desktop-amd64.iso",
      "Holiday-in-Lisbon.mp4",
      "Nivalis-Nights.zip",
      "report_2026.pdf",
    ]);
    expect(within(row("ubuntu-26.04-desktop-amd64.iso")).getByText("41%")).toBeInTheDocument();
    expect(
      within(row("Nivalis-Nights.zip")).getByText("server said 404 Not Found"),
    ).toBeInTheDocument();
    expect(screen.getByText("4 downloads")).toBeInTheDocument();
    expect(screen.getByText("18 MB/s")).toBeInTheDocument();
    expect(screen.getByText("1 download running")).toBeInTheDocument();
  });

  it("filters by state and category, with counts", async () => {
    const { user } = await renderApp();
    const nav = screen.getByRole("navigation", { name: "Downloads" });
    expect(within(button("Unfinished 2", nav)).getByText("2")).toBeInTheDocument();

    await user.click(button("Finished 1", nav));
    expect(screen.getByRole("heading", { name: "Finished" })).toBeInTheDocument();
    expect(names()).toEqual(["report_2026.pdf"]);

    await user.click(button("Archives 2", nav));
    expect(names()).toEqual(["ubuntu-26.04-desktop-amd64.iso", "Nivalis-Nights.zip"]);

    await user.click(button("Music", nav));
    expect(screen.getByText("Nothing here")).toBeInTheDocument();
  });

  it("searches names and links", async () => {
    const { user } = await renderApp();
    await user.type(screen.getByRole("textbox", { name: "Search" }), "lisbon");
    expect(names()).toEqual(["Holiday-in-Lisbon.mp4"]);
    expect(screen.getByText("1 download matching “lisbon”")).toBeInTheDocument();

    await user.type(screen.getByRole("textbox", { name: "Search" }), "zzz");
    expect(screen.getByText("Nothing matches “lisbonzzz”")).toBeInTheDocument();
    await user.click(button("Clear search"));
    expect(names()).toHaveLength(4);
  });

  it("says when there is nothing yet, and offers to add a link", async () => {
    const { user } = await renderApp(new FakeApi([]));
    expect(screen.getByText("No downloads yet")).toBeInTheDocument();
    await user.click(button("Add a link"));
    expect(screen.getByRole("textbox", { name: "Link" })).toBeInTheDocument();
  });

  it("follows changes from the Go side", async () => {
    const { api } = await renderApp();
    const running = api.downloads.find((d) => d.id === "running");
    if (!running) throw new Error("no running sample");

    api.emit({ ...running, done: running.size, status: "done", finished: Date.now() });
    await waitFor(() =>
      expect(within(row(running.name)).getByText(/just now/)).toBeInTheDocument(),
    );

    api.emit(sampleDownload({ id: "fresh", name: "fresh.zip", status: "downloading" }));
    await waitFor(() => expect(names()[0]).toBe("fresh.zip"));

    api.emitRemoved("fresh");
    await waitFor(() => expect(names()).not.toContain("fresh.zip"));
  });

  it("replaces the list when many downloads arrive at once", async () => {
    const { api } = await renderApp();
    api.emitReset([
      sampleDownload({ id: "a", name: "one.mp3", status: "queued" }),
      sampleDownload({ id: "b", name: "two.mp3", status: "queued" }),
    ]);
    await waitFor(() => expect(names()).toEqual(["one.mp3", "two.mp3"]));
  });

  it("keeps retrying until the Go side is ready", async () => {
    const api = new FakeApi(settled());
    api.failing.set("list", "Snag is still starting");
    setTimeout(() => api.failing.delete("list"), 100);
    await renderApp(api);
    expect(names()).toHaveLength(4);
  });
});
