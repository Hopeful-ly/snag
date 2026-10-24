import { describe, expect, it } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import SaveWindow from "../SaveWindow.svelte";
import { FakeApi, samples } from "./fake-api";
import { button, renderApp, row } from "./render";
import { resizeAll } from "./setup";

async function openSaveWindow(api = new FakeApi(samples())) {
  const user = userEvent.setup();
  render(SaveWindow, { props: { api, id: "pending" } });
  await waitFor(() => expect(nameField()).toHaveValue("A Day at the Beach"));
  return { api, user, main: screen.getByRole("main", { name: "Save download" }) };
}

const nameField = () => screen.getByRole("textbox", { name: "Name" });
const folderField = () => screen.getByRole("textbox", { name: "Save to" });

describe("the Save window", () => {
  it("does not size the window until the form is shown", async () => {
    const api = new FakeApi(samples());
    api.holding.add("get");
    render(SaveWindow, { props: { api, id: "pending" } });
    await waitFor(() => expect(api.calls.some((call) => call.startsWith("get "))).toBe(true));
    expect(api.fitted).toEqual([]);

    api.release();
    await waitFor(() => expect(nameField()).toHaveValue("A Day at the Beach"));
    await waitFor(() => expect(api.fitted.length).toBeGreaterThan(0));
  });

  it("shows the download with its name and the default folder", async () => {
    const { main } = await openSaveWindow();
    expect(within(main).getByText(".mp4")).toBeInTheDocument(); // streams get their container
    expect(folderField()).toHaveValue("/home/me/Downloads");
    expect(within(main).getByText("~734 MB · example.com")).toBeInTheDocument();
    await waitFor(() => expect(nameField()).toHaveFocus());
  });

  it("downloads it where it was told, sending a new name only", async () => {
    const { api, user, main } = await openSaveWindow();
    await user.click(within(main).getByRole("button", { name: "~/Videos/Trips" }));
    await user.click(button("Download", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'confirm pending {"directory":"/home/me/Videos/Trips","filename":"","start":true,"remember":false,"replace":false,"queue":"main"}',
      ),
    );
  });

  it("renames, remembers the folder, and starts with Enter", async () => {
    const { api, user, main } = await openSaveWindow();
    await user.clear(nameField());
    await user.type(nameField(), "Beach day");
    await user.click(within(main).getByRole("switch", { name: "Always save here" }));
    await user.click(nameField());
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(api.calls).toContain(
        'confirm pending {"directory":"/home/me/Downloads","filename":"Beach day.mp4","start":true,"remember":true,"replace":false,"queue":"main"}',
      ),
    );
  });

  it("saves it for later", async () => {
    const { api, user, main } = await openSaveWindow();
    await user.click(button("Later", main));
    await waitFor(() =>
      expect(api.calls.some((c) => /^confirm pending .*"start":false/.test(c))).toBe(true),
    );
  });

  it("cancels it", async () => {
    const { api, user, main } = await openSaveWindow();
    await user.click(button("Cancel", main));
    await waitFor(() => expect(api.calls).toContain("remove pending false"));
  });

  it("picks a folder with the system's dialog", async () => {
    const api = new FakeApi(samples());
    api.folderPick = "/mnt/media/films";
    const { user, main } = await openSaveWindow(api);
    await user.click(button("Choose a folder…", main));
    await waitFor(() => expect(folderField()).toHaveValue("/mnt/media/films"));
  });

  it("spins until Snag answers, and shows a failure in place", async () => {
    const api = new FakeApi(samples());
    api.holding.add("confirm");
    api.failing.set("confirm", "can't write to /home/me/Downloads");
    const { user, main } = await openSaveWindow(api);
    await user.click(button("Download", main));
    expect(button("Download", main)).toHaveAttribute("aria-busy", "true");
    expect(button("Later", main)).toBeDisabled();

    api.release();
    expect(await screen.findByRole("alert")).toHaveTextContent("can't write");
    expect(button("Download", main)).toBeEnabled();
  });

  it("closes with Escape, leaving the download waiting", async () => {
    const { api, user } = await openSaveWindow();
    await user.keyboard("{Escape}");
    await waitFor(() => expect(api.calls).toContain("closeWindow"));
    expect(api.calls.some((c) => c.startsWith("remove") || c.startsWith("confirm"))).toBe(false);
  });
});

describe("the Save window's size", () => {
  // jsdom lays nothing out: the page is as tall as this says.
  const pageHeight = (height: number) => {
    const main = screen.getByRole("main", { name: "Save download" });
    main.getBoundingClientRect = () => ({ height }) as DOMRect;
    resizeAll();
  };

  it("fits what it shows, so its buttons are never cut off", async () => {
    const { api } = await openSaveWindow();
    pageHeight(396);
    expect(api.fitted.at(-1)).toBe(396);
    pageHeight(430); // more saved folders than fit on a line
    expect(api.fitted.at(-1)).toBe(430);
  });

  it("keeps its size for the question about a taken name", async () => {
    const api = new FakeApi(samples());
    api.clashes.set("/home/me/Downloads/A Day at the Beach.mp4", {
      existing: "A Day at the Beach.mp4",
      inUse: false,
      keepBoth: "A Day at the Beach (1)",
    });
    const { user, main } = await openSaveWindow(api);
    pageHeight(430);
    await user.click(button("Download", main));
    expect(await screen.findByRole("alertdialog")).toHaveStyle({ minHeight: "392px" });
  });
});

describe("a name that is already taken", () => {
  const beach = "/home/me/Downloads/A Day at the Beach.mp4";
  const taken = (inUse = false) => {
    const api = new FakeApi(samples());
    api.clashes.set(beach, {
      existing: "A Day at the Beach.mp4",
      inUse,
      keepBoth: "A Day at the Beach (1)",
    });
    return api;
  };
  const question = () => screen.getByRole("alertdialog");

  it("is flagged in the name field as it is typed", async () => {
    const { main } = await openSaveWindow(taken());
    expect(
      await within(main).findByRole("img", { name: "Already in this folder" }),
    ).toBeInTheDocument();
  });

  it("asks before saving, with Keep both as Enter's answer", async () => {
    const { api, user, main } = await openSaveWindow(taken());
    await user.click(button("Download", main));
    expect(await screen.findByRole("alertdialog")).toHaveAccessibleName(
      "Replace “A Day at the Beach.mp4”?",
    );
    expect(question()).toHaveTextContent(
      "Keep both saves this one as “A Day at the Beach (1).mp4”",
    );
    expect(api.calls.some((c) => c.startsWith("confirm"))).toBe(false);
    await waitFor(() => expect(button("Keep both", question())).toHaveFocus());

    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(api.calls).toContain(
        'confirm pending {"directory":"/home/me/Downloads","filename":"A Day at the Beach (1).mp4","start":true,"remember":false,"replace":false,"queue":"main"}',
      ),
    );
  });

  it("replaces the file when told to, later too", async () => {
    const { api, user, main } = await openSaveWindow(taken());
    await user.click(button("Later", main));
    await user.click(button("Replace", await screen.findByRole("alertdialog")));
    await waitFor(() =>
      expect(api.calls).toContain(
        'confirm pending {"directory":"/home/me/Downloads","filename":"A Day at the Beach.mp4","start":false,"remember":false,"replace":true,"queue":"main"}',
      ),
    );
  });

  it("goes back to change the name, by button or Escape", async () => {
    const { api, user, main } = await openSaveWindow(taken());
    await user.click(button("Download", main));
    await user.click(button("Change name", await screen.findByRole("alertdialog")));
    await waitFor(() => expect(nameField()).toHaveFocus());

    await user.click(button("Download", screen.getByRole("main")));
    await screen.findByRole("alertdialog");
    await user.keyboard("{Escape}");
    await waitFor(() => expect(nameField()).toHaveFocus());
    expect(api.calls).not.toContain("closeWindow");

    await user.clear(nameField());
    await user.type(nameField(), "Beach day");
    await user.keyboard("{Enter}");
    await waitFor(() =>
      expect(api.calls.some((c) => c.includes('"filename":"Beach day.mp4"'))).toBe(true),
    );
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("can't replace what another download is saving", async () => {
    const { user, main } = await openSaveWindow(taken(true));
    await user.click(button("Download", main));
    const asked = await screen.findByRole("alertdialog");
    expect(asked).toHaveAccessibleName("“A Day at the Beach.mp4” is taken");
    expect(within(asked).queryByRole("button", { name: "Replace" })).not.toBeInTheDocument();
    expect(button("Keep both", asked)).toBeEnabled();
  });
});

describe("a download waiting to be told where to go, in the list", () => {
  const beach = "A Day at the Beach";

  it("brings up its Save window when clicked", async () => {
    const { api, user } = await renderApp(new FakeApi(samples()));
    await user.click(row(beach));
    await waitFor(() => expect(api.calls).toContain("choose pending"));
  });

  it("brings it up from its button, its menu and Enter too", async () => {
    const { api, user } = await renderApp(new FakeApi(samples()));
    await user.click(button("Choose where to save", row(beach)));
    await user.click(button("More", row(beach)));
    await user.click(screen.getByRole("menuitem", { name: "Choose where to save…" }));
    await user.keyboard("{Enter}");
    await waitFor(() => expect(api.calls.filter((c) => c === "choose pending")).toHaveLength(3));
  });

  it("isn't asked about inside the main window", async () => {
    await renderApp(new FakeApi(samples()));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
