import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, settled } from "./fake-api";
import { button, renderApp } from "./render";

async function openSettings(api = new FakeApi(settled())) {
  const ctx = await renderApp(api);
  await ctx.user.click(button("Settings"));
  return { ...ctx, sheet: screen.getByRole("region", { name: "Settings" }) };
}

const lastSaved = (api: FakeApi) => {
  const call = [...api.calls].reverse().find((c) => c.startsWith("setSettings "));
  return call ? JSON.parse(call.slice("setSettings ".length)) : null;
};

describe("Settings", () => {
  it("shows the settings", async () => {
    const { sheet } = await openSettings();
    expect(within(sheet).getByText("~/Downloads")).toBeInTheDocument();
    expect(
      within(sheet).getByRole("switch", { name: "Ask where to save each download" }),
    ).toBeChecked();
    expect(
      within(sheet).getByRole("group", { name: "Connections per download" }),
    ).toHaveTextContent("8");
  });

  it("takes a TMDB key, trimmed, once typed", async () => {
    const { api, user, sheet } = await openSettings();
    const key = within(sheet).getByLabelText("TMDB key");
    await user.type(key, " abc123 ");
    await user.tab();
    await waitFor(() => expect(lastSaved(api)).toMatchObject({ tmdbKey: "abc123" }));
  });

  it("saves each change at once, spinning meanwhile", async () => {
    const { api, user, sheet } = await openSettings();
    api.holding.add("setSettings");
    const ask = within(sheet).getByRole("switch", { name: "Ask where to save each download" });
    await user.click(ask);
    expect(ask).toHaveAttribute("aria-busy", "true");
    api.release();
    await waitFor(() => expect(ask).not.toBeChecked());
    expect(lastSaved(api)).toMatchObject({ ask: false, connections: 8 });

    const connections = within(sheet).getByRole("group", { name: "Connections per download" });
    await user.click(within(connections).getByRole("button", { name: "More" }));
    await waitFor(() => expect(lastSaved(api)).toMatchObject({ connections: 9 }));

    const atOnce = within(sheet).getByRole("group", { name: "Downloads at once" });
    await user.click(within(atOnce).getByRole("button", { name: "Fewer" }));
    await waitFor(() => expect(lastSaved(api)).toMatchObject({ maxActive: 2 }));

    await user.click(
      within(sheet).getByRole("switch", { name: "Notify when a download finishes" }),
    );
    await waitFor(() => expect(lastSaved(api)).toMatchObject({ notify: false }));
  });

  it("changes how playlist files are named", async () => {
    const { api, user, sheet } = await openSettings();
    const songs = within(sheet).getByRole("textbox", { name: "Songs" });
    await user.clear(songs);
    await user.type(songs, "{{album} - {{title}");
    await user.tab();
    await waitFor(() =>
      expect(lastSaved(api)).toMatchObject({ naming: { songs: "{album} - {title}" } }),
    );
  });

  it("changes the download folder with the system's dialog", async () => {
    const api = new FakeApi(settled());
    api.folderPick = "/mnt/media";
    const { user, sheet } = await openSettings(api);
    await user.click(within(sheet).getByRole("button", { name: "Change…" }));
    await waitFor(() => expect(within(sheet).getByText("/mnt/media")).toBeInTheDocument());
    expect(lastSaved(api)).toMatchObject({ downloadDir: "/mnt/media" });
  });

  it("stops at the limits", async () => {
    const api = new FakeApi(settled());
    api.settingsValue.maxActive = 1;
    const { sheet } = await openSettings(api);
    const atOnce = within(sheet).getByRole("group", { name: "Downloads at once" });
    expect(within(atOnce).getByRole("button", { name: "Fewer" })).toBeDisabled();
  });

  it("follows changes made elsewhere, and closes with Escape", async () => {
    const { user, sheet } = await openSettings();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(sheet).not.toBeInTheDocument());
    expect(screen.getByRole("listbox", { name: "Downloads" })).toBeInTheDocument();
  });

  it("is a screen of its own, in place of the downloads", async () => {
    const { user, sheet } = await openSettings();
    expect(screen.queryByRole("listbox", { name: "Downloads" })).toBeNull();
    expect(screen.queryByRole("dialog")).toBeNull();
    for (const section of ["Downloads", "Naming playlist files", "Snag", "About"])
      expect(within(sheet).getByRole("heading", { name: section })).toBeInTheDocument();

    await user.click(button("Back to downloads (Esc)", sheet));
    expect(screen.queryByRole("region", { name: "Settings" })).toBeNull();
    expect(screen.getByRole("listbox", { name: "Downloads" })).toBeInTheDocument();

    // Going anywhere in the sidebar leaves Settings too.
    await user.click(button("Settings"));
    await user.click(
      within(screen.getByRole("navigation", { name: "Downloads" })).getByRole("button", {
        name: /^Finished/,
      }),
    );
    expect(screen.queryByRole("region", { name: "Settings" })).toBeNull();
  });

  it("turns starting at login off and on", async () => {
    const api = new FakeApi(settled());
    const { user, sheet } = await openSettings(api);
    const toggle = within(sheet).getByRole("switch", { name: "Start Snag when I log in" });
    await waitFor(() => expect(toggle).toBeChecked());
    await user.click(toggle);
    await waitFor(() => expect(toggle).not.toBeChecked());
    expect(api.calls).toContain("setStartAtLogin false");
    await user.click(toggle);
    await waitFor(() => expect(toggle).toBeChecked());
  });

  it("doesn't offer starting at login where Snag can't", async () => {
    const api = new FakeApi(settled());
    api.startAtLoginValue = { on: false, available: false, desktop: "", command: "" };
    const { sheet } = await openSettings(api);
    await waitFor(() => expect(api.calls).toContain("settings"));
    expect(within(sheet).queryByRole("switch", { name: "Start Snag when I log in" })).toBeNull();
  });

  it("says what to add on a desktop that ignores autostart, like Hyprland", async () => {
    const api = new FakeApi(settled());
    api.startAtLoginValue = {
      on: true,
      available: true,
      desktop: "Hyprland",
      command: "/home/me/.local/lib/snag/snag --at-login",
    };
    const { user, sheet } = await openSettings(api);
    const note = await within(sheet).findByRole("note");
    expect(note).toHaveTextContent("Hyprland doesn't start what other desktops do at login");
    expect(note).toHaveTextContent("/home/me/.local/lib/snag/snag --at-login");
    await user.click(within(note).getByRole("button", { name: "Copy the command" }));
    await waitFor(() =>
      expect(api.calls).toContain("copyText /home/me/.local/lib/snag/snag --at-login"),
    );
  });

  it("names its author, with a link to their page", async () => {
    const api = new FakeApi(settled());
    const { user, sheet } = await openSettings(api);
    expect(within(sheet).queryByRole("note")).toBeNull();
    await user.click(await within(sheet).findByRole("button", { name: "Roham1299" }));
    expect(api.calls).toContain("openAuthorPage");
    expect(within(sheet).queryByText("github.com/Roham1299")).toBeNull();
  });

  it("says when a newer Snag is out, and opens its page", async () => {
    const api = new FakeApi(settled());
    api.updateValue = { version: "1.2.0" };
    const { user, sheet } = await openSettings(api);
    await user.click(await within(sheet).findByRole("button", { name: "get it" }));
    expect(api.calls).toContain("openUpdatePage");
    expect(within(sheet).getByText(/is out/)).toHaveTextContent("Snag 1.2.0 is out: get it");
  });

  it("says nothing of updates while this Snag is the latest", async () => {
    const { sheet } = await openSettings();
    await within(sheet).findByRole("button", { name: "Roham1299" });
    expect(within(sheet).queryByText(/is out/)).toBeNull();
  });
});
