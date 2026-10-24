import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, sampleSubscriptions, settled, subscriptionDownloads } from "./fake-api";
import { button, renderApp } from "./render";

function withSubscriptions() {
  const api = new FakeApi([...subscriptionDownloads(), ...settled()]);
  api.subscriptionsValue = sampleSubscriptions();
  return api;
}

const nav = () => screen.getByRole("navigation", { name: "Downloads" });
const page = () => screen.getByRole("region", { name: "Subscriptions" });
const following = () => within(page()).getByRole("list", { name: "Following" });

async function openPage(user: Awaited<ReturnType<typeof renderApp>>["user"]) {
  await user.click(await within(nav()).findByRole("button", { name: /^All subscriptions/ }));
  return page();
}

describe("subscriptions in the sidebar", () => {
  it("lists what Snag follows, with what each did", async () => {
    await renderApp(withSubscriptions());
    await waitFor(() => expect(within(nav()).getByText("Subscriptions")).toBeInTheDocument());
    expect(
      within(nav()).getByRole("button", { name: "Lanterns Studio: 2 new, 2 h ago" }),
    ).toBeInTheDocument();
    expect(
      within(nav()).getByRole("button", { name: "The Harbour Hour: Can't read it" }),
    ).toBeInTheDocument();
    // The Harbour Hour says it can't be read; All subscriptions needn't say it again.
    expect(within(nav()).getByRole("button", { name: "All subscriptions" })).toBeInTheDocument();
  });

  it("offers following something when nothing is followed", async () => {
    const { user } = await renderApp(new FakeApi(settled()));
    await user.click(
      within(nav()).getByRole("button", { name: "Follow a channel…: Or a playlist, or a podcast" }),
    );
    expect(within(page()).getByText("Nothing followed yet")).toBeInTheDocument();
  });

  it("shows what a subscription downloaded, with its actions over the list", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    await user.click(await within(nav()).findByRole("button", { name: /^Lanterns Studio/ }));
    expect(screen.getByRole("heading", { name: "Lanterns Studio" })).toBeInTheDocument();
    expect(screen.getAllByRole("option").map((o) => o.getAttribute("aria-label"))).toEqual([
      expect.stringContaining("Paper lanterns at dusk"),
      expect.stringContaining("Folding a lantern"),
    ]);
    await user.click(button("Check now"));
    await waitFor(() => expect(api.calls).toContain("checkSubscription s1"));
    await user.click(button("Edit"));
    expect(screen.getByRole("dialog", { name: "Lanterns Studio" })).toBeInTheDocument();
  });

  it("checks now, pauses and unsubscribes from the menu", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    const row = await within(nav()).findByRole("button", { name: /^Lanterns Studio/ });

    await user.pointer({ keys: "[MouseRight]", target: row });
    await user.click(screen.getByRole("menuitem", { name: "Check now" }));
    await waitFor(() => expect(api.calls).toContain("checkSubscription s1"));
    await waitFor(() =>
      expect(
        within(nav()).getByRole("button", { name: "Lanterns Studio: Looking for new ones…" }),
      ).toBeInTheDocument(),
    );

    await user.pointer({
      keys: "[MouseRight]",
      target: within(nav()).getByRole("button", { name: /^The Harbour Hour/ }),
    });
    await user.click(screen.getByRole("menuitem", { name: "Pause" }));
    await waitFor(() =>
      expect(
        within(nav()).getByRole("button", { name: "The Harbour Hour: Paused" }),
      ).toBeInTheDocument(),
    );

    await user.pointer({
      keys: "[MouseRight]",
      target: within(nav()).getByRole("button", { name: /^The Harbour Hour/ }),
    });
    await user.click(screen.getByRole("menuitem", { name: "Open its page" }));
    await waitFor(() => expect(api.calls).toContain("openSubscriptionPage s2"));

    await user.pointer({
      keys: "[MouseRight]",
      target: within(nav()).getByRole("button", { name: /^The Harbour Hour/ }),
    });
    await user.click(screen.getByRole("menuitem", { name: "Unsubscribe" }));
    expect(
      await screen.findByText("Unsubscribed from The Harbour Hour; what it downloaded stays"),
    ).toBeInTheDocument();
    expect(within(nav()).queryByRole("button", { name: /^The Harbour Hour/ })).toBeNull();
  });

  it("goes back to everything when the subscription shown is unsubscribed from", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    await user.click(await within(nav()).findByRole("button", { name: /^Lanterns Studio/ }));
    await user.pointer({
      keys: "[MouseRight]",
      target: within(nav()).getByRole("button", { name: /^Lanterns Studio/ }),
    });
    await user.click(screen.getByRole("menuitem", { name: "Unsubscribe" }));
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "All downloads" })).toBeInTheDocument(),
    );
    // What it downloaded stays in the list.
    expect(screen.getByRole("option", { name: /Paper lanterns at dusk/ })).toBeInTheDocument();
  });

  it("keeps the sidebar short when many are followed", async () => {
    const api = new FakeApi(settled());
    const [one] = sampleSubscriptions();
    api.subscriptionsValue = Array.from({ length: 9 }, (_, i) => ({
      ...one!,
      id: `s${i}`,
      title: `Channel ${i}`,
      error: i % 2 ? "the site answered 500" : "",
    }));
    await renderApp(api);
    await waitFor(() =>
      expect(within(nav()).getByRole("button", { name: /^Channel 0/ })).toBeInTheDocument(),
    );
    expect(within(nav()).getAllByRole("button", { name: /^Channel / })).toHaveLength(5);
    // Of those it has no room for, it says which can't be read.
    expect(
      within(nav()).getByRole("button", { name: "All subscriptions: 2 can't be read" }),
    ).toHaveTextContent("9");
  });
});

describe("the subscriptions screen", () => {
  it("shows each subscription: what it is, how it downloads, what it did", async () => {
    const { user } = await renderApp(withSubscriptions());
    await openPage(user);
    expect(page()).toHaveTextContent("2 subscriptions");
    expect(page()).toHaveTextContent("1 can't be read");
    const lanterns = within(following()).getByRole("listitem", { name: "Lanterns Studio" });
    expect(lanterns).toHaveTextContent("YouTube channel");
    expect(lanterns).toHaveTextContent("Every 6 hours");
    expect(lanterns).toHaveTextContent("~/Downloads/Lanterns");
    expect(lanterns).toHaveTextContent(
      "Looked 2 h ago, next in 4 h, 2 new last time, 5 downloaded",
    );
    const harbour = within(following()).getByRole("listitem", { name: "The Harbour Hour" });
    expect(harbour).toHaveTextContent("Podcast");
    expect(harbour).toHaveTextContent("Can't read it lately: the feed's site answered 500");
  });

  it("checks, pauses and edits from a subscription's row", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    await openPage(user);
    const harbour = () => within(following()).getByRole("listitem", { name: "The Harbour Hour" });

    await user.click(button("Pause: stop looking for now", harbour()));
    await waitFor(() => expect(harbour()).toHaveTextContent("paused"));
    await user.click(button("Resume: look for new ones again", harbour()));
    await waitFor(() => expect(harbour()).not.toHaveTextContent("paused"));

    await user.click(button("Check now", harbour()));
    await waitFor(() => expect(api.calls).toContain("checkSubscription s2"));

    await user.click(button("Edit…", harbour()));
    expect(screen.getByRole("dialog", { name: "The Harbour Hour" })).toBeInTheDocument();
  });

  it("shows a subscription's downloads from its row", async () => {
    const { user } = await renderApp(withSubscriptions());
    await openPage(user);
    const lanterns = within(following()).getByRole("listitem", { name: "Lanterns Studio" });
    await user.click(button("Show what it downloaded", lanterns));
    expect(screen.queryByRole("region", { name: "Subscriptions" })).toBeNull();
    expect(screen.getByRole("heading", { name: "Lanterns Studio" })).toBeInTheDocument();
    expect(screen.getAllByRole("option")).toHaveLength(2);
  });

  it("checks all that aren't paused", async () => {
    const api = withSubscriptions();
    api.subscriptionsValue[1]!.paused = true;
    const { user } = await renderApp(api);
    await openPage(user);
    await user.click(button("Check all", page()));
    await waitFor(() => expect(api.calls).toContain("checkSubscription s1"));
    expect(api.calls).not.toContain("checkSubscription s2");
  });

  it("follows something new from its address", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    await user.click(await within(nav()).findByRole("button", { name: "Subscribe to…" }));
    const field = within(page()).getByRole("textbox", { name: "Address to subscribe to" });
    expect(field).toHaveFocus();
    expect(button("Subscribe…", page())).toBeDisabled();
    await user.type(field, "https://www.youtube.com/@harbourhour{Enter}");
    await waitFor(() =>
      expect(api.calls).toContain("findSubscription https://www.youtube.com/@harbourhour"),
    );
    await waitFor(() => expect(field).toHaveValue(""));
  });

  it("says why when there's nothing to follow at the address", async () => {
    const api = withSubscriptions();
    api.failing.set("findSubscription", "Snag found no channel, playlist or podcast there");
    const { user } = await renderApp(api);
    await openPage(user);
    const field = within(page()).getByRole("textbox", { name: "Address to subscribe to" });
    await user.type(field, "https://example.org/about");
    await user.click(button("Subscribe…", page()));
    expect(
      await screen.findByText("Snag found no channel, playlist or podcast there"),
    ).toBeInTheDocument();
    expect(field).toHaveValue("https://example.org/about");
  });

  it("finds one among many", async () => {
    const api = new FakeApi(settled());
    const [one] = sampleSubscriptions();
    api.subscriptionsValue = Array.from({ length: 8 }, (_, i) => ({
      ...one!,
      id: `s${i}`,
      title: i === 3 ? "Woodworking weekly" : `Channel ${i}`,
    }));
    const { user } = await renderApp(api);
    await openPage(user);
    await user.type(within(page()).getByRole("textbox", { name: "Find a subscription" }), "wood");
    expect(
      within(following())
        .getAllByRole("listitem")
        .map((r) => r.ariaLabel),
    ).toEqual(["Woodworking weekly"]);
  });

  it("goes back to the downloads with Escape", async () => {
    const { user } = await renderApp(withSubscriptions());
    await openPage(user);
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("region", { name: "Subscriptions" })).toBeNull();
  });
});

describe("a subscription's sheet", () => {
  it("edits how often, the quality and what to leave out", async () => {
    const api = withSubscriptions();
    const { user } = await renderApp(api);
    await openPage(user);
    await user.click(within(following()).getByRole("button", { name: "Lanterns Studio" }));
    const sheet = screen.getByRole("dialog", { name: "Lanterns Studio" });
    await user.click(within(sheet).getByRole("radio", { name: "Every day" }));
    await user.click(within(sheet).getByRole("radio", { name: "720p" }));
    await user.type(within(sheet).getByRole("textbox", { name: "No titles with" }), "trailer");
    await user.click(within(sheet).getByRole("checkbox", { name: "No Shorts" }));
    await user.click(button("Save", sheet));
    await waitFor(() => expect(screen.queryByRole("dialog")).toBeNull());
    const saved = api.subscriptionsValue.find((s) => s.id === "s1")!;
    expect(saved.everyHours).toBe(24);
    expect(saved.quality).toBe("720");
    expect(saved.filters).toEqual({ exclude: "trailer", noShorts: true });
  });

  it("says subscriptions need Snag running when it doesn't start at login", async () => {
    const api = withSubscriptions();
    api.startAtLoginValue = { on: false, available: true, desktop: "", command: "" };
    const { user } = await renderApp(api);
    await openPage(user);
    await waitFor(() =>
      expect(page()).toHaveTextContent("Snag checks for new ones only while it's running"),
    );
    await user.click(within(following()).getByRole("button", { name: "Lanterns Studio" }));
    const sheet = screen.getByRole("dialog", { name: "Lanterns Studio" });
    await waitFor(() =>
      expect(sheet).toHaveTextContent("Snag checks for new ones only while it's running"),
    );
    await user.click(button("Turn it on", sheet));
    await waitFor(() => expect(api.calls).toContain("setStartAtLogin true"));
    await waitFor(() =>
      expect(sheet).not.toHaveTextContent("Snag checks for new ones only while it's running"),
    );
  });
});
