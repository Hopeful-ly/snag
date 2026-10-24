import { describe, expect, it } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/svelte";
import userEvent from "@testing-library/user-event";
import SaveWindow from "../SaveWindow.svelte";
import { FakeApi, sampleDownload, sampleQueues, samples, settled } from "./fake-api";
import { button, renderApp, row } from "./render";

// An app with Main, Tonight (on a schedule, not running) and Big games, and three downloads
// waiting in Tonight's line.
function withQueues(): FakeApi {
  const api = new FakeApi([
    ...settled(),
    sampleDownload({ id: "n1", name: "first.iso", queue: "night", position: 10 }),
    sampleDownload({ id: "n2", name: "second.zip", queue: "night", position: 11 }),
    sampleDownload({ id: "n3", name: "third.mp4", queue: "night", position: 12 }),
  ]);
  api.queuesValue = sampleQueues();
  return api;
}

const sidebar = () => screen.getByRole("navigation", { name: "Downloads" });
const tonight = () => within(sidebar()).getByRole("button", { name: /^Tonight:/ });
const order = () =>
  within(screen.getByRole("listbox", { name: "Tonight, in order" }))
    .getAllByRole("option")
    .map((o) => o.getAttribute("aria-label"));

describe("queues", () => {
  it("show in the sidebar with where each stands", async () => {
    await renderApp(withQueues());
    expect(within(sidebar()).getByRole("button", { name: "Main: 1 running" })).toBeInTheDocument();
    expect(tonight()).toHaveAccessibleName(/^Tonight: Starts at 2:00\sAM$/);
    expect(
      within(sidebar()).getByRole("button", { name: "Big games: Stopped" }),
    ).toBeInTheDocument();
    expect(within(tonight()).getByText("3")).toBeInTheDocument();
  });

  it("open as a line in order, saying how they run and why each waits", async () => {
    const { user } = await renderApp(withQueues());
    await user.click(tonight());
    expect(screen.getByRole("heading", { name: "Tonight" })).toBeInTheDocument();
    expect(order()).toEqual(["first.iso", "second.zip", "third.mp4"]);
    const strip = screen.getByRole("region", { name: "How Tonight runs" });
    expect(strip).toHaveTextContent(/2:00\sAM to 7:00\sAM, every day/);
    expect(strip).toHaveTextContent("2 at a time");
    expect(strip).toHaveTextContent("Shut down the PC when done");
    expect(row("first.iso")).toHaveTextContent(/Waits for 2:00\sAM in Tonight/);
    expect(within(row("first.iso")).getByLabelText("1st in line")).toBeInTheDocument();
  });

  it("start and stop", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    await user.click(button("Start now"));
    await waitFor(() => expect(api.calls).toContain("startQueue night"));
    await user.click(await screen.findByRole("button", { name: "Stop" }));
    await waitFor(() => expect(api.calls).toContain("stopQueue night"));
  });

  it("start one download now, whatever its queue does", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    await user.click(button("Start this one now", row("second.zip")));
    await waitFor(() => expect(api.calls).toContain("startNow n2"));
  });

  it("reverses the line so the last download is next", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    await user.click(button("Reverse"));
    await waitFor(() => expect(api.calls).toContain("reverseQueue night"));
    await waitFor(() => expect(order()).toEqual(["third.mp4", "second.zip", "first.iso"]));
  });

  it("reorder by dragging", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    // Third onto the top half of the first: it goes first.
    const data = dataTransfer();
    await fireEvent.dragStart(row("third.mp4"), { dataTransfer: data });
    const first = row("first.iso").parentElement!;
    first.getBoundingClientRect = () => ({ top: 0, height: 72 }) as DOMRect;
    await fireEvent.dragOver(first, { dataTransfer: data, clientY: 10 });
    await fireEvent.drop(first, { dataTransfer: data, clientY: 10 });
    await waitFor(() => expect(api.calls).toContain('reorder night ["n3","n1","n2"]'));
    await waitFor(() => expect(order()).toEqual(["third.mp4", "first.iso", "second.zip"]));
  });

  it("take downloads dropped on them from the list", async () => {
    const { api, user } = await renderApp(withQueues());
    const data = dataTransfer();
    await fireEvent.dragStart(row("Holiday-in-Lisbon.mp4"), { dataTransfer: data });
    const games = within(sidebar()).getByRole("button", { name: /^Big games/ });
    await fireEvent.dragOver(games, { dataTransfer: data });
    await fireEvent.drop(games, { dataTransfer: data });
    await waitFor(() => expect(api.calls).toContain('moveToQueue ["paused"] games'));
    await user.click(games);
    expect(screen.getByRole("option", { name: "Holiday-in-Lisbon.mp4" })).toBeInTheDocument();
  });

  it("have a menu of their own", async () => {
    const { api, user } = await renderApp(withQueues());
    const games = within(sidebar()).getByRole("button", { name: /^Big games/ });
    await fireEvent.contextMenu(games, { clientX: 40, clientY: 300 });
    const menu = screen.getByRole("menu", { name: "Big games actions" });
    expect(within(menu).getByRole("menuitem", { name: "Move down" })).toBeDisabled(); // last
    await user.click(within(menu).getByRole("menuitem", { name: "Start now" }));
    await waitFor(() => expect(api.calls).toContain("startQueue games"));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();

    await fireEvent.contextMenu(games, { clientX: 40, clientY: 300 });
    await user.click(screen.getByRole("menuitem", { name: "Move up" }));
    await waitFor(() => expect(api.calls).toContain('arrangeQueues ["main","games","night"]'));
    const names = () =>
      within(sidebar())
        .getAllByRole("button", { name: /^(Main|Tonight|Big games):/ })
        .map((b) => b.getAttribute("aria-label")?.split(":")[0]);
    await waitFor(() => expect(names()).toEqual(["Main", "Big games", "Tonight"]));

    await fireEvent.contextMenu(tonight(), { clientX: 40, clientY: 300 });
    await user.click(screen.getByRole("menuitem", { name: "Delete…" }));
    const sheet = screen.getByRole("dialog", { name: "Edit Tonight" });
    expect(button("Keep them in Main", sheet)).toBeInTheDocument(); // at the question at once
  });

  it("keep Main first, with only its own controls", async () => {
    await renderApp(withQueues());
    const main = within(sidebar()).getByRole("button", { name: /^Main:/ });
    await fireEvent.contextMenu(main, { clientX: 40, clientY: 200 });
    const items = within(screen.getByRole("menu")).getAllByRole("menuitem");
    expect(items.map((i) => i.textContent?.trim())).toEqual(["Stop", "Edit…", "Reverse"]);
  });

  it("are put in another order by dragging, as the others slide aside", async () => {
    const { api } = await renderApp(withQueues());
    // Tonight at y 100, Big games at 140, each 40 high.
    const night = tonight();
    const games = within(sidebar()).getByRole("button", { name: /^Big games/ });
    night.getBoundingClientRect = () => ({ top: 100, height: 40 }) as DOMRect;
    games.getBoundingClientRect = () => ({ top: 140, height: 40 }) as DOMRect;

    await fireEvent.pointerDown(games, { button: 0, pointerId: 1, clientY: 160 });
    await fireEvent.pointerMove(games, { pointerId: 1, clientY: 157 }); // a press, not yet a drag
    expect(games).not.toHaveClass("dragged");
    await fireEvent.pointerMove(games, { pointerId: 1, clientY: 115 }); // past Tonight's middle
    expect(games).toHaveClass("dragged");
    expect(games.style.transform).toBe("translateY(-40px)");
    expect(night.style.transform).toBe("translateY(40px)");
    await fireEvent.pointerUp(games, { pointerId: 1, clientY: 115 });
    await fireEvent.click(games); // the click that ends a drag opens nothing
    await waitFor(() => expect(api.calls).toContain('arrangeQueues ["main","games","night"]'));
    expect(screen.getByRole("heading", { level: 1 })).not.toHaveTextContent("Big games");
  });

  it("aren't moved by a click, and Main isn't moved at all", async () => {
    const { api, user } = await renderApp(withQueues());
    const main = within(sidebar()).getByRole("button", { name: /^Main:/ });
    await fireEvent.pointerDown(main, { button: 0, pointerId: 1, clientY: 10 });
    await fireEvent.pointerMove(main, { pointerId: 1, clientY: 200 });
    expect(main).not.toHaveClass("dragged");
    await fireEvent.pointerUp(main, { pointerId: 1, clientY: 200 });
    await user.click(tonight());
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("Tonight");
    expect(api.calls.some((c) => c.startsWith("arrangeQueues"))).toBe(false);
  });

  it("move a download from its menu", async () => {
    const { api } = await renderApp(withQueues());
    await fireEvent.contextMenu(row("Holiday-in-Lisbon.mp4"));
    const menu = screen.getByRole("menu");
    expect(within(menu).queryByRole("menuitem", { name: "Move to Main" })).not.toBeInTheDocument();
    await fireEvent.click(within(menu).getByRole("menuitem", { name: "Move to Tonight" }));
    await waitFor(() => expect(api.calls).toContain('moveToQueue ["paused"] night'));
  });

  it("are made in the queue sheet, which then opens them", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(within(sidebar()).getByRole("button", { name: "New queue" }));
    const sheet = screen.getByRole("dialog", { name: "New queue" });
    await user.type(within(sheet).getByRole("textbox", { name: "Name" }), "Weekends");
    await user.click(within(sheet).getByRole("switch", { name: "Run on a schedule" }));
    for (const day of ["Mon", "Tue", "Wed", "Thu", "Fri"])
      await user.click(within(sheet).getByRole("button", { name: day }));
    expect(within(sheet).getByText(/Starts at 2:00\sAM/)).toBeInTheDocument();
    expect(within(sheet).getByRole("group", { name: "Retries" })).toHaveTextContent("3");
    expect(within(sheet).getByRole("spinbutton", { name: "Seconds between tries" })).toHaveValue(5);
    await user.click(within(sheet).getByRole("radio", { name: "Sleep" }));
    await user.click(within(sheet).getByRole("switch", { name: "Limit the speed" }));
    await user.click(button("Create queue", sheet));
    await waitFor(() =>
      expect(api.calls).toContain(
        'saveQueue {"id":"","name":"Weekends","atOnce":2,"schedule":{"on":true,"start":"02:00","stop":"","days":[0,6]},"speedLimit":512000,"whenDone":"sleep","naming":"","retries":3,"retryWait":5}',
      ),
    );
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "Weekends" })).toBeInTheDocument(),
    );
  });

  it("are deleted from their sheet, keeping their downloads in Main", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    await user.click(button("Edit"));
    const sheet = screen.getByRole("dialog", { name: "Edit Tonight" });
    await user.click(button("Delete", sheet));
    expect(api.calls.some((c) => c.startsWith("deleteQueue"))).toBe(false);
    await user.click(button("Back", sheet)); // changed their mind: the form's buttons again
    expect(button("Save", sheet)).toBeInTheDocument();
    await user.click(button("Delete", sheet));
    await user.click(button("Keep them in Main", sheet));
    await waitFor(() => expect(api.calls).toContain("deleteQueue night false"));
    await waitFor(() =>
      expect(screen.getByRole("heading", { name: "All downloads" })).toBeInTheDocument(),
    );
    expect(row("first.iso")).toBeInTheDocument(); // in Main now
  });

  it("are deleted with their downloads, as a playlist's queue is", async () => {
    const { api, user } = await renderApp(withQueues());
    await user.click(tonight());
    await user.click(button("Delete queue…"));
    const sheet = screen.getByRole("dialog", { name: "Edit Tonight" });
    expect(within(sheet).getByRole("alertdialog")).toHaveTextContent(
      "Its 3 downloads aren't finished",
    );
    expect(within(sheet).queryByRole("button", { name: "Save" })).not.toBeInTheDocument();
    await user.click(button("Delete them too", sheet));
    await waitFor(() => expect(api.calls).toContain("deleteQueue night true"));
    expect(await screen.findByText("Deleted Tonight and its downloads")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "first.iso" })).not.toBeInTheDocument();
  });

  it("count down before shutting down, which can be called off", async () => {
    const { api, user } = await renderApp(withQueues());
    api.startCountdown({
      queue: "night",
      name: "Tonight",
      action: "shutdown",
      at: new Date(Date.now() + 55_000).toISOString() as unknown as never,
    });
    const dialog = await screen.findByRole("dialog", { name: "Tonight is done" });
    expect(dialog).toHaveTextContent(/Snag will shut down this PC in\s*0:5\d/);
    await user.click(button("Don't shut down", dialog));
    await waitFor(() => expect(api.calls).toContain("cancelCountdown"));
    await waitFor(() => expect(screen.queryByRole("dialog")).not.toBeInTheDocument());
  });
});

describe("the Save window's queue", () => {
  async function open() {
    const api = new FakeApi(samples());
    api.queuesValue = sampleQueues();
    const user = userEvent.setup();
    render(SaveWindow, { props: { api, id: "pending" } });
    await waitFor(() =>
      expect(screen.getByRole("textbox", { name: "Name" })).toHaveValue("A Day at the Beach"),
    );
    return { api, user, main: screen.getByRole("main", { name: "Save download" }) };
  }

  it("adds it to a queue that isn't running, to wait there", async () => {
    const { api, user, main } = await open();
    await user.click(within(main).getByRole("radio", { name: "Tonight" }));
    expect(main).toHaveTextContent(/Tonight starts at 2:00\sAM; this one waits for it/);
    await user.click(button("Add to Tonight", main));
    await waitFor(() =>
      expect(api.calls).toContain(
        'confirm pending {"directory":"/home/me/Downloads","filename":"","start":true,"remember":false,"replace":false,"queue":"night"}',
      ),
    );
    expect(api.calls).not.toContain("startNow pending");
  });

  it("or downloads it now, in that queue", async () => {
    const { api, user, main } = await open();
    await user.click(within(main).getByRole("radio", { name: "Big games" }));
    await user.click(button("Download now", main));
    await waitFor(() => expect(api.calls).toContain("startNow pending"));
  });

  it("isn't asked about with only Main", async () => {
    const api = new FakeApi(samples());
    render(SaveWindow, { props: { api, id: "pending" } });
    await waitFor(() => expect(screen.getByRole("textbox", { name: "Name" })).toBeInTheDocument());
    expect(screen.queryByRole("radiogroup", { name: "Queue" })).not.toBeInTheDocument();
  });
});

// dataTransfer is a stand-in for a drag's data, which jsdom lacks.
function dataTransfer() {
  const data = new Map<string, string>();
  return {
    get types() {
      return [...data.keys()];
    },
    setData: (kind: string, value: string) => data.set(kind, value),
    getData: (kind: string) => data.get(kind) ?? "",
    effectAllowed: "",
    dropEffect: "",
  };
}
