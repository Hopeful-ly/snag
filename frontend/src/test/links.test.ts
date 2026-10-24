import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import LinksWindow from "../LinksWindow.svelte";
import { FakeApi, sampleQueues, settled } from "./fake-api";
import { button, renderApp } from "./render";
import { matches } from "../lib/links";

async function openLinksWindow(api = new FakeApi(settled())) {
  const user = userEvent.setup();
  render(LinksWindow, { props: { api, id: "l1" } });
  await screen.findByRole("heading", { name: "Example downloads" });
  return { api, user, main: screen.getByRole("main", { name: "Download links" }) };
}

const list = () => within(screen.getByRole("group", { name: "Links" }));
const row = (name: string) => list().getByRole("checkbox", { name });
const shown = () =>
  list()
    .getAllByRole("checkbox")
    .map((box) => box.getAttribute("aria-label"));
const ticked = () =>
  list()
    .getAllByRole("checkbox")
    .filter((box) => box.getAttribute("aria-checked") === "true")
    .map((box) => box.getAttribute("aria-label"));

describe("the Download links window", () => {
  it("ticks the files, hides web pages, and learns what links are", async () => {
    const { main } = await openLinksWindow();
    expect(within(main).getByText("example.org · 7 links")).toBeInTheDocument();
    // The link without a name turns out to be a PDF, and is ticked once Snag knows.
    await waitFor(() => expect(row("Manual.pdf")).toBeInTheDocument());
    expect(shown()).toEqual([
      "tool-1.0.zip",
      "distro.iso",
      "setup.exe",
      "Manual.pdf",
      "The launch video",
      "logo.png",
    ]);
    expect(ticked()).toEqual(["tool-1.0.zip", "distro.iso", "setup.exe", "Manual.pdf"]);
    expect(within(row("tool-1.0.zip")).getByText("12 MB")).toBeInTheDocument();
    expect(within(row("The launch video")).getByText("Video pages")).toBeInTheDocument();
    expect(within(row("distro.iso")).getByText("iso/")).toBeInTheDocument();
    expect(button("Download 4", main)).toBeEnabled();
  });

  it("ticks a group at once, and filters", async () => {
    const { user, main } = await openLinksWindow();
    await waitFor(() => expect(row("Manual.pdf")).toBeInTheDocument());
    await user.click(button("Archives 2", main));
    expect(ticked()).toEqual(["setup.exe", "Manual.pdf"]);
    await user.click(button("Images 1", main));
    expect(ticked()).toContain("logo.png");

    await user.type(screen.getByRole("textbox", { name: "Filter" }), "*.pdf");
    expect(shown()).toEqual(["Manual.pdf"]);
    expect(button("Download 1", main)).toBeEnabled();
    await user.clear(screen.getByRole("textbox", { name: "Filter" }));

    await user.click(screen.getByRole("checkbox", { name: "Web pages" }));
    expect(shown()).toContain("about.html");
  });

  it("downloads the links picked into the folder and queue chosen, with the site's folders", async () => {
    const api = new FakeApi(settled());
    api.queuesValue = sampleQueues();
    const { user, main } = await openLinksWindow(api);
    await waitFor(() => expect(row("Manual.pdf")).toBeInTheDocument());
    await user.click(button("None", main));
    await user.click(row("distro.iso"));
    await user.click(row("The launch video"));
    await user.click(screen.getByRole("radio", { name: "Tonight" }));
    await user.click(button("Download 2", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'downloadLinks l1 {"entries":[1,4],"directory":"/home/me/Downloads","queue":"night","keepFolders":true}',
      ),
    );
  });

  it("marks and leaves out what Snag has, and can hide it", async () => {
    const api = new FakeApi(settled());
    api.linksHaveValue = [true, false, false, false, false, false, false];
    const { user } = await openLinksWindow(api);
    expect(within(row("tool-1.0.zip")).getByText("In Snag")).toBeInTheDocument();
    expect(ticked()).not.toContain("tool-1.0.zip");
    await user.click(screen.getByRole("checkbox", { name: "Hide what Snag has" }));
    expect(shown()).not.toContain("tool-1.0.zip");
  });
});

describe("pasting several links", () => {
  it("opens the Download links window for them", async () => {
    const api = new FakeApi(settled());
    api.clipboard = "https://example.org/a.zip\nhttps://example.org/b.zip";
    const { user } = await renderApp(api);
    await user.click(button("Add a link (Ctrl+V)"));
    const field = screen.getByRole("textbox", { name: "Link" });
    await waitFor(() =>
      expect(field).toHaveValue("https://example.org/a.zip https://example.org/b.zip"),
    );
    await user.click(button("Choose…"));
    await waitFor(() =>
      expect(api.calls).toContain("addLinks https://example.org/a.zip https://example.org/b.zip"),
    );
    expect(screen.queryByRole("textbox", { name: "Link" })).not.toBeInTheDocument();
  });

  it("imports links from a file", async () => {
    const api = new FakeApi(settled());
    api.importValue = "l1";
    const { user } = await renderApp(api);
    await user.click(button("Add a link (Ctrl+V)"));
    await user.click(button("Import links from a file…"));
    await waitFor(() => expect(api.calls).toContain("importLinks"));
    await waitFor(() =>
      expect(screen.queryByRole("textbox", { name: "Link" })).not.toBeInTheDocument(),
    );
  });
});

describe("filtering links", () => {
  const l = {
    url: "https://e.org/pub/ubuntu-24.04-amd64.iso",
    name: "ubuntu-24.04-amd64.iso",
    text: "Ubuntu",
  };
  it.each([
    ["", true],
    ["ubuntu", true],
    ["ubuntu amd64", true],
    ["ubuntu arm64", false],
    ["*.iso", true],
    ["*.zip", false],
    ["ubuntu*amd64*", true],
    ["e.org/pub", true],
  ])("%s", (filter, want) => {
    expect(matches(l, filter)).toBe(want);
  });
});
