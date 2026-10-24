import { describe, expect, it } from "vitest";
import { screen, waitFor } from "@testing-library/svelte";
import { FakeApi, settled } from "./fake-api";
import { button, renderApp } from "./render";

const field = () => screen.getByRole("textbox", { name: "Link" });

describe("adding a link", () => {
  it("opens with the link on the clipboard, and adds it", async () => {
    const api = new FakeApi(settled());
    api.clipboard = "https://files.example.com/new-album.zip";
    const { user } = await renderApp(api);

    await user.click(button("Add a link (Ctrl+V)"));
    await waitFor(() => expect(field()).toHaveValue("https://files.example.com/new-album.zip"));
    await user.click(button("Download"));

    expect(await screen.findByText("Downloading new-album.zip")).toBeInTheDocument();
    expect(api.calls).toContain("add https://files.example.com/new-album.zip");
    expect(screen.queryByRole("textbox", { name: "Link" })).not.toBeInTheDocument();
    expect(screen.getByRole("option", { name: "new-album.zip" })).toBeInTheDocument();
  });

  it("leaves a playlist's link to the playlist window", async () => {
    const api = new FakeApi(settled());
    api.playlistLinks.add("https://v.example.com/lanterns/1");
    const { user } = await renderApp(api);
    await user.click(button("Add a link (Ctrl+V)"));
    await user.type(field(), "https://v.example.com/lanterns/1{Enter}");
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Link" })).not.toBeInTheDocument(),
    );
    expect(api.calls).toContain("add https://v.example.com/lanterns/1");
    expect(screen.queryByText(/^Downloading/)).not.toBeInTheDocument();
    expect(api.downloads).toHaveLength(settled().length);
  });

  it("leaves the clipboard alone when it holds no link", async () => {
    const api = new FakeApi(settled());
    api.clipboard = "some words";
    const { user } = await renderApp(api);
    await user.keyboard("{Control>}n{/Control}");
    await waitFor(() => expect(field()).toHaveFocus());
    expect(field()).toHaveValue("");
    expect(button("Download")).toBeDisabled();
  });

  it("refuses what isn't a link", async () => {
    const { api, user } = await renderApp();
    await user.click(button("Add a link (Ctrl+V)"));
    await user.type(field(), "ftp://example.com/file{Enter}");
    expect(await screen.findByRole("alert")).toHaveTextContent("doesn't look like a link");
    expect(api.calls.some((c) => c.startsWith("add"))).toBe(false);
  });

  it("says when the link is already in the list", async () => {
    const { user } = await renderApp();
    await user.click(button("Add a link (Ctrl+V)"));
    await user.type(field(), "https://files.example.com/Holiday-in-Lisbon.mp4{Enter}");
    expect(await screen.findByText("Already in your list")).toBeInTheDocument();
  });

  it("pastes with its button, and closes with Escape", async () => {
    const { api, user } = await renderApp();
    await user.click(button("Add a link (Ctrl+V)"));
    api.clipboard = "  https://example.com/a.iso ";
    await user.click(button("Paste from clipboard"));
    expect(field()).toHaveValue("https://example.com/a.iso");
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("textbox", { name: "Link" })).not.toBeInTheDocument();
  });

  it("stays open with the link when adding fails", async () => {
    const { api, user } = await renderApp();
    api.failing.set("add", "can't reach server");
    await user.click(button("Add a link (Ctrl+V)"));
    await user.type(field(), "https://example.com/a.iso{Enter}");
    expect(await screen.findByRole("alert")).toHaveTextContent("can't reach server");
    expect(field()).toHaveValue("https://example.com/a.iso");
  });
});
