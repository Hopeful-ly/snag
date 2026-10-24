import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, settled } from "./fake-api";
import { button, renderApp } from "./render";

const sheet = () => screen.getByRole("dialog", { name: "Connect your browser" });

function firstRun() {
  const api = new FakeApi(settled());
  api.setupValue = { ...api.setupValue, connectedBefore: false };
  return api;
}

describe("Connect your browser", () => {
  it("opens by itself until the extension has connected once", async () => {
    await renderApp(firstRun());
    expect(sheet()).toBeInTheDocument();
    expect(within(sheet()).getByText("~/.local/share/snag/extension")).toBeInTheDocument();
    expect(within(sheet()).getByRole("status")).toHaveTextContent("Waiting for the extension…");
  });

  it("stays shut once it has", async () => {
    await renderApp();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("copies the address and the folder, and shows the folder", async () => {
    const { api, user } = await renderApp(firstRun());
    await user.click(button("Copy", sheet()));
    await user.click(button("Copy the folder's path", sheet()));
    await user.click(button("Show the folder", sheet()));
    await waitFor(() =>
      expect(api.calls).toEqual(
        expect.arrayContaining([
          "copyText chrome://extensions",
          "copyText /home/me/.local/share/snag/extension",
          "showExtensionFolder",
        ]),
      ),
    );
    expect(await screen.findByText("Copied the folder's path")).toBeInTheDocument();
  });

  it("opens each browser found on its extensions page", async () => {
    const { api, user } = await renderApp(firstRun());
    await user.click(await within(sheet()).findByRole("button", { name: "Vivaldi" }));
    await user.click(button("Brave", sheet()));
    await waitFor(() =>
      expect(api.calls).toEqual(
        expect.arrayContaining([
          "openExtensionsPage vivaldi:/usr/bin/vivaldi-stable",
          "openExtensionsPage brave:/usr/bin/brave",
        ]),
      ),
    );
  });

  it("says where to paste the address when it finds no browser", async () => {
    const api = firstRun();
    api.browsersValue = [];
    await renderApp(api);
    expect(
      within(sheet()).getByText("Paste this in your browser's address bar:"),
    ).toBeInTheDocument();
    expect(within(sheet()).queryByRole("button", { name: "Vivaldi" })).not.toBeInTheDocument();
  });

  it("turns green when the extension talks to Snag", async () => {
    const { api } = await renderApp(firstRun());
    api.connect();
    await waitFor(() => expect(within(sheet()).getByRole("status")).toHaveTextContent("Connected"));
  });

  it("sets the browsers up again", async () => {
    const { user } = await renderApp(firstRun());
    await user.click(button("Set up browsers again", sheet()));
    expect(await screen.findByText("Set up for Vivaldi, Firefox")).toBeInTheDocument();
  });

  it("opens from Settings, which also says whether it is connected", async () => {
    const { user } = await renderApp();
    await user.click(button("Settings"));
    const settings = screen.getByRole("region", { name: "Settings" });
    expect(within(settings).getByText("Not connected right now")).toBeInTheDocument();
    await user.click(within(settings).getByRole("button", { name: "Connect…" }));
    expect(sheet()).toBeInTheDocument();
    // Escape closes the sheet, not Settings behind it.
    await user.keyboard("{Escape}");
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    expect(screen.getByRole("region", { name: "Settings" })).toBeInTheDocument();
  });
});
