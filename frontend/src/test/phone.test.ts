import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { FakeApi, settled } from "./fake-api";
import { button, renderApp } from "./render";

async function openSettings(api = new FakeApi(settled())) {
  const ctx = await renderApp(api);
  await ctx.user.click(button("Settings"));
  const sheet = screen.getByRole("region", { name: "Settings" });
  await within(sheet).findByRole("switch", { name: "Send links from my phone" });
  return { ...ctx, sheet };
}

describe("Settings, Phone", () => {
  it("turns on, pairs a phone with a QR code, and removes it", async () => {
    const { api, user, sheet } = await openSettings();
    const toggle = within(sheet).getByRole("switch", { name: "Send links from my phone" });
    expect(toggle).not.toBeChecked();
    expect(within(sheet).queryByRole("button", { name: "Pair a phone…" })).not.toBeInTheDocument();

    await user.click(toggle);
    await waitFor(() => expect(toggle).toBeChecked());
    expect(api.calls).toContain("setPhone true");

    await user.click(within(sheet).getByRole("button", { name: "Pair a phone…" }));
    const qr = await within(sheet).findByRole("group", { name: "Pairing QR code" });
    expect(within(qr).getByRole("img", { name: "QR code" })).toBeInTheDocument();
    expect(qr).toHaveTextContent("192.168.1.5:41234/#pair=code1");

    // The phone opens the page: the code is used, the QR code goes, and the phone is listed.
    api.pairedPhone("iPhone, Safari");
    await waitFor(() =>
      expect(
        within(sheet).queryByRole("group", { name: "Pairing QR code" }),
      ).not.toBeInTheDocument(),
    );
    expect(await screen.findByText("Paired iPhone, Safari")).toBeInTheDocument();
    expect(within(sheet).getByText("iPhone, Safari")).toBeInTheDocument();

    await user.click(within(sheet).getByRole("button", { name: "Remove" }));
    await waitFor(() =>
      expect(within(sheet).queryByText("iPhone, Safari")).not.toBeInTheDocument(),
    );
    expect(api.calls).toContain("removePhone p1");
  });

  it("closes pairing when the QR code is closed, or it is turned off", async () => {
    const { api, user, sheet } = await openSettings();
    await user.click(within(sheet).getByRole("switch", { name: "Send links from my phone" }));
    await user.click(await within(sheet).findByRole("button", { name: "Pair a phone…" }));
    await user.click(await within(sheet).findByRole("button", { name: "Close" }));
    expect(within(sheet).queryByRole("group", { name: "Pairing QR code" })).not.toBeInTheDocument();
    expect(api.calls).toContain("stopPairing");

    await user.click(within(sheet).getByRole("button", { name: "Pair a phone…" }));
    await within(sheet).findByRole("group", { name: "Pairing QR code" });
    await user.click(within(sheet).getByRole("switch", { name: "Send links from my phone" }));
    await waitFor(() =>
      expect(
        within(sheet).queryByRole("button", { name: "Pair a phone…" }),
      ).not.toBeInTheDocument(),
    );
    expect(within(sheet).queryByRole("group", { name: "Pairing QR code" })).not.toBeInTheDocument();
  });

  it("says why it can't turn on away from a home network", async () => {
    const api = new FakeApi(settled());
    api.phoneNetwork = "";
    const { user, sheet } = await openSettings(api);
    const toggle = within(sheet).getByRole("switch", { name: "Send links from my phone" });
    await user.click(toggle);
    expect(
      await screen.findByText("this computer isn't on a home network Snag can listen on"),
    ).toBeInTheDocument();
    expect(toggle).not.toBeChecked();
  });

  it("says when the computer is on another network than the one it was turned on in", async () => {
    const api = new FakeApi(settled());
    api.phoneValue = { on: true, address: "", network: "192.168.1.0/24", devices: [] };
    const { sheet } = await openSettings(api);
    expect(within(sheet).getByRole("note")).toHaveTextContent(
      "isn't on the network sending from a phone was turned on in (192.168.1.0/24)",
    );
    expect(within(sheet).queryByRole("button", { name: "Pair a phone…" })).not.toBeInTheDocument();
  });
});
