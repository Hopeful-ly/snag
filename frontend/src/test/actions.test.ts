import { describe, expect, it } from "vitest";
import { screen, waitFor, within } from "@testing-library/svelte";
import { button, renderApp, row, settle } from "./render";

const iso = "ubuntu-26.04-desktop-amd64.iso";
const lisbon = "Holiday-in-Lisbon.mp4";
const nivalis = "Nivalis-Nights.zip";
const report = "report_2026.pdf";

describe("a download's buttons", () => {
  it("pause a running download, spinning until Snag answers", async () => {
    const { api, user } = await renderApp();
    api.holding.add("pause");
    await user.click(button("Pause", row(iso)));

    expect(button("Pause", row(iso))).toHaveAttribute("aria-busy", "true");
    expect(button("Show in folder", row(iso))).toBeDisabled(); // the row's other buttons wait
    api.release();
    await waitFor(() => expect(button("Resume", row(iso))).toBeEnabled());
    expect(api.calls).toContain("pause running");
  });

  it.each([
    [lisbon, "Resume", "resume paused"],
    [nivalis, "Try again", "resume failed"],
    [report, "Open file", "open done"],
    [report, "Show in folder", "reveal done"],
    [report, "Remove from list (keeps the file)", "remove done false"],
    [iso, "Cancel and remove", "remove running false"],
  ])("%s: %s", async (name, label, call) => {
    const { api, user } = await renderApp();
    await user.click(button(label, row(name)));
    await waitFor(() => expect(api.calls).toContain(call));
  });

  it("double-click opens a finished file, else shows it in its folder", async () => {
    const { api, user } = await renderApp();
    await user.dblClick(row(report));
    await user.dblClick(row(lisbon));
    await waitFor(() =>
      expect(api.calls).toEqual(expect.arrayContaining(["open done", "reveal paused"])),
    );
  });

  it("shows what went wrong", async () => {
    const { api, user } = await renderApp();
    api.failing.set("open", "the file is gone");
    await user.click(button("Open file", row(report)));
    expect(await screen.findByRole("alert")).toHaveTextContent("the file is gone");
    expect(button("Open file", row(report))).toBeEnabled();
  });
});

describe("a download's menu", () => {
  async function openMenu(name: string) {
    const ctx = await renderApp();
    await ctx.user.click(button("More", row(name)));
    const menu = screen.getByRole("menu");
    const items = within(menu)
      .getAllByRole("menuitem")
      .map((item) => item.textContent?.trim());
    return { ...ctx, menu, items };
  }

  it.each([
    [
      report,
      [
        "Open",
        "Details",
        "Show in folder",
        "Copy link",
        "Check checksum…",
        "Remove from list",
        "Delete file",
      ],
    ],
    [iso, ["Details", "Show in folder", "Copy link", "Pause", "Cancel and remove"]],
    [lisbon, ["Details", "Show in folder", "Copy link", "Resume", "Cancel and remove"]],
    [nivalis, ["Details", "Show in folder", "Copy link", "Try again", "Cancel and remove"]],
  ])("offers what fits %s", async (name, expected) => {
    const { items } = await openMenu(name);
    expect(items).toEqual(expected);
  });

  it("copies the link", async () => {
    const { api, user, menu } = await openMenu(iso);
    await user.click(within(menu).getByRole("menuitem", { name: "Copy link" }));
    expect(await screen.findByText("Link copied")).toBeInTheDocument();
    expect(api.calls).toContain("copyLink running");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("deletes a finished file", async () => {
    const { api, user, menu } = await openMenu(report);
    await user.click(within(menu).getByRole("menuitem", { name: "Delete file" }));
    expect(await screen.findByText(`Deleted ${report}`)).toBeInTheDocument();
    expect(api.calls).toContain("remove done true");
  });

  it("opens on a right click and closes with Escape", async () => {
    const { user } = await renderApp();
    await user.pointer({ keys: "[MouseRight]", target: row(lisbon) });
    expect(screen.getByRole("menu")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });
});

describe("everything at once", () => {
  it("pauses all, resumes all, and clears finished ones", async () => {
    const { api, user } = await renderApp();
    await user.click(button("Pause all"));
    await user.click(button("Resume all"));
    await user.click(button("Clear finished from the list (keeps the files)"));
    expect(await screen.findByText("Cleared finished downloads")).toBeInTheDocument();
    expect(api.calls).toEqual(expect.arrayContaining(["pauseAll", "resumeAll", "clearDone"]));
    await waitFor(() =>
      expect(screen.queryByRole("option", { name: report })).not.toBeInTheDocument(),
    );
    expect(button("Clear finished from the list (keeps the files)")).toBeDisabled();
  });
});

describe("keys", () => {
  it("select with the arrows, pause with Space, open with Enter, remove with Delete", async () => {
    const { api, user } = await renderApp();
    await user.keyboard("{ArrowDown}");
    expect(row(iso)).toHaveAttribute("aria-selected", "true");
    await user.keyboard(" ");
    await waitFor(() => expect(api.calls).toContain("pause running"));

    await user.keyboard("{ArrowDown}{ArrowDown}{ArrowDown}");
    expect(row(report)).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{Enter}");
    await user.keyboard("{Delete}");
    await waitFor(() =>
      expect(api.calls).toEqual(expect.arrayContaining(["open done", "remove done false"])),
    );
  });

  it("Ctrl+F searches, Escape clears it, then the selection", async () => {
    const { user } = await renderApp();
    await user.keyboard("{Control>}f{/Control}");
    expect(screen.getByRole("textbox", { name: "Search" })).toHaveFocus();
    await user.keyboard("iso{Escape}");
    expect(screen.getByRole("textbox", { name: "Search" })).toHaveValue("");

    await user.click(row(report));
    expect(screen.getByRole("complementary", { name: `Details of ${report}` })).toBeInTheDocument();
    await user.keyboard("{Escape}"); // the details first
    await settle();
    expect(screen.queryByRole("complementary")).toBeNull();
    expect(row(report)).toHaveAttribute("aria-selected", "true");
    await user.keyboard("{Escape}");
    await settle();
    expect(row(report)).toHaveAttribute("aria-selected", "false");
  });
});
