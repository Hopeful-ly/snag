import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, sampleDownload } from "./fake-api";
import { button, renderApp, row } from "./render";

const MB = 2 ** 20;

function downloads() {
  return [
    sampleDownload({
      id: "film",
      url: "https://cdn.example.com/film.mp4",
      name: "film.mp4",
      status: "downloading",
      size: 900 * MB,
      done: 40 * MB,
      speed: 6 * MB,
      watch: "ready",
    }),
    sampleDownload({
      id: "next",
      url: "https://cdn.example.com/next.mp4",
      name: "next.mp4",
      status: "downloading",
      size: 700 * MB,
      done: MB,
      speed: 80 * 1024,
      watch: "soon",
      watchIn: 12,
    }),
    sampleDownload({
      id: "iso",
      name: "ubuntu.iso",
      status: "downloading",
      size: 5000 * MB,
      done: 900 * MB,
    }),
  ];
}

describe("watching a video while it downloads", () => {
  it("opens a ready video in the player", async () => {
    const { api, user } = await renderApp(new FakeApi(downloads()));
    await user.click(button("Watch", row("film.mp4")));
    await waitFor(() => expect(api.calls).toContain("watch film"));
  });

  it("says a long wait in minutes", async () => {
    const list = downloads();
    list[1] = { ...list[1]!, watchIn: 103 };
    await renderApp(new FakeApi(list));
    expect(
      within(row("next.mp4")).getByRole("button", { name: "Watch in 1 min 43 s" }),
    ).toBeDisabled();
  });

  it("says when a video will be ready, and waits till then", async () => {
    await renderApp(new FakeApi(downloads()));
    const soon = within(row("next.mp4")).getByRole("button", { name: "Watch in 12 s" });
    expect(soon).toBeDisabled();
  });

  it("offers nothing to watch on files that aren't videos", async () => {
    await renderApp(new FakeApi(downloads()));
    expect(
      within(row("ubuntu.iso")).queryByRole("button", { name: /Watch/ }),
    ).not.toBeInTheDocument();
  });

  it("can be watched from the download's menu, in any player that plays streams", async () => {
    const { api, user } = await renderApp(new FakeApi(downloads()));
    await user.click(button("More", row("film.mp4")));
    await user.click(screen.getByRole("menuitem", { name: "Watch in Haruna" }));
    await waitFor(() => expect(api.calls).toContain("watch film"));

    await user.click(button("More", row("film.mp4")));
    await user.click(screen.getByRole("menuitem", { name: "Watch in mpv Media Player" }));
    await waitFor(() => expect(api.calls).toContain("watch film mpv.desktop"));
  });

  it("copies the stream address for any other player", async () => {
    const { api, user } = await renderApp(new FakeApi(downloads()));
    await user.click(button("More", row("film.mp4")));
    await user.click(screen.getByRole("menuitem", { name: "Copy stream address" }));
    expect(await screen.findByText(/Stream address copied/)).toBeInTheDocument();
    expect(api.calls).toContain("copyStreamAddress film");
  });

  it("offers no watching in the menu until a video is ready", async () => {
    const { user } = await renderApp(new FakeApi(downloads()));
    await user.click(button("More", row("next.mp4")));
    expect(
      screen.queryByRole("menuitem", { name: /Watch|stream address/ }),
    ).not.toBeInTheDocument();
  });

  it("shows why it couldn't open", async () => {
    const api = new FakeApi(downloads());
    api.failing.set("watch", "it hasn't started yet; try again in a moment");
    const { user } = await renderApp(api);
    await user.click(button("Watch", row("film.mp4")));
    expect(await screen.findByText(/hasn't started yet/)).toBeInTheDocument();
  });
});
